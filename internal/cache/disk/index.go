package disk

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sashko-guz/mage/internal/format"
	"github.com/sashko-guz/mage/internal/logger"
)

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
