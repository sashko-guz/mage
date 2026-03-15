package cache

import (
	"github.com/sashko-guz/mage/internal/cache/memory"
)

// MemoryCache provides high-performance in-memory caching with LRU eviction.
// The implementation lives in the memory sub-package.
type MemoryCache = memory.MemoryCache

// MemoryCacheConfig defines configuration for the memory cache.
type MemoryCacheConfig = memory.Config

// NewMemoryCache creates a new in-memory cache with the given configuration.
func NewMemoryCache(cfg MemoryCacheConfig) (*MemoryCache, error) {
	return memory.New(cfg)
}
