package disk

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lukechampine.com/blake3"
)

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
