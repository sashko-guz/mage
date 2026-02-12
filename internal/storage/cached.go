package storage

import (
	"context"
	"log"

	"github.com/sashko-guz/mage/internal/cache"
)

// CachedStorage wraps a Storage implementation with unified multi-layer caching
// The cache stores BOTH source images and generated thumbnails with a shared size limit
// Layer 1: In-memory LRU cache (fastest, optional) - unified for sources + thumbnails
// Layer 2: Disk-based cache (persistent) - unified for sources + thumbnails
// Layer 3: Underlying storage (S3, local, etc.) - only for source images
type CachedStorage struct {
	underlying  Storage
	diskCache   *cache.DiskCache
	memoryCache *cache.MemoryCache
	storageName string
}

// GetObject retrieves an object through the multi-layer cache hierarchy
// 1. Check memory cache (fastest)
// 2. Check file cache
// 3. Fetch from underlying storage and populate caches
func (cs *CachedStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	cacheKey := "source:" + key

	// Layer 1: Check memory cache first (if enabled)
	if cs.memoryCache != nil {
		if data, found := cs.memoryCache.Get(cacheKey); found {
			log.Printf("[CachedStorage:%s] Memory cache HIT (source) for key: %s", cs.storageName, key)
			return data, nil
		}
		log.Printf("[CachedStorage:%s] Memory cache miss for key: %s", cs.storageName, key)
	}

	// Layer 2: Check disk cache
	if data, err := cs.diskCache.Get(cacheKey); err == nil {
		log.Printf("[CachedStorage:%s] Disk cache HIT (source) for key: %s", cs.storageName, key)

		// Populate memory cache for next time
		if cs.memoryCache != nil {
			cs.memoryCache.Set(cacheKey, data, cs.diskCache.TTL)
			log.Printf("[CachedStorage:%s] Promoted to memory cache: %s", cs.storageName, key)
		}

		return data, nil
	}

	// Layer 3: Fetch from underlying storage (S3, local, etc.)
	log.Printf("[CachedStorage:%s] Cache miss, fetching from underlying storage: %s", cs.storageName, key)
	data, err := cs.underlying.GetObject(ctx, key)
	if err != nil {
		return nil, err
	}

	// Backfill both caches
	// Memory cache first (fast, non-blocking)
	if cs.memoryCache != nil {
		cs.memoryCache.Set(cacheKey, data, cs.diskCache.TTL)
	}

	// Disk cache (slower, but persistent)
	if err := cs.diskCache.Set(cacheKey, data); err != nil {
		log.Printf("[CachedStorage:%s] Error writing to disk cache: %v", cs.storageName, err)
		// Don't fail if caching fails, just return the data
	}

	return data, nil
}

// GetThumbnail retrieves a cached thumbnail
// Returns (data, found, error) where found indicates if the thumbnail was in cache
func (cs *CachedStorage) GetThumbnail(cacheKey string) ([]byte, bool, error) {
	thumbnailKey := "thumb:" + cacheKey

	// Layer 1: Check memory cache first (if enabled)
	if cs.memoryCache != nil {
		if data, found := cs.memoryCache.Get(thumbnailKey); found {
			log.Printf("[CachedStorage:%s] Memory cache HIT (thumbnail) for key: %s", cs.storageName, cacheKey)
			return data, true, nil
		}
	}

	// Layer 2: Check disk cache
	if data, err := cs.diskCache.Get(thumbnailKey); err == nil {
		log.Printf("[CachedStorage:%s] Disk cache HIT (thumbnail) for key: %s", cs.storageName, cacheKey)

		// Populate memory cache for next time
		if cs.memoryCache != nil {
			cs.memoryCache.Set(thumbnailKey, data, cs.diskCache.TTL)
			log.Printf("[CachedStorage:%s] Promoted thumbnail to memory cache: %s", cs.storageName, cacheKey)
		}

		return data, true, nil
	}

	return nil, false, nil
}

// SetThumbnail stores a thumbnail in the cache
func (cs *CachedStorage) SetThumbnail(cacheKey string, data []byte) error {
	thumbnailKey := "thumb:" + cacheKey

	// Memory cache first (fast, non-blocking)
	if cs.memoryCache != nil {
		cs.memoryCache.Set(thumbnailKey, data, cs.diskCache.TTL)
	}

	// Disk cache (slower, but persistent)
	if err := cs.diskCache.Set(thumbnailKey, data); err != nil {
		log.Printf("[CachedStorage:%s] Error writing thumbnail to disk cache: %v", cs.storageName, err)
		return err
	}

	log.Printf("[CachedStorage:%s] Cached thumbnail: %s", cs.storageName, cacheKey)
	return nil
}

// ClearCache clears all cached entries (useful for testing or manual invalidation)
func (cs *CachedStorage) ClearCache() error {
	if cs.memoryCache != nil {
		cs.memoryCache.Clear()
	}
	return cs.diskCache.Clear()
}

// CacheStats returns cache statistics
func (cs *CachedStorage) CacheStats() (count int, totalSize int64, err error) {
	return cs.diskCache.CacheStats()
}

// GetMemoryCacheStats returns memory cache statistics if enabled
func (cs *CachedStorage) GetMemoryCacheStats() map[string]interface{} {
	if cs.memoryCache == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := cs.memoryCache.GetStats()
	stats["enabled"] = true
	return stats
}

// Close releases cache resources
func (cs *CachedStorage) Close() error {
	if cs.memoryCache != nil {
		cs.memoryCache.Wait() // Ensure all pending writes are committed
		cs.memoryCache.Close()
	}
	return nil
}
