package storage

import (
	"os"
	"strconv"
	"time"

	"github.com/sashko-guz/mage/internal/storage/drivers"
)

type StorageDriver string

const (
	DriverS3    StorageDriver = "s3"
	DriverLocal StorageDriver = "local"
)

type StorageConfig struct {
	Driver StorageDriver

	// S3 specific fields
	Bucket       string
	Region       string
	AccessKey    string
	SecretKey    string
	BaseURL      string
	UsePathStyle bool
	S3HTTPConfig *drivers.S3HTTPConfig

	// Local specific fields
	Root string

	// Cache configuration
	Cache *StorageCacheConfig
}

// StorageCacheConfig defines separate cache configurations for sources and thumbnails
type StorageCacheConfig struct {
	Sources *CachePair
	Thumbs  *CachePair
}

// CachePair defines separate memory and disk cache configuration for a cache layer
type CachePair struct {
	Memory *MemoryCacheOptions
	Disk   *DiskCacheOptions
}

// MemoryCacheOptions defines configuration for in-memory cache
type MemoryCacheOptions struct {
	Enabled   bool
	MaxSizeMB int
	MaxItems  int
	TTL       time.Duration
}

// DiskCacheOptions defines configuration for disk-based cache
type DiskCacheOptions struct {
	Enabled        bool
	TTL            time.Duration
	MaxSizeMB      int
	MaxItems       int
	Dir            string
	ClearOnStartup bool
	AsyncWrite     *AsyncWriteOptions
}

// AsyncWriteOptions defines configuration for asynchronous disk cache writes
type AsyncWriteOptions struct {
	Enabled    bool
	NumWorkers int
	QueueSize  int
}

// DiskCacheConfig contains configuration for disk-based cache (internal use)
type DiskCacheConfig struct {
	Enabled        bool
	BasePath       string
	TTL            time.Duration
	ClearOnStartup bool
	MaxSizeMB      int
	MaxItems       int
	AsyncWrite     *AsyncWriteConfig
}

// AsyncWriteConfig contains configuration for asynchronous disk cache writes (internal use)
type AsyncWriteConfig struct {
	Enabled    bool
	NumWorkers int
	QueueSize  int
}

// MemoryCacheConfig contains configuration for in-memory cache (internal use)
type MemoryCacheConfig struct {
	Enabled   bool
	MaxSizeMB int
	MaxItems  int
	TTL       time.Duration
}

// CachedStorageConfig contains separate cache configurations for sources and thumbnails (internal use)
type CachedStorageConfig struct {
	SourceDiskCache   *DiskCacheConfig
	SourceMemoryCache *MemoryCacheConfig
	ThumbDiskCache    *DiskCacheConfig
	ThumbMemoryCache  *MemoryCacheConfig
	SourceAsyncWrite  *AsyncWriteConfig
	ThumbAsyncWrite   *AsyncWriteConfig
}

// LoadConfig loads storage configuration from environment variables
func LoadConfig() *StorageConfig {
	cfg := &StorageConfig{
		Driver: StorageDriver(getEnv("STORAGE_DRIVER", "local")),

		// Local storage
		Root: getEnv("STORAGE_ROOT", ""),

		// S3 storage
		Bucket:       getEnv("S3_BUCKET", ""),
		Region:       getEnv("S3_REGION", "us-east-1"),
		AccessKey:    getEnv("S3_ACCESS_KEY", ""),
		SecretKey:    getEnv("S3_SECRET_KEY", ""),
		BaseURL:      getEnv("S3_BASE_URL", ""),
		UsePathStyle: getEnvBool("S3_USE_PATH_STYLE", false),

		// S3 HTTP config
		S3HTTPConfig: &drivers.S3HTTPConfig{
			MaxIdleConns:          getEnvInt("S3_MAX_IDLE_CONNS", 100),
			MaxIdleConnsPerHost:   getEnvInt("S3_MAX_IDLE_CONNS_PER_HOST", 100),
			MaxConnsPerHost:       getEnvInt("S3_MAX_CONNS_PER_HOST", 0),
			IdleConnTimeout:       getEnvInt("S3_IDLE_CONN_TIMEOUT_SEC", 90),
			ConnectTimeout:        getEnvInt("S3_CONNECT_TIMEOUT_SEC", 10),
			RequestTimeout:        getEnvInt("S3_REQUEST_TIMEOUT_SEC", 30),
			ResponseHeaderTimeout: getEnvInt("S3_RESPONSE_HEADER_TIMEOUT_SEC", 10),
		},
	}

	// Load cache configuration
	cfg.Cache = loadCacheConfig()

	return cfg
}

func loadCacheConfig() *StorageCacheConfig {
	sources := loadCachePair("SOURCE")
	thumbs := loadCachePair("THUMB")

	if sources == nil && thumbs == nil {
		return nil
	}

	return &StorageCacheConfig{
		Sources: sources,
		Thumbs:  thumbs,
	}
}

func loadCachePair(prefix string) *CachePair {
	memory := loadMemoryCacheOptions(prefix)
	disk := loadDiskCacheOptions(prefix)

	if memory == nil && disk == nil {
		return nil
	}

	return &CachePair{
		Memory: memory,
		Disk:   disk,
	}
}

func loadMemoryCacheOptions(prefix string) *MemoryCacheOptions {
	enabled := getEnvBool(prefix+"_MEMORY_CACHE_ENABLED", false)
	if !enabled {
		return nil
	}

	return &MemoryCacheOptions{
		Enabled:   true,
		MaxSizeMB: getEnvInt(prefix+"_MEMORY_CACHE_MAX_SIZE_MB", 256),
		MaxItems:  getEnvInt(prefix+"_MEMORY_CACHE_MAX_ITEMS", 1000),
		TTL:       time.Duration(getEnvInt(prefix+"_MEMORY_CACHE_TTL_SEC", 300)) * time.Second,
	}
}

func loadDiskCacheOptions(prefix string) *DiskCacheOptions {
	enabled := getEnvBool(prefix+"_DISK_CACHE_ENABLED", false)
	if !enabled {
		return nil
	}

	return &DiskCacheOptions{
		Enabled:        true,
		Dir:            getEnv(prefix+"_DISK_CACHE_DIR", ""),
		MaxSizeMB:      getEnvInt(prefix+"_DISK_CACHE_MAX_SIZE_MB", 2048),
		MaxItems:       getEnvInt(prefix+"_DISK_CACHE_MAX_ITEMS", 100000),
		TTL:            time.Duration(getEnvInt(prefix+"_DISK_CACHE_TTL_SEC", 600)) * time.Second,
		ClearOnStartup: getEnvBool(prefix+"_DISK_CACHE_CLEAR_ON_STARTUP", false),
		AsyncWrite: &AsyncWriteOptions{
			Enabled:    getEnvBool(prefix+"_DISK_CACHE_ASYNC_ENABLED", true),
			NumWorkers: getEnvInt(prefix+"_DISK_CACHE_ASYNC_WORKERS", 4),
			QueueSize:  getEnvInt(prefix+"_DISK_CACHE_ASYNC_QUEUE_SIZE", 1000),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return defaultValue
	}
	return parsed
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
