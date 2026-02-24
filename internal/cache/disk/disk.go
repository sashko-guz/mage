package disk

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/sashko-guz/mage/internal/format"
	"github.com/sashko-guz/mage/internal/logger"
)

// ErrNotFound is returned by Get when the requested key is not in the cache.
var ErrNotFound = errors.New("cache entry not found")

// Per-cleanup-run budgets to bound CPU and I/O usage.
// cleanupScanBudget limits how many LRU entries are inspected per run.
// cleanupRemoveBudget limits how many entries are collected for removal (expired or missing files).
const (
	cleanupScanBudget   = 4096
	cleanupRemoveBudget = 512
	defaultMaxItems     = 100_000
)

// -------------------------------------------------------------------
// Types
// -------------------------------------------------------------------

// DiskCache provides disk-based caching with TTL support and LRU eviction.
type DiskCache struct {
	basePath string
	MaxSize  int64         // Maximum cache size in bytes (0 = unlimited)
	TTL      time.Duration // Exported so CachedStorage can access it
	MaxItems int

	mu          sync.Mutex
	currentSize atomic.Int64
	lru         *simplelru.LRU[string, *cacheEntry]
	cleanupWake chan struct{}
	deleteQueue chan string
	cleanupPos  int
}

type cacheEntry struct {
	hash      string
	path      string
	size      int64
	expiresAt time.Time
}

type cleanupStats struct {
	removed      int
	currentSize  int64
	currentCount int
}

type cleanupCandidate struct {
	key  string
	path string
}

// -------------------------------------------------------------------
// Constructor
// -------------------------------------------------------------------

// New creates a new disk-based cache.
// basePath is the directory where cache files will be stored.
// ttl is the time-to-live for cache entries.
// clearOnStartup: if true, removes ALL cache files on startup.
// maxSizeBytes is the maximum cache size in bytes (0 = unlimited).
// maxItems is the maximum number of items tracked in the LRU index.
func New(basePath string, ttl time.Duration, clearOnStartup bool, maxSizeBytes int64, maxItems int) (*DiskCache, error) {
	absPath, err := prepareCacheDir(basePath)
	if err != nil {
		return nil, err
	}

	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}

	dc := &DiskCache{
		basePath:    absPath,
		MaxSize:     maxSizeBytes,
		TTL:         ttl,
		MaxItems:    maxItems,
		cleanupWake: make(chan struct{}, 1),
		deleteQueue: make(chan string, 4096),
	}

	go dc.deleteWorker()
	dc.initLRU()

	if clearOnStartup {
		logger.Infof("[DiskCache] Clearing all cache files in %s (clearOnStartup=true)", absPath)
		if err := dc.Clear(); err != nil {
			logger.Errorf("[DiskCache] Error during startup cache clear: %v", err)
		}
	} else {
		logger.Infof("[DiskCache] Loading cache index from disk: %s", absPath)
		dc.loadIndexFromDisk()
	}

	go dc.cleanupExpired()

	logger.Infof("[DiskCache] Initialized: BasePath=%s, TTL=%v, MaxSize=%v, MaxItems=%d",
		absPath, ttl, format.Bytes(maxSizeBytes), dc.MaxItems)
	return dc, nil
}

// -------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------

// Get retrieves a cached item by key.
func (dc *DiskCache) Get(key string) ([]byte, error) {
	dc.notifyActivity()

	hash := dc.getHash(key)
	now := time.Now()

	dc.mu.Lock()
	entry, ok := dc.lru.Get(hash)
	if !ok {
		dc.mu.Unlock()
		return nil, ErrNotFound
	}

	if now.After(entry.expiresAt) {
		dc.lru.Remove(hash)
		dc.mu.Unlock()
		logger.Debugf("[DiskCache] Cache entry expired for key: %s (expired at %v)", key, entry.expiresAt.Format(time.RFC3339))
		return nil, ErrNotFound
	}

	filePath := entry.path
	dc.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			dc.removeStaleLRUEntry(hash, filePath)
			return nil, ErrNotFound
		}
		logger.Errorf("[DiskCache] Error reading cache file %s: %v", filePath, err)
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	logger.Debugf("[DiskCache] Cache HIT for key: %s (expires at %v)", key, entry.expiresAt.Format(time.RFC3339))
	return data, nil
}

// Set stores a cached item by key with TTL.
func (dc *DiskCache) Set(key string, data []byte) error {
	dc.notifyActivity()

	expiresAt := time.Now().Add(dc.TTL)
	hash := dc.getHash(key)
	filePath := dc.getFilePathWithExpiration(hash, expiresAt)

	if err := atomicWriteFile(filePath, data); err != nil {
		return err
	}

	dc.updateLRUEntry(&cacheEntry{
		hash:      hash,
		path:      filePath,
		size:      int64(len(data)),
		expiresAt: expiresAt,
	})

	logger.Debugf("[DiskCache] Cached data for key: %s (expires at %v, TTL: %v)", key, expiresAt.Format(time.RFC3339), dc.TTL)
	return nil
}

// Delete removes a cache entry.
func (dc *DiskCache) Delete(key string) error {
	dc.notifyActivity()

	hash := dc.getHash(key)
	dc.mu.Lock()
	dc.lru.Remove(hash)
	dc.mu.Unlock()
	return nil
}

// Clear removes all cache entries.
func (dc *DiskCache) Clear() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if err := os.RemoveAll(dc.basePath); err != nil {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}
	if err := os.MkdirAll(dc.basePath, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}

	dc.currentSize.Store(0)
	dc.cleanupPos = 0
	dc.initLRU()

	logger.Infof("[DiskCache] Cache cleared")
	return nil
}
