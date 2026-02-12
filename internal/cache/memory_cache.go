package cache

import (
	"fmt"
	"log"
	"time"

	"github.com/dgraph-io/ristretto"
)

// MemoryCache provides high-performance in-memory caching with LRU eviction
type MemoryCache struct {
	cache *ristretto.Cache
	name  string
}

// MemoryCacheConfig defines configuration for the memory cache
type MemoryCacheConfig struct {
	Name        string        // Cache name for logging
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
		cfg.MaxItems = cfg.MaxSize / (100 * 1024)
		if cfg.MaxItems < 100 {
			cfg.MaxItems = 100
		}
	}

	if cfg.BufferItems == 0 {
		cfg.BufferItems = cfg.MaxItems * 10
		if cfg.BufferItems < 1000 {
			cfg.BufferItems = 1000
		}
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cfg.BufferItems, // Number of keys to track frequency (10x expected items)
		MaxCost:     cfg.MaxSize,     // Max memory usage in bytes
		BufferItems: 64,              // Number of keys per Get buffer
		Metrics:     true,            // Enable metrics collection
		OnEvict: func(item *ristretto.Item) {
			if cfg.Name != "" {
				log.Printf("[MemoryCache:%s] Evicted item (cost: %d bytes)", cfg.Name, item.Cost)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	log.Printf("[MemoryCache:%s] Initialized: MaxSize=%dMB, MaxItems=%d",
		cfg.Name, cfg.MaxSize/(1024*1024), cfg.MaxItems)

	return &MemoryCache{
		cache: cache,
		name:  cfg.Name,
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
		log.Printf("[MemoryCache:%s] Invalid data type for key: %s", mc.name, key)
		return nil, false
	}

	log.Printf("[MemoryCache:%s] Cache HIT for key: %s", mc.name, key)
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
		log.Printf("[MemoryCache:%s] Warning: Failed to set key (buffer full or rejected)", mc.name)
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
	log.Printf("[MemoryCache:%s] Cache cleared", mc.name)
}

// Wait blocks until all pending writes are processed
// This is useful before closing to ensure all Sets are committed
func (mc *MemoryCache) Wait() {
	mc.cache.Wait()
}

// GetMetrics returns cache performance metrics
func (mc *MemoryCache) GetMetrics() *ristretto.Metrics {
	return mc.cache.Metrics
}

// GetStats returns formatted cache statistics
func (mc *MemoryCache) GetStats() map[string]interface{} {
	metrics := mc.cache.Metrics

	hits := metrics.Hits()
	misses := metrics.Misses()
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	return map[string]interface{}{
		"name":         mc.name,
		"hits":         hits,
		"misses":       misses,
		"total":        total,
		"hit_ratio":    hitRatio,
		"keys_added":   metrics.KeysAdded(),
		"keys_evicted": metrics.KeysEvicted(),
		"keys_updated": metrics.KeysUpdated(),
		"cost_added":   metrics.CostAdded(),
		"cost_evicted": metrics.CostEvicted(),
	}
}

// Close closes the cache and releases resources
func (mc *MemoryCache) Close() {
	mc.cache.Close()
	log.Printf("[MemoryCache:%s] Cache closed", mc.name)
}
