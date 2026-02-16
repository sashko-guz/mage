package storage

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/sashko-guz/mage/internal/cache"
)

// cacheWriteTask represents a single cache write operation
type cacheWriteTask struct {
	key  string
	data []byte
}

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

	// Async write workers for sources
	sourceWriteQueue chan cacheWriteTask
	sourceWriteDone  chan struct{}
	sourceWriteMu    sync.WaitGroup

	// Async write workers for thumbnails
	thumbWriteQueue chan cacheWriteTask
	thumbWriteDone  chan struct{}
	thumbWriteMu    sync.WaitGroup
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
		cs.SetSourceAsync(cacheKey, data)
	}

	return data, nil
}

// initSourceWorkers starts worker goroutines for asynchronous source cache writes
func (cs *CachedStorage) initSourceWorkers(numWorkers, queueSize int) {
	if numWorkers <= 0 {
		numWorkers = 4
	}
	if queueSize <= 0 {
		queueSize = 1000
	}

	cs.sourceWriteQueue = make(chan cacheWriteTask, queueSize)
	cs.sourceWriteDone = make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		cs.sourceWriteMu.Add(1)
		go cs.sourceWriter()
	}
}

// sourceWriter is a worker goroutine that processes asynchronous source cache writes
func (cs *CachedStorage) sourceWriter() {
	defer cs.sourceWriteMu.Done()

	for {
		select {
		case task, ok := <-cs.sourceWriteQueue:
			if !ok {
				return // Channel closed, exit gracefully
			}
			if cs.sourceDiskCache != nil {
				if err := cs.sourceDiskCache.Set(task.key, task.data); err != nil {
					log.Printf("[CachedStorage] Error writing source to disk cache: %v", err)
				}
			}
		case <-cs.sourceWriteDone:
			return // Shutdown signal received
		}
	}
}

// SetSourceAsync queues an asynchronous write of source data to disk cache
// Returns immediately without waiting for write to complete
// If queue is full, the write is dropped (safe - data is in memory cache anyway)
func (cs *CachedStorage) SetSourceAsync(cacheKey string, data []byte) {
	// Make a copy of data since it will be written asynchronously
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	if cs.sourceWriteQueue == nil {
		// Async writes not enabled, do nothing (data is in memory cache)
		return
	}

	select {
	case cs.sourceWriteQueue <- cacheWriteTask{key: cacheKey, data: dataCopy}:
		// Queued successfully
	default:
		// Queue full - drop the write (image is in memory cache already)
		log.Printf("[CachedStorage] Source write queue full, skipping async write for: %s", cacheKey)
	}
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

// SetThumbnail stores a thumbnail in the thumb caches (memory only, synchronously)
// Disk writes happen asynchronously via SetThumbnailAsync
func (cs *CachedStorage) SetThumbnail(cacheKey string, data []byte) error {
	// If thumbs caching is disabled, don't store anything
	if !cs.ThumbsCacheEnabled() {
		return nil
	}

	thumbnailKey := "thumb:" + cacheKey

	// Store in memory cache synchronously (fast, blocking only on memory allocation)
	if cs.thumbMemoryCache != nil {
		cs.thumbMemoryCache.Set(thumbnailKey, data, cs.thumbTTL)
	}

	// Async disk write happens separately via SetThumbnailAsync
	log.Printf("[CachedStorage] Cached thumbnail (memory): %s", cacheKey)
	return nil
}

// initThumbWorkers starts worker goroutines for asynchronous thumbnail cache writes
func (cs *CachedStorage) initThumbWorkers(numWorkers, queueSize int) {
	if numWorkers <= 0 {
		numWorkers = 4
	}
	if queueSize <= 0 {
		queueSize = 1000
	}

	cs.thumbWriteQueue = make(chan cacheWriteTask, queueSize)
	cs.thumbWriteDone = make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		cs.thumbWriteMu.Add(1)
		go cs.thumbWriter()
	}
}

// thumbWriter is a worker goroutine that processes asynchronous thumbnail cache writes
func (cs *CachedStorage) thumbWriter() {
	defer cs.thumbWriteMu.Done()

	for {
		select {
		case task, ok := <-cs.thumbWriteQueue:
			if !ok {
				return // Channel closed, exit gracefully
			}
			if cs.thumbDiskCache != nil {
				if err := cs.thumbDiskCache.Set(task.key, task.data); err != nil {
					log.Printf("[CachedStorage] Error writing thumbnail to disk cache: %v", err)
				}
			}
		case <-cs.thumbWriteDone:
			return // Shutdown signal received
		}
	}
}

// SetThumbnailAsync queues an asynchronous write of thumbnail data to disk cache
// Returns immediately without waiting for write to complete
// If queue is full, the write is dropped (safe - data is in memory cache anyway)
func (cs *CachedStorage) SetThumbnailAsync(cacheKey string, data []byte) {
	// Make a copy of data since it will be written asynchronously
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	if cs.thumbWriteQueue == nil {
		// Async writes not enabled, do nothing (data is in memory cache)
		return
	}

	select {
	case cs.thumbWriteQueue <- cacheWriteTask{key: "thumb:" + cacheKey, data: dataCopy}:
		// Queued successfully
	default:
		// Queue full - drop the write (thumbnail is in memory cache already)
		log.Printf("[CachedStorage] Thumb write queue full, skipping async write for: %s", cacheKey)
	}
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

// Close releases cache resources and shuts down async workers
func (cs *CachedStorage) Close() error {
	// Shutdown source workers gracefully
	if cs.sourceWriteQueue != nil {
		close(cs.sourceWriteQueue)
		cs.sourceWriteMu.Wait() // Wait for all workers to finish draining queue
	}

	// Shutdown thumb workers gracefully
	if cs.thumbWriteQueue != nil {
		close(cs.thumbWriteQueue)
		cs.thumbWriteMu.Wait() // Wait for all workers to finish draining queue
	}

	// Close memory caches
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
