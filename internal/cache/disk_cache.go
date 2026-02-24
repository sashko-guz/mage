package cache

import (
	"time"

	"github.com/sashko-guz/mage/internal/cache/disk"
)

// DiskCache is a disk-based cache with TTL and LRU eviction.
// The implementation lives in the disk sub-package.
type DiskCache = disk.DiskCache

// NewDiskCache creates a new disk-based cache.
// basePath is the directory where cache files will be stored.
// ttl is the time-to-live for cache entries.
// clearOnStartup: if true, removes ALL cache files on startup.
// maxSizeBytes is the maximum cache size in bytes (0 = unlimited).
// maxItems is the maximum number of items tracked in the LRU index.
func NewDiskCache(basePath string, ttl time.Duration, clearOnStartup bool, maxSizeBytes int64, maxItems int) (*DiskCache, error) {
	return disk.New(basePath, ttl, clearOnStartup, maxSizeBytes, maxItems)
}
