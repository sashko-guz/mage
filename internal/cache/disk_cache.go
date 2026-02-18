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
	"github.com/sashko-guz/mage/internal/logger"
	"lukechampine.com/blake3"
)

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(bytes)
	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.2f%s", size, units[unitIndex])
}

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

// NewDiskCache creates a new disk-based cache.
// basePath is the directory where cache files will be stored.
// ttl is the time-to-live for cache entries.
// clearOnStartup: if true, removes ALL cache files on startup.
// maxSizeBytes is the maximum cache size in bytes (0 = unlimited).
// maxItems is the maximum number of items tracked in the LRU index.
func NewDiskCache(basePath string, ttl time.Duration, clearOnStartup bool, maxSizeBytes int64, maxItems int) (*DiskCache, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
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

	logger.Infof("[DiskCache] Initialized: BasePath=%s, TTL=%v, MaxSize=%v, MaxItems=%d", absPath, ttl, formatBytes(maxSizeBytes), dc.MaxItems)
	return dc, nil
}

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
			dc.mu.Lock()
			if stale, exists := dc.lru.Peek(hash); exists && stale.path == filePath {
				dc.lru.Remove(hash)
			}
			dc.mu.Unlock()
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
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory structure: %w", err)
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	entry := &cacheEntry{
		hash:      hash,
		path:      filePath,
		size:      int64(len(data)),
		expiresAt: expiresAt,
	}

	dc.mu.Lock()
	if _, exists := dc.lru.Peek(hash); exists {
		dc.lru.Remove(hash)
	}
	dc.currentSize.Add(entry.size)
	dc.lru.Add(hash, entry)
	dc.evictForSizeLocked(2048)
	dc.mu.Unlock()

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
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(interval)
				logger.Debugf("[DiskCache] Activity detected, cleanup accelerated (removed=%d, entries=%d, size=%v)",
					stats.removed, stats.currentCount, formatBytes(stats.currentSize))
			}
			continue
		}

		runs++

		checkMissingFiles := runs%missingCheckEvery == 0
		stats := dc.performCleanup(checkMissingFiles)

		if stats.currentCount == 0 || stats.removed == 0 {
			interval += idleStep
			if interval > maxInterval {
				interval = maxInterval
			}
		} else {
			interval = baseInterval
		}

		timer.Reset(interval)
		logger.Debugf("[DiskCache] Cleanup scheduled in %v (removed=%d, entries=%d, size=%v)",
			interval, stats.removed, stats.currentCount, formatBytes(stats.currentSize))
	}
}

func (dc *DiskCache) notifyActivity() {
	select {
	case dc.cleanupWake <- struct{}{}:
	default:
	}
}

func (dc *DiskCache) performCleanup(checkMissingFiles bool) cleanupStats {
	const (
		scanBudget         = 1024
		removeBudget       = 256
		missingCheckBudget = 256
	)

	now := time.Now()
	removed := 0

	var expiredCandidates []cleanupCandidate
	var existingCandidates []cleanupCandidate

	dc.mu.Lock()
	keys := dc.lru.Keys()
	keyCount := len(keys)
	if keyCount > 0 {
		start := dc.cleanupPos % keyCount
		scanCount := scanBudget
		if keyCount < scanCount {
			scanCount = keyCount
		}

		for i := 0; i < scanCount; i++ {
			idx := (start + i) % keyCount
			key := keys[idx]
			entry, ok := dc.lru.Peek(key)
			if !ok || entry == nil {
				continue
			}

			candidate := cleanupCandidate{key: key, path: entry.path}
			if now.After(entry.expiresAt) {
				expiredCandidates = append(expiredCandidates, candidate)
				if len(expiredCandidates) >= removeBudget {
					break
				}
				continue
			}

			if checkMissingFiles && len(existingCandidates) < missingCheckBudget {
				existingCandidates = append(existingCandidates, candidate)
			}
		}

		dc.cleanupPos = (start + scanCount) % keyCount
	} else {
		dc.cleanupPos = 0
	}
	dc.mu.Unlock()

	var missingCandidates []cleanupCandidate
	if checkMissingFiles {
		for _, candidate := range existingCandidates {
			if _, err := os.Stat(candidate.path); err != nil && os.IsNotExist(err) {
				missingCandidates = append(missingCandidates, candidate)
				if len(missingCandidates) >= removeBudget {
					break
				}
			}
		}
	}

	dc.mu.Lock()
	for _, candidate := range expiredCandidates {
		entry, ok := dc.lru.Peek(candidate.key)
		if !ok || entry == nil {
			continue
		}
		if entry.path == candidate.path && now.After(entry.expiresAt) {
			dc.lru.Remove(candidate.key)
			removed++
		}
	}

	for _, candidate := range missingCandidates {
		entry, ok := dc.lru.Peek(candidate.key)
		if !ok || entry == nil {
			continue
		}
		if entry.path == candidate.path {
			dc.lru.Remove(candidate.key)
			removed++
		}
	}

	removed += dc.evictForSizeLocked(1024)

	currentSize := dc.currentSize.Load()
	currentCount := dc.lru.Len()
	dc.mu.Unlock()

	return cleanupStats{
		removed:      removed,
		currentSize:  currentSize,
		currentCount: currentCount,
	}
}

func (dc *DiskCache) loadIndexFromDisk() {
	now := time.Now()
	totalScanned := 0
	deletedCount := 0

	_ = filepath.Walk(dc.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || filepath.Ext(path) != ".cache" {
			return nil
		}

		totalScanned++
		hash, expiresAt, parseErr := dc.parseCacheFilename(filepath.Base(path))
		if parseErr != nil || now.After(expiresAt) {
			if removeErr := os.Remove(path); removeErr == nil || os.IsNotExist(removeErr) {
				deletedCount++
			}
			dc.cleanupEmptyDirs(filepath.Dir(path))
			return nil
		}

		entry := &cacheEntry{
			hash:      hash,
			path:      path,
			size:      info.Size(),
			expiresAt: expiresAt,
		}

		dc.mu.Lock()
		dc.currentSize.Add(entry.size)
		dc.lru.Add(hash, entry)
		dc.evictForSizeLocked(2048)
		dc.mu.Unlock()

		return nil
	})

	logger.Infof("[DiskCache] Startup index load complete: scanned=%d, removed=%d, entries=%d, size=%v",
		totalScanned, deletedCount, dc.lru.Len(), formatBytes(dc.currentSize.Load()))
}

func (dc *DiskCache) initLRU() {
	lruIndex, err := simplelru.NewLRU[string, *cacheEntry](dc.MaxItems, func(_ string, entry *cacheEntry) {
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

// getHash generates BLAKE3 hash from key.
func (dc *DiskCache) getHash(key string) string {
	hash := blake3.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getDirPath generates directory path using hierarchical structure.
// Uses nginx-style levels=2:2 to limit files per directory.
func (dc *DiskCache) getDirPath(hashStr string) string {
	n := len(hashStr)
	level1 := hashStr[n-2 : n]
	level2 := hashStr[n-4 : n-2]
	return filepath.Join(dc.basePath, level1, level2)
}

// getFilePathWithExpiration generates cache file path with expiration in filename.
// Format: basePath/f1/8e/{hash}_{unixTimestamp}.cache
func (dc *DiskCache) getFilePathWithExpiration(hashStr string, expiresAt time.Time) string {
	dir := dc.getDirPath(hashStr)
	fileName := fmt.Sprintf("%s_%d.cache", hashStr, expiresAt.Unix())
	return filepath.Join(dir, fileName)
}

// parseCacheFilename extracts hash and expiration timestamp from filename.
// Format: {hash}_{unixTimestamp}.cache
func (dc *DiskCache) parseCacheFilename(filename string) (string, time.Time, error) {
	name := strings.TrimSuffix(filename, ".cache")
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore == -1 {
		return "", time.Time{}, fmt.Errorf("invalid filename format: %s", filename)
	}

	hash := name[:lastUnderscore]
	timestampStr := name[lastUnderscore+1:]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid timestamp in filename: %w", err)
	}

	return hash, time.Unix(timestamp, 0), nil
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
