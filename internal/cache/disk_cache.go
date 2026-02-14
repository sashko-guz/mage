package cache

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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

// DiskCache provides disk-based caching with TTL support
type DiskCache struct {
	basePath string
	MaxSize  int64         // Maximum cache size in bytes (0 = unlimited)
	TTL      time.Duration // Exported so CachedStorage can access it
	mu       sync.RWMutex
}

type cleanupStats struct {
	totalFiles   int
	totalSize    int64 // Total cache size in bytes
	keptCount    int
	keptSize     int64
	deletedCount int
	deletedSize  int64
	errorCount   int
}

type cacheFile struct {
	path      string
	size      int64
	expiresAt time.Time
}

// NewDiskCache creates a new disk-based cache
// basePath is the directory where cache files will be stored
// ttl is the time-to-live for cache entries
// clearOnStartup: if true, removes ALL cache files on startup (useful for testing or ensuring fresh start)
// maxSizeBytes is the maximum cache size in bytes (0 = unlimited). Cleanup will evict oldest files if exceeded.
func NewDiskCache(basePath string, ttl time.Duration, clearOnStartup bool, maxSizeBytes int64) (*DiskCache, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dc := &DiskCache{
		basePath: absPath,
		MaxSize:  maxSizeBytes,
		TTL:      ttl,
	}

	if clearOnStartup {
		// Clear ALL cache files on startup
		log.Printf("[DiskCache] Clearing all cache files in %s (clearOnStartup=true)", absPath)
		if err := dc.Clear(); err != nil {
			log.Printf("[DiskCache] Error during startup cache clear: %v", err)
		}
	} else {
		// Run cleanup synchronously at startup to clear only expired entries before serving requests
		log.Printf("[DiskCache] Running initial cleanup for %s", absPath)
		dc.performCleanup()
	}

	// Start background cleanup goroutine
	go dc.cleanupExpired()

	log.Printf("[DiskCache] Initialized: BasePath=%s, TTL=%v, MaxSize=%v, CleanupInterval=30s (adaptive backoff)", absPath, ttl, formatBytes(maxSizeBytes))
	return dc, nil
}

// Get retrieves a cached item by key
func (dc *DiskCache) Get(key string) ([]byte, error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	hash := dc.getHash(key)
	dir := dc.getDirPath(hash)

	// Find cache file in directory
	filePath, expiresAt, err := dc.findCacheFile(dir, hash)
	if err != nil {
		return nil, ErrCacheNotFound
	}

	// Check if entry has expired
	now := time.Now()
	if now.After(expiresAt) {
		log.Printf("[DiskCache] Cache entry expired for key: %s (expired at %v)", key, expiresAt.Format(time.RFC3339))
		// Delete immediately
		go func() {
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Printf("[DiskCache] Error deleting expired cache file %s: %v", filePath, err)
			} else {
				log.Printf("[DiskCache] Deleted expired cache file: %s", filePath)
			}
		}()
		return nil, ErrCacheNotFound
	}

	// Read raw data directly (no JSON parsing!)
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[DiskCache] Error reading cache file %s: %v", filePath, err)
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	log.Printf("[DiskCache] Cache HIT for key: %s (expires at %v)", key, expiresAt.Format(time.RFC3339))
	return data, nil
}

// Set stores a cached item by key with TTL
func (dc *DiskCache) Set(key string, data []byte) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	expiresAt := time.Now().Add(dc.TTL)
	hash := dc.getHash(key)
	filePath := dc.getFilePathWithExpiration(hash, expiresAt)

	// Ensure the nested directory structure exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory structure: %w", err)
	}

	// Write raw data directly (no JSON marshalling!)
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	log.Printf("[DiskCache] Cached data for key: %s (expires at %v, TTL: %v)", key, expiresAt.Format(time.RFC3339), dc.TTL)
	return nil
}

// Delete removes a cache entry
func (dc *DiskCache) Delete(key string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	hash := dc.getHash(key)
	dir := dc.getDirPath(hash)

	// Find and delete cache file
	filePath, _, err := dc.findCacheFile(dir, hash)
	if err != nil {
		// File not found, which is okay for delete
		return nil
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}
	return nil
}

// Clear removes all cache entries
func (dc *DiskCache) Clear() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Remove the entire cache directory structure and recreate it
	if err := os.RemoveAll(dc.basePath); err != nil {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}

	if err := os.MkdirAll(dc.basePath, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}

	log.Printf("[DiskCache] Cache cleared")
	return nil
}

// cleanupExpired periodically removes expired cache entries
func (dc *DiskCache) cleanupExpired() {
	const baseInterval = 30 * time.Second
	const increaseInterval = 10 * time.Second
	const maxInterval = 10 * time.Minute
	const lowDeletionRateThreshold = 0.05 // 5% - only backoff if deletion rate is very low

	interval := baseInterval
	timer := time.NewTimer(interval)
	defer timer.Stop()

	log.Printf("[DiskCache] Cleanup goroutine started for %s, next cleanup in %v", dc.basePath, interval)

	for {
		<-timer.C
		stats := dc.performCleanup()

		// Calculate deletion rate to determine if we should back off
		var deletionRate float64
		if stats.totalFiles > 0 {
			deletionRate = float64(stats.deletedCount) / float64(stats.totalFiles)
		}

		// Only increase interval if deletion rate is very low (few files expiring)
		// High deletion rate means cache is turning over, keep cleanup frequent
		if deletionRate < lowDeletionRateThreshold && stats.deletedCount == 0 {
			interval += increaseInterval
			if interval > maxInterval {
				interval = maxInterval
			}
		} else if deletionRate >= lowDeletionRateThreshold || stats.deletedCount > 0 {
			// Reset to base interval if we're actively cleaning up files
			interval = baseInterval
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(interval)
		log.Printf("[DiskCache] %s - Next cleanup in %v (deleted: %d/%d, rate: %.2f%%, kept: %d)", dc.basePath, interval, stats.deletedCount, stats.totalFiles, deletionRate*100, stats.keptCount)
	}
}

func (dc *DiskCache) performCleanup() cleanupStats {
	now := time.Now()
	stats := cleanupStats{}
	var validFiles []cacheFile // Track valid files for size-based eviction

	log.Printf("[DiskCache] Starting cleanup scan in %s", dc.basePath)

	// First pass: delete expired files and collect valid files with their sizes
	err := filepath.Walk(dc.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-cache files
		if filepath.Ext(path) != ".cache" {
			return nil
		}

		stats.totalFiles++
		fileSize := info.Size()
		stats.totalSize += fileSize

		// Parse expiration from filename (no file I/O needed!)
		expiresAt, err := dc.parseExpirationFromFilename(filepath.Base(path))
		if err != nil {
			// Corrupted filename, delete it
			if err := os.Remove(path); err == nil {
				stats.deletedCount++
				stats.deletedSize += fileSize
			} else {
				stats.errorCount++
				log.Printf("[DiskCache] Error deleting corrupted entry %s: %v", path, err)
			}
			return nil
		}

		// Check if expired
		if now.After(expiresAt) {
			// Lock only for the delete operation
			dc.mu.Lock()
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("[DiskCache] Error deleting expired cache file %s: %v", path, err)
				stats.errorCount++
			} else {
				stats.deletedCount++
				stats.deletedSize += fileSize
			}
			dc.mu.Unlock()

			// Clean up empty parent directories
			dc.cleanupEmptyDirs(filepath.Dir(path))
		} else {
			stats.keptCount++
			stats.keptSize += fileSize
			// Collect valid files for potential size-based eviction
			validFiles = append(validFiles, cacheFile{
				path:      path,
				size:      fileSize,
				expiresAt: expiresAt,
			})
		}

		return nil
	})

	if err != nil {
		log.Printf("[DiskCache] Error during cleanup walk: %v", err)
	}

	// Second pass: if MaxSize is set and exceeded, delete oldest files
	if dc.MaxSize > 0 && stats.keptSize > dc.MaxSize {
		log.Printf("[DiskCache] Cache size exceeded: %v > %v, evicting oldest files",
			formatBytes(stats.keptSize), formatBytes(dc.MaxSize))

		// Sort by expiration time (oldest first)
		sort.Slice(validFiles, func(i, j int) bool {
			return validFiles[i].expiresAt.Before(validFiles[j].expiresAt)
		})

		// Delete oldest files until we're under limit
		for _, file := range validFiles {
			if stats.keptSize <= dc.MaxSize {
				break
			}

			dc.mu.Lock()
			if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
				log.Printf("[DiskCache] Error deleting file during size-based eviction %s: %v", file.path, err)
				stats.errorCount++
				dc.mu.Unlock()
				continue
			}
			dc.mu.Unlock()

			stats.deletedCount++
			stats.deletedSize += file.size
			stats.keptCount--
			stats.keptSize -= file.size

			// Clean up empty parent directories
			dc.cleanupEmptyDirs(filepath.Dir(file.path))
		}

		log.Printf("[DiskCache] Size-based eviction complete: freed %v (now %v)",
			formatBytes(stats.deletedSize), formatBytes(stats.keptSize))
	}

	log.Printf("[DiskCache] Cleanup complete: scanned %d files (%v), kept %d (%v), deleted %d (%v), errors %d",
		stats.totalFiles, formatBytes(stats.totalSize), stats.keptCount, formatBytes(stats.keptSize), stats.deletedCount, formatBytes(stats.deletedSize), stats.errorCount)
	return stats
}

// getHash generates BLAKE3 hash from key
func (dc *DiskCache) getHash(key string) string {
	hash := blake3.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getDirPath generates directory path using hierarchical structure
// Uses nginx-style levels=2:2 to limit files per directory
func (dc *DiskCache) getDirPath(hashStr string) string {
	n := len(hashStr)
	level1 := hashStr[n-2 : n]   // last 2 chars
	level2 := hashStr[n-4 : n-2] // next 2 chars
	return filepath.Join(dc.basePath, level1, level2)
}

// getFilePathWithExpiration generates cache file path with expiration in filename
// Format: basePath/f1/8e/{hash}_{unixTimestamp}.cache
// Example: basePath/f1/8e/b7f4c9d3e8a1f6c2d5e9a0b3c4d7e8f1_1738789123.cache
func (dc *DiskCache) getFilePathWithExpiration(hashStr string, expiresAt time.Time) string {
	dir := dc.getDirPath(hashStr)
	fileName := fmt.Sprintf("%s_%d.cache", hashStr, expiresAt.Unix())
	return filepath.Join(dir, fileName)
}

// findCacheFile finds a cache file in directory matching hash prefix
func (dc *DiskCache) findCacheFile(dir, hashStr string) (string, time.Time, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}, err
	}

	prefix := hashStr + "_"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".cache") {
			expiresAt, err := dc.parseExpirationFromFilename(name)
			if err != nil {
				continue
			}
			return filepath.Join(dir, name), expiresAt, nil
		}
	}
	return "", time.Time{}, fmt.Errorf("cache file not found")
}

// parseExpirationFromFilename extracts expiration timestamp from filename
// Format: {hash}_{unixTimestamp}.cache
func (dc *DiskCache) parseExpirationFromFilename(filename string) (time.Time, error) {
	// Remove .cache extension
	name := strings.TrimSuffix(filename, ".cache")

	// Find last underscore
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore == -1 {
		return time.Time{}, fmt.Errorf("invalid filename format: %s", filename)
	}

	// Parse timestamp
	timestampStr := name[lastUnderscore+1:]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp in filename: %w", err)
	}

	return time.Unix(timestamp, 0), nil
}

// CacheStats returns information about cache usage
func (dc *DiskCache) CacheStats() (count int, totalSize int64, err error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	// Walk through the hierarchical directory structure
	err = filepath.Walk(dc.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".cache" {
			count++
			totalSize += info.Size()
		}

		return nil
	})

	return count, totalSize, err
}

// cleanupEmptyDirs removes empty directories up to the base path
func (dc *DiskCache) cleanupEmptyDirs(dir string) {
	// Don't remove the base path itself
	if dir == dc.basePath || !filepath.HasPrefix(dir, dc.basePath) {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}

	// Directory is empty, remove it
	if err := os.Remove(dir); err == nil {
		// Recursively clean parent directories
		dc.cleanupEmptyDirs(filepath.Dir(dir))
	}
}
