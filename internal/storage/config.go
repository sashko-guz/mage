package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sashko-guz/mage/internal/storage/drivers"
)

type StorageDriver string

const (
	DriverS3    StorageDriver = "s3"
	DriverLocal StorageDriver = "local"
)

type StorageConfig struct {
	Driver StorageDriver `json:"driver"`

	// Cache configuration
	Cache *StorageCacheConfig `json:"cache,omitempty"`

	// Signature secret key for HMAC signature validation (optional)
	SignatureSecret string `json:"signature_secret,omitempty"`

	// S3 specific fields
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	BaseURL   string `json:"base_url,omitempty"` // Custom endpoint for S3-compatible storage

	// S3 HTTP client tuning (optional, with sensible defaults)
	S3HTTPConfig *drivers.S3HTTPConfig `json:"s3_http_config,omitempty"`

	// Local specific fields
	Root string `json:"root,omitempty"`
}

// MemoryCacheOptions defines configuration for in-memory cache
// This cache stores both source images and generated thumbnails with a unified size limit
type MemoryCacheOptions struct {
	Enabled    *bool `json:"enabled,omitempty"`
	MaxSizeMB  int   `json:"max_size_mb,omitempty"`  // Total memory limit for sources + thumbnails
	MaxItems   int   `json:"max_items,omitempty"`    // Maximum number of cached items (sources + thumbnails)
	TTLSeconds int   `json:"ttl_seconds,omitempty"` // Time-to-live for cache entries in seconds
}

// DiskCacheOptions defines configuration for disk-based cache
// This cache stores both source images and generated thumbnails with a unified size limit
type DiskCacheOptions struct {
	Enabled        *bool  `json:"enabled,omitempty"`
	TTLSeconds     int    `json:"ttl_seconds,omitempty"`
	MaxSizeMB      int    `json:"max_size_mb,omitempty"` // Total disk limit for sources + thumbnails (0 = unlimited)
	Dir            string `json:"dir,omitempty"`
	ClearOnStartup *bool  `json:"clear_on_startup,omitempty"`
}

type StorageCacheConfig struct {
	Memory *MemoryCacheOptions `json:"memory,omitempty"`
	Disk   *DiskCacheOptions   `json:"disk,omitempty"`
}

// DiskCacheConfig contains configuration for disk-based cache
// The cache stores both source images and thumbnails with a unified size limit
type DiskCacheConfig struct {
	Enabled        bool
	BasePath       string
	TTL            time.Duration
	ClearOnStartup bool
	MaxSizeMB      int // Maximum total cache size for sources + thumbnails in MB (0 = unlimited)
}

// MemoryCacheConfig contains configuration for in-memory cache
// The cache stores both source images and thumbnails with a unified size limit
type MemoryCacheConfig struct {
	Enabled   bool
	MaxSizeMB int           // Maximum total memory for sources + thumbnails in megabytes
	MaxItems  int           // Maximum number of cached items (sources + thumbnails combined)
	TTL       time.Duration // Time-to-live for cache entries
}

// CachedStorageConfig contains configuration for cached storage
type CachedStorageConfig struct {
	DiskCache   *DiskCacheConfig
	MemoryCache *MemoryCacheConfig
}

func LoadConfig(configPath string) (*StorageConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config StorageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}
