package cache

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/sashko-guz/mage/internal/format"
	"github.com/sashko-guz/mage/internal/logger"
	"lukechampine.com/blake3"
)

// Per-cleanup-run budgets to bound CPU and I/O usage.
// cleanupScanBudget limits how many LRU entries are inspected per run.
// cleanupRemoveBudget limits how many entries are collected for removal (expired or missing files).
const (
	cleanupScanBudget   = 4096
	cleanupRemoveBudget = 512
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

// NewDiskCache creates a new disk-based cache.
// basePath is the directory where cache files will be stored.
// ttl is the time-to-live for cache entries.
// clearOnStartup: if true, removes ALL cache files on startup.
// maxSizeBytes is the maximum cache size in bytes (0 = unlimited).
// maxItems is the maximum number of items tracked in the LRU index.
func NewDiskCache(basePath string, ttl time.Duration, clearOnStartup bool, maxSizeBytes int64, maxItems int) (*DiskCache, error) {
	absPath, err := prepareCacheDir(basePath)
	if err != nil {
		return nil, err
	}

	if maxItems <= 0 {
		maxItems = 100_000
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

// prepareCacheDir resolves the absolute path and ensures the directory exists.
func prepareCacheDir(basePath string) (string, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve cache path: %w", err)
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return absPath, nil
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
		return nil, ErrCacheNotFound
	}

	if now.After(entry.expiresAt) {
		dc.lru.Remove(hash)
		dc.mu.Unlock()
		logger.Debugf("[DiskCache] Cache entry expired for key: %s (expired at %v)", key, entry.expiresAt.Format(time.RFC3339))
		return nil, ErrCacheNotFound
	}

	filePath := entry.path
	dc.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			dc.removeStaleLRUEntry(hash, filePath)
			return nil, ErrCacheNotFound
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

// -------------------------------------------------------------------
// Cleanup goroutine
// -------------------------------------------------------------------

// cleanupExpired periodically removes expired cache entries.
func (dc *DiskCache) cleanupExpired() {
	const (
		baseInterval      = 30 * time.Second
		maxInterval       = 10 * time.Minute
		idleStep          = 30 * time.Second
		missingCheckEvery = 10
	)

	interval := baseInterval
	runs := 0
	timer := time.NewTimer(interval)
	defer timer.Stop()

	logger.Debugf("[DiskCache] Cleanup goroutine started for %s, base interval %v", dc.basePath, baseInterval)

	for {
		select {
		case <-timer.C:
		case <-dc.cleanupWake:
			if interval > baseInterval {
				stats := dc.performCleanup(false)
				interval = baseInterval
				resetTimer(timer, interval)
				logger.Debugf("[DiskCache] Activity detected, cleanup accelerated (removed=%d, entries=%d, size=%v)",
					stats.removed, stats.currentCount, format.Bytes(stats.currentSize))
			}
			continue
		}

		runs++
		checkMissingFiles := runs%missingCheckEvery == 0
		stats := dc.performCleanup(checkMissingFiles)
		interval = nextCleanupInterval(stats, interval, baseInterval, maxInterval, idleStep)
		timer.Reset(interval)
		logger.Debugf("[DiskCache] Cleanup scheduled in directory %s with interval %v (removed=%d, entries=%d, size=%v)",
			dc.basePath, interval, stats.removed, stats.currentCount, format.Bytes(stats.currentSize))
	}
}

// nextCleanupInterval backs off toward maxInterval when the cache is idle,
// and resets to baseInterval when entries are being actively removed.
func nextCleanupInterval(stats cleanupStats, current, base, max, step time.Duration) time.Duration {
	if stats.currentCount == 0 || stats.removed == 0 {
		if next := current + step; next < max {
			return next
		}
		return max
	}
	return base
}

// resetTimer safely stops and resets a timer, draining any pending tick.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func (dc *DiskCache) notifyActivity() {
	select {
	case dc.cleanupWake <- struct{}{}:
	default:
	}
}

// -------------------------------------------------------------------
// Cleanup implementation
// -------------------------------------------------------------------

func (dc *DiskCache) performCleanup(checkMissingFiles bool) cleanupStats {
	now := time.Now()

	expired, existing := dc.scanCandidates(now, checkMissingFiles)

	var missing []cleanupCandidate
	if checkMissingFiles {
		missing = dc.findMissingFiles(existing)
	}

	removed, size, count := dc.pruneIndex(expired, missing, now)
	return cleanupStats{removed: removed, currentSize: size, currentCount: count}
}

// scanCandidates walks a portion of the LRU index under the lock and returns:
//   - expired: entries whose TTL has elapsed
//   - existing: non-expired entries to be checked for missing files (only when collectExisting is true)
func (dc *DiskCache) scanCandidates(now time.Time, collectExisting bool) (expired, existing []cleanupCandidate) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	keys := dc.lru.Keys()
	keyCount := len(keys)
	if keyCount == 0 {
		dc.cleanupPos = 0
		return
	}

	start := dc.cleanupPos % keyCount
	scanCount := min(cleanupScanBudget, keyCount)

	for i := range scanCount {
		key := keys[(start+i)%keyCount]
		entry, ok := dc.lru.Peek(key)
		if !ok || entry == nil {
			continue
		}

		c := cleanupCandidate{key: key, path: entry.path}
		if now.After(entry.expiresAt) {
			expired = append(expired, c)
			if len(expired) >= cleanupRemoveBudget {
				break
			}
		} else if collectExisting && len(existing) < cleanupRemoveBudget {
			existing = append(existing, c)
		}
	}

	dc.cleanupPos = (start + scanCount) % keyCount
	return
}

// findMissingFiles returns entries from candidates whose backing file no longer exists on disk.
func (dc *DiskCache) findMissingFiles(candidates []cleanupCandidate) []cleanupCandidate {
	var missing []cleanupCandidate
	for _, c := range candidates {
		if _, err := os.Stat(c.path); os.IsNotExist(err) {
			missing = append(missing, c)
			if len(missing) >= cleanupRemoveBudget {
				break
			}
		}
	}
	return missing
}

// pruneIndex removes expired and missing entries from the LRU index under the lock,
// then evicts for size. Returns removal count, current cache size, and entry count.
func (dc *DiskCache) pruneIndex(expired, missing []cleanupCandidate, now time.Time) (removed int, size int64, count int) {
	dc.mu.Lock()

	for _, c := range expired {
		if entry, ok := dc.lru.Peek(c.key); ok && entry != nil && entry.path == c.path && now.After(entry.expiresAt) {
			dc.lru.Remove(c.key)
			removed++
		}
	}
	for _, c := range missing {
		if entry, ok := dc.lru.Peek(c.key); ok && entry != nil && entry.path == c.path {
			dc.lru.Remove(c.key)
			removed++
		}
	}

	removed += dc.evictForSizeLocked(1024)
	size = dc.currentSize.Load()
	count = dc.lru.Len()
	dc.mu.Unlock()
	return
}

// -------------------------------------------------------------------
// Disk index management
// -------------------------------------------------------------------

func (dc *DiskCache) loadIndexFromDisk() {
	now := time.Now()
	totalScanned := 0
	deletedCount := 0

	_ = filepath.Walk(dc.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".cache" {
			return nil
		}
		totalScanned++
		if dc.processIndexFile(path, info, now) {
			deletedCount++
		}
		return nil
	})

	logger.Infof("[DiskCache] Startup index load complete: scanned=%d, removed=%d, entries=%d, size=%v",
		totalScanned, deletedCount, dc.lru.Len(), format.Bytes(dc.currentSize.Load()))
}

// processIndexFile attempts to index a single cache file.
// Returns true if the file was removed (expired or unparseable), false if it was indexed.
func (dc *DiskCache) processIndexFile(path string, info os.FileInfo, now time.Time) (deleted bool) {
	hash, expiresAt, err := dc.parseCacheFilename(filepath.Base(path))
	if err != nil || now.After(expiresAt) {
		if removeErr := os.Remove(path); removeErr == nil || os.IsNotExist(removeErr) {
			deleted = true
		}
		dc.cleanupEmptyDirs(filepath.Dir(path))
		return
	}

	dc.updateLRUEntry(&cacheEntry{
		hash:      hash,
		path:      path,
		size:      info.Size(),
		expiresAt: expiresAt,
	})
	return false
}

// -------------------------------------------------------------------
// LRU management
// -------------------------------------------------------------------

func (dc *DiskCache) initLRU() {
	lruIndex, err := simplelru.NewLRU(dc.MaxItems, func(_ string, entry *cacheEntry) {
		if entry == nil {
			return
		}
		dc.currentSize.Add(-entry.size)
		dc.enqueueDelete(entry.path)
	})
	if err != nil {
		panic(fmt.Sprintf("failed to initialize LRU index: %v", err))
	}
	dc.lru = lruIndex
}

// updateLRUEntry registers or replaces an entry in the LRU index, then evicts if over the size limit.
func (dc *DiskCache) updateLRUEntry(entry *cacheEntry) {
	dc.mu.Lock()
	if _, exists := dc.lru.Peek(entry.hash); exists {
		dc.lru.Remove(entry.hash)
	}
	dc.currentSize.Add(entry.size)
	dc.lru.Add(entry.hash, entry)
	dc.evictForSizeLocked(2048)
	dc.mu.Unlock()
}

// removeStaleLRUEntry removes the LRU entry for hash if it still points to the given path.
func (dc *DiskCache) removeStaleLRUEntry(hash, path string) {
	dc.mu.Lock()
	if stale, exists := dc.lru.Peek(hash); exists && stale.path == path {
		dc.lru.Remove(hash)
	}
	dc.mu.Unlock()
}

// evictForSizeLocked evicts LRU entries until the cache is below the low-water mark.
// Must be called with dc.mu held.
func (dc *DiskCache) evictForSizeLocked(maxRemovals int) int {
	high, low := dc.sizeWatermarks()
	if high <= 0 || dc.currentSize.Load() <= high {
		return 0
	}

	removed := 0
	for dc.currentSize.Load() > low {
		if maxRemovals > 0 && removed >= maxRemovals {
			break
		}
		_, _, ok := dc.lru.RemoveOldest()
		if !ok {
			break
		}
		removed++
	}
	return removed
}

func (dc *DiskCache) sizeWatermarks() (high int64, low int64) {
	if dc.MaxSize <= 0 {
		return 0, 0
	}

	high = (dc.MaxSize * 95) / 100
	low = (dc.MaxSize * 85) / 100

	if high <= 0 {
		high = dc.MaxSize
	}
	if low <= 0 || low >= high {
		low = (high * 9) / 10
		if low <= 0 {
			low = high
		}
	}
	return high, low
}

// -------------------------------------------------------------------
// File & delete helpers
// -------------------------------------------------------------------

// atomicWriteFile writes data to a temp file and atomically renames it into place.
func atomicWriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory structure: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}
	return nil
}

func (dc *DiskCache) enqueueDelete(path string) {
	select {
	case dc.deleteQueue <- path:
	default:
		go dc.deleteFile(path)
	}
}

func (dc *DiskCache) deleteWorker() {
	for path := range dc.deleteQueue {
		dc.deleteFile(path)
	}
}

func (dc *DiskCache) deleteFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Errorf("[DiskCache] Error deleting evicted file %s: %v", path, err)
		return
	}
	dc.cleanupEmptyDirs(filepath.Dir(path))
}

// -------------------------------------------------------------------
// Path & hash utilities
// -------------------------------------------------------------------

// getHash generates a BLAKE3 hash from key.
func (dc *DiskCache) getHash(key string) string {
	hash := blake3.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getDirPath generates a hierarchical directory path using nginx-style levels=2:2
// to limit files per directory.
func (dc *DiskCache) getDirPath(hashStr string) string {
	n := len(hashStr)
	return filepath.Join(dc.basePath, hashStr[n-2:n], hashStr[n-4:n-2])
}

// getFilePathWithExpiration generates a cache file path with the expiry timestamp encoded in the name.
// Format: basePath/f1/8e/{hash}_{unixTimestamp}.cache
func (dc *DiskCache) getFilePathWithExpiration(hashStr string, expiresAt time.Time) string {
	return filepath.Join(dc.getDirPath(hashStr), fmt.Sprintf("%s_%d.cache", hashStr, expiresAt.Unix()))
}

// parseCacheFilename extracts hash and expiration timestamp from a cache filename.
// Format: {hash}_{unixTimestamp}.cache
func (dc *DiskCache) parseCacheFilename(filename string) (string, time.Time, error) {
	name := strings.TrimSuffix(filename, ".cache")
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore == -1 {
		return "", time.Time{}, fmt.Errorf("invalid filename format: %s", filename)
	}

	timestamp, err := strconv.ParseInt(name[lastUnderscore+1:], 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid timestamp in filename: %w", err)
	}
	return name[:lastUnderscore], time.Unix(timestamp, 0), nil
}

// cleanupEmptyDirs removes empty directories up to the base path.
func (dc *DiskCache) cleanupEmptyDirs(dir string) {
	if dir == dc.basePath || !strings.HasPrefix(dir, dc.basePath) {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	if err := os.Remove(dir); err == nil {
		dc.cleanupEmptyDirs(filepath.Dir(dir))
	}
}
