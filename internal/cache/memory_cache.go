package cache

import (
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/sashko-guz/mage/internal/logger"
)

// MemoryCache provides high-performance in-memory caching with LRU eviction
type MemoryCache struct {
	cache *ristretto.Cache
}

// MemoryCacheConfig defines configuration for the memory cache
type MemoryCacheConfig struct {
	MaxSize     int64         // Max memory in bytes
	MaxItems    int64         // Max number of items (optional)
	BufferItems int64         // Internal buffer size (10x MaxItems recommended)
	TTL         time.Duration // Default time to live for entries
}

// NewMemoryCache creates a new in-memory cache with the given configuration
func NewMemoryCache(cfg MemoryCacheConfig) (*MemoryCache, error) {
	// Validate and set defaults
	if cfg.MaxSize == 0 {
		return nil, fmt.Errorf("MaxSize must be specified for memory cache")
	}

	if cfg.MaxItems == 0 {
		// Estimate: assume average item is ~100KB
		cfg.MaxItems = max(cfg.MaxSize/(100*1024), 100)
	}

	if cfg.BufferItems == 0 {
		cfg.BufferItems = max(cfg.MaxItems*10, 1000)
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cfg.BufferItems, // Number of keys to track frequency (10x expected items)
		MaxCost:     cfg.MaxSize,     // Max memory usage in bytes
		BufferItems: 64,              // Number of keys per Get buffer
		Metrics:     true,            // Enable metrics collection
		OnEvict: func(item *ristretto.Item) {
			logger.Debugf("[MemoryCache] Evicted item (cost: %d bytes)", item.Cost)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	logger.Infof("[MemoryCache] Initialized: MaxSize=%dMB, MaxItems=%d, TTL=%v",
		cfg.MaxSize/(1024*1024), cfg.MaxItems, cfg.TTL)

	return &MemoryCache{
		cache: cache,
	}, nil
}

// Get retrieves a value from the cache
// Returns (data, found) where found indicates if the key was present
func (mc *MemoryCache) Get(key string) ([]byte, bool) {
	value, found := mc.cache.Get(key)
	if !found {
		return nil, false
	}

	data, ok := value.([]byte)
	if !ok {
		logger.Warnf("[MemoryCache] Invalid data type for key: %s", key)
		return nil, false
	}

	logger.Debugf("[MemoryCache] Cache HIT for key: %s", key)
	return data, true
}

// Set stores a value in the cache with the specified TTL
// Returns true if the value was successfully set (may return false if buffer is full)
func (mc *MemoryCache) Set(key string, data []byte, ttl time.Duration) bool {
	// Cost = size of data in bytes for accurate memory tracking
	cost := int64(len(data))

	// Set with TTL
	success := mc.cache.SetWithTTL(key, data, cost, ttl)

	if !success {
		logger.Warnf("[MemoryCache] Failed to set key (buffer full or rejected)")
	}

	return success
}

// Delete removes a key from the cache
func (mc *MemoryCache) Delete(key string) {
	mc.cache.Del(key)
}

// Clear removes all entries from the cache
func (mc *MemoryCache) Clear() {
	mc.cache.Clear()
	logger.Infof("[MemoryCache] Cache cleared")
}

// Wait blocks until all pending writes are processed
// This is useful before closing to ensure all Sets are committed
func (mc *MemoryCache) Wait() {
	mc.cache.Wait()
}

// Close closes the cache and releases resources
func (mc *MemoryCache) Close() {
	mc.cache.Close()
	logger.Infof("[MemoryCache] Cache closed")
}
