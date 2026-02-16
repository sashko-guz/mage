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
type MemoryCacheOptions struct {
	Enabled    *bool `json:"enabled,omitempty"`
	MaxSizeMB  int   `json:"max_size_mb,omitempty"`
	MaxItems   int   `json:"max_items,omitempty"`
	TTLSeconds int   `json:"ttl_seconds,omitempty"`
}

// DiskCacheOptions defines configuration for disk-based cache
type DiskCacheOptions struct {
	Enabled        *bool              `json:"enabled,omitempty"`
	TTLSeconds     int                `json:"ttl_seconds,omitempty"`
	MaxSizeMB      int                `json:"max_size_mb,omitempty"`
	MaxItems       int                `json:"max_items,omitempty"`
	Dir            string             `json:"dir,omitempty"`
	ClearOnStartup *bool              `json:"clear_on_startup,omitempty"`
	AsyncWrite     *AsyncWriteOptions `json:"async_write,omitempty"`
}

// AsyncWriteOptions defines configuration for asynchronous disk cache writes
type AsyncWriteOptions struct {
	Enabled    *bool `json:"enabled,omitempty"`     // Enable async writes (default: true)
	NumWorkers int   `json:"num_workers,omitempty"` // Number of worker goroutines (default: 4)
	QueueSize  int   `json:"queue_size,omitempty"`  // Channel buffer size (default: 1000)
}

// CachePair defines separate memory and disk cache configuration for a cache layer (sources or thumbnails)
type CachePair struct {
	Memory *MemoryCacheOptions `json:"memory,omitempty"`
	Disk   *DiskCacheOptions   `json:"disk,omitempty"`
}

// StorageCacheConfig defines separate cache configurations for sources and thumbnails
type StorageCacheConfig struct {
	Sources *CachePair `json:"sources,omitempty"` // Cache for source images from storage
	Thumbs  *CachePair `json:"thumbs,omitempty"`  // Cache for generated thumbnails
}

// DiskCacheConfig contains configuration for disk-based cache
type DiskCacheConfig struct {
	Enabled        bool
	BasePath       string
	TTL            time.Duration
	ClearOnStartup bool
	MaxSizeMB      int // Maximum cache size in MB (0 = unlimited)
	MaxItems       int // Maximum number of items tracked in LRU index
	AsyncWrite     *AsyncWriteConfig
}

// AsyncWriteConfig contains configuration for asynchronous disk cache writes
type AsyncWriteConfig struct {
	Enabled    bool // Enable async writes
	NumWorkers int  // Number of worker goroutines
	QueueSize  int  // Channel buffer size
}

// MemoryCacheConfig contains configuration for in-memory cache
type MemoryCacheConfig struct {
	Enabled   bool
	MaxSizeMB int
	MaxItems  int
	TTL       time.Duration
}

// CachedStorageConfig contains separate cache configurations for sources and thumbnails
type CachedStorageConfig struct {
	// Source image caching
	SourceDiskCache   *DiskCacheConfig
	SourceMemoryCache *MemoryCacheConfig

	// Generated thumbnail caching
	ThumbDiskCache   *DiskCacheConfig
	ThumbMemoryCache *MemoryCacheConfig

	// Async write configurations
	SourceAsyncWrite *AsyncWriteConfig
	ThumbAsyncWrite  *AsyncWriteConfig
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
