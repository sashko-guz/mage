package storage

import (
	"context"
	"log"
	"time"

	"github.com/sashko-guz/mage/internal/cache"
)

// CachedStorage wraps a Storage implementation with separate multi-layer caching for sources and thumbnails
// Layer 1: In-memory LRU cache (fastest, optional) - separate for sources and thumbnails
// Layer 2: Disk-based cache (persistent, optional) - separate for sources and thumbnails
// Layer 3: Underlying storage (S3, local, etc.) - only for source images
type CachedStorage struct {
	underlying Storage

	// Source image caching
	sourceMemoryCache *cache.MemoryCache
	sourceDiskCache   *cache.DiskCache
	sourceTTL         time.Duration

	// Generated thumbnail caching
	thumbMemoryCache *cache.MemoryCache
	thumbDiskCache   *cache.DiskCache
	thumbTTL         time.Duration
}

// SourcesCacheEnabled returns true if any source cache layer is enabled
func (cs *CachedStorage) SourcesCacheEnabled() bool {
	return cs.sourceMemoryCache != nil || cs.sourceDiskCache != nil
}

// ThumbsCacheEnabled returns true if any thumbnail cache layer is enabled
func (cs *CachedStorage) ThumbsCacheEnabled() bool {
	return cs.thumbMemoryCache != nil || cs.thumbDiskCache != nil
}

// GetObject retrieves a source image through the multi-layer cache hierarchy
// 1. Check source memory cache (fastest)
// 2. Check source disk cache
// 3. Fetch from underlying storage and populate caches
func (cs *CachedStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	cacheKey := "source:" + key

	// If sources caching is disabled, bypass all cache logic
	if !cs.SourcesCacheEnabled() {
		return cs.underlying.GetObject(ctx, key)
	}

	// Layer 1: Check source memory cache first (if enabled)
	if cs.sourceMemoryCache != nil {
		if data, found := cs.sourceMemoryCache.Get(cacheKey); found {
			log.Printf("[CachedStorage] Source memory cache HIT for key: %s", key)
			return data, nil
		}
	}

	// Layer 2: Check source disk cache (if enabled)
	if cs.sourceDiskCache != nil {
		if data, err := cs.sourceDiskCache.Get(cacheKey); err == nil {
			log.Printf("[CachedStorage] Source disk cache HIT for key: %s", key)

			// Populate source memory cache for next time
			if cs.sourceMemoryCache != nil {
				cs.sourceMemoryCache.Set(cacheKey, data, cs.sourceTTL)
				log.Printf("[CachedStorage] Source promoted to memory cache: %s", key)
			}

			return data, nil
		}
	}

	// Layer 3: Fetch from underlying storage (S3, local, etc.)
	log.Printf("[CachedStorage] Source cache miss, fetching from underlying storage: %s", key)
	data, err := cs.underlying.GetObject(ctx, key)
	if err != nil {
		return nil, err
	}

	// Backfill source caches
	if cs.sourceMemoryCache != nil {
		cs.sourceMemoryCache.Set(cacheKey, data, cs.sourceTTL)
	}

	if cs.sourceDiskCache != nil {
		if err := cs.sourceDiskCache.Set(cacheKey, data); err != nil {
			log.Printf("[CachedStorage] Error writing source to disk cache: %v", err)
		}
	}

	return data, nil
}

// GetThumbnail retrieves a cached thumbnail from thumb caches
// Returns (data, found, error) where found indicates if the thumbnail was in cache
func (cs *CachedStorage) GetThumbnail(cacheKey string) ([]byte, bool, error) {
	// If thumbs caching is disabled, no thumbnail can be cached
	if !cs.ThumbsCacheEnabled() {
		return nil, false, nil
	}

	thumbnailKey := "thumb:" + cacheKey

	// Layer 1: Check thumb memory cache first (if enabled)
	if cs.thumbMemoryCache != nil {
		if data, found := cs.thumbMemoryCache.Get(thumbnailKey); found {
			log.Printf("[CachedStorage] Thumb memory cache HIT for key: %s", cacheKey)
			return data, true, nil
		}
	}

	// Layer 2: Check thumb disk cache (if enabled)
	if cs.thumbDiskCache != nil {
		if data, err := cs.thumbDiskCache.Get(thumbnailKey); err == nil {
			log.Printf("[CachedStorage] Thumb disk cache HIT for key: %s", cacheKey)

			// Populate thumb memory cache for next time
			if cs.thumbMemoryCache != nil {
				cs.thumbMemoryCache.Set(thumbnailKey, data, cs.thumbTTL)
				log.Printf("[CachedStorage] Thumb promoted to memory cache: %s", cacheKey)
			}

			return data, true, nil
		}
	}

	return nil, false, nil
}

// SetThumbnail stores a thumbnail in the thumb caches
func (cs *CachedStorage) SetThumbnail(cacheKey string, data []byte) error {
	// If thumbs caching is disabled, don't store anything
	if !cs.ThumbsCacheEnabled() {
		return nil
	}

	thumbnailKey := "thumb:" + cacheKey

	// Thumb memory cache first (fast, non-blocking)
	if cs.thumbMemoryCache != nil {
		cs.thumbMemoryCache.Set(thumbnailKey, data, cs.thumbTTL)
	}

	// Thumb disk cache (slower, but persistent)
	if cs.thumbDiskCache != nil {
		if err := cs.thumbDiskCache.Set(thumbnailKey, data); err != nil {
			log.Printf("[CachedStorage] Error writing thumbnail to disk cache: %v", err)
			return err
		}
	}

	log.Printf("[CachedStorage] Cached thumbnail: %s", cacheKey)
	return nil
}

// ClearCache clears all cached entries (useful for testing or manual invalidation)
func (cs *CachedStorage) ClearCache() error {
	if cs.sourceMemoryCache != nil {
		cs.sourceMemoryCache.Clear()
	}
	if cs.sourceDiskCache != nil {
		if err := cs.sourceDiskCache.Clear(); err != nil {
			log.Printf("[CachedStorage] Error clearing source disk cache: %v", err)
			return err
		}
	}
	if cs.thumbMemoryCache != nil {
		cs.thumbMemoryCache.Clear()
	}
	if cs.thumbDiskCache != nil {
		if err := cs.thumbDiskCache.Clear(); err != nil {
			log.Printf("[CachedStorage] Error clearing thumb disk cache: %v", err)
			return err
		}
	}
	return nil
}

// Close releases cache resources
func (cs *CachedStorage) Close() error {
	if cs.sourceMemoryCache != nil {
		cs.sourceMemoryCache.Wait()
		cs.sourceMemoryCache.Close()
	}
	if cs.thumbMemoryCache != nil {
		cs.thumbMemoryCache.Wait()
		cs.thumbMemoryCache.Close()
	}
	return nil
}
