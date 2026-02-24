package disk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashko-guz/mage/internal/logger"
)

// -------------------------------------------------------------------
// File & delete helpers
// -------------------------------------------------------------------

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
