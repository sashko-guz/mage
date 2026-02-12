package storage

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sashko-guz/mage/internal/cache"
	"github.com/sashko-guz/mage/internal/storage/drivers"
)

// NewStorage creates a fully configured storage with all cache layers applied
func NewStorage(cfg StorageItem) (Storage, error) {
	// Step 1: Create base storage (S3 or local)
	baseStorage, err := createBaseStorage(cfg)
	if err != nil {
		return nil, err
	}

	// Build log message parts
	var logParts []string
	logParts = append(logParts, fmt.Sprintf("driver: %s", cfg.Driver))

	// Step 2: Check if caching is configured
	if cfg.Cache == nil {
		log.Printf("[Storage:%s] Initialized (%s)", cfg.Name, strings.Join(logParts, ", "))
		return baseStorage, nil
	}

	// Check if any cache layer is enabled
	diskEnabled := cfg.Cache.Disk != nil && cfg.Cache.Disk.Enabled != nil && *cfg.Cache.Disk.Enabled
	memoryEnabled := cfg.Cache.Memory != nil && cfg.Cache.Memory.Enabled != nil && *cfg.Cache.Memory.Enabled

	if !diskEnabled && !memoryEnabled {
		log.Printf("[Storage:%s] Initialized (%s)", cfg.Name, strings.Join(logParts, ", "))
		return baseStorage, nil
	}

	// Add cache info to log
	var cacheInfo []string
	if memoryEnabled && cfg.Cache.Memory.MaxSizeMB > 0 {
		cacheInfo = append(cacheInfo, fmt.Sprintf("memory: %dMB", cfg.Cache.Memory.MaxSizeMB))
	}
	if diskEnabled {
		cacheInfo = append(cacheInfo, "disk")
	}
	if len(cacheInfo) > 0 {
		logParts = append(logParts, fmt.Sprintf("cache: %s", strings.Join(cacheInfo, ", ")))
	}

	// Step 3: Wrap with cache layers
	cachedStorage, err := wrapWithCache(baseStorage, cfg)
	if err != nil {
		return nil, err
	}

	log.Printf("[Storage:%s] Initialized (%s)", cfg.Name, strings.Join(logParts, ", "))
	return cachedStorage, nil
}

// createBaseStorage creates the underlying storage driver (S3 or local)
func createBaseStorage(cfg StorageItem) (Storage, error) {
	switch cfg.Driver {
	case DriverS3:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage '%s': bucket is required for S3 driver", cfg.Name)
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1" // default region
		}
		// Require credentials when using custom base_url (S3-compatible storage)
		if cfg.BaseURL != "" && (cfg.AccessKey == "" || cfg.SecretKey == "") {
			return nil, fmt.Errorf("storage '%s': access_key and secret_key are required when using base_url for S3-compatible storage", cfg.Name)
		}
		return drivers.NewS3Client(cfg.Region, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.BaseURL, cfg.S3HTTPConfig)

	case DriverLocal:
		if cfg.Root == "" {
			return nil, fmt.Errorf("storage '%s': root is required for local driver", cfg.Name)
		}
		return drivers.NewLocalStorage(cfg.Root)

	default:
		return nil, fmt.Errorf("storage '%s': unknown driver '%s'", cfg.Name, cfg.Driver)
	}
}

// wrapWithCache wraps the base storage with cache layers (disk and/or memory)
func wrapWithCache(baseStorage Storage, cfg StorageItem) (Storage, error) {
	diskEnabled := cfg.Cache.Disk != nil && cfg.Cache.Disk.Enabled != nil && *cfg.Cache.Disk.Enabled
	memoryEnabled := cfg.Cache.Memory != nil && cfg.Cache.Memory.Enabled != nil && *cfg.Cache.Memory.Enabled

	if !diskEnabled && !memoryEnabled {
		return baseStorage, nil
	}

	// Disk cache is required as the base layer
	if !diskEnabled {
		return nil, fmt.Errorf("storage '%s': disk cache must be enabled when using cache", cfg.Name)
	}

	if cfg.Cache.Disk.Dir == "" {
		return nil, fmt.Errorf("storage '%s': cache dir is required when cache is enabled", cfg.Name)
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cfg.Cache.Disk.Dir, 0755); err != nil {
		return nil, fmt.Errorf("storage '%s': failed to create cache directory: %w", cfg.Name, err)
	}

	// Build disk cache configuration
	cacheTTL := 5 * time.Minute // default
	if cfg.Cache.Disk.TTLSeconds > 0 {
		cacheTTL = time.Duration(cfg.Cache.Disk.TTLSeconds) * time.Second
	}

	clearOnStartup := false
	if cfg.Cache.Disk.ClearOnStartup != nil {
		clearOnStartup = *cfg.Cache.Disk.ClearOnStartup
	}

	cacheConfig := CachedStorageConfig{
		StorageName: cfg.Name,
		DiskCache: &DiskCacheConfig{
			Enabled:        true,
			BasePath:       cfg.Cache.Disk.Dir,
			TTL:            cacheTTL,
			ClearOnStartup: clearOnStartup,
			MaxSizeMB:      cfg.Cache.Disk.MaxSizeMB,
		},
	}

	// Add memory cache configuration if enabled
	if memoryEnabled {
		maxItems := cfg.Cache.Memory.MaxItems
		if maxItems == 0 {
			maxItems = 1000 // default
		}
		cacheConfig.MemoryCache = &MemoryCacheConfig{
			Enabled:   true,
			MaxSizeMB: cfg.Cache.Memory.MaxSizeMB,
			MaxItems:  maxItems,
		}
	}

	// Create cached storage with both layers
	return newCachedStorage(baseStorage, cacheConfig)
}

// newCachedStorage creates a wrapped storage with multi-layer caching
func newCachedStorage(underlying Storage, cfg CachedStorageConfig) (*CachedStorage, error) {
	// Validate configuration
	if cfg.DiskCache == nil || !cfg.DiskCache.Enabled {
		return nil, fmt.Errorf("disk cache configuration is required and must be enabled")
	}

	if cfg.DiskCache.BasePath == "" {
		return nil, fmt.Errorf("disk cache base path is required")
	}

	cs := &CachedStorage{
		underlying:  underlying,
		storageName: cfg.StorageName,
	}

	// Initialize memory cache first (if enabled and configured)
	if cfg.MemoryCache != nil && cfg.MemoryCache.Enabled && cfg.MemoryCache.MaxSizeMB > 0 {
		memorySizeBytes := int64(cfg.MemoryCache.MaxSizeMB) * 1024 * 1024
		maxItems := int64(cfg.MemoryCache.MaxItems)

		// Use reasonable default for MaxItems if not specified
		if maxItems == 0 {
			maxItems = int64(cfg.MemoryCache.MaxSizeMB) // Estimate: ~1MB per item
		}

		memCache, err := cache.NewMemoryCache(cache.MemoryCacheConfig{
			Name:     cfg.StorageName,
			MaxSize:  memorySizeBytes,
			MaxItems: maxItems,
			TTL:      cfg.DiskCache.TTL,
		})
		if err != nil {
			log.Printf("[CachedStorage:%s] Failed to init memory cache: %v", cfg.StorageName, err)
		} else {
			cs.memoryCache = memCache
			log.Printf("[CachedStorage:%s] Enabled in-memory cache: %dMB, max items: %d", cfg.StorageName, cfg.MemoryCache.MaxSizeMB, maxItems)
		}
	}

	// Initialize disk cache second
	// Convert MB to bytes (0 = unlimited)
	diskCacheMaxBytes := int64(0)
	if cfg.DiskCache.MaxSizeMB > 0 {
		diskCacheMaxBytes = int64(cfg.DiskCache.MaxSizeMB) * 1024 * 1024
	}

	diskCache, err := cache.NewDiskCache(
		cfg.DiskCache.BasePath,
		cfg.DiskCache.TTL,
		cfg.DiskCache.ClearOnStartup,
		cfg.StorageName,
		diskCacheMaxBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk cache: %w", err)
	}

	cs.diskCache = diskCache
	return cs, nil
}

// InitializeStorages creates all configured storages from the configuration
func InitializeStorages(config *StorageConfig) (map[string]Storage, error) {
	if len(config.Storages) == 0 {
		return nil, fmt.Errorf("no storages configured")
	}

	log.Printf("[Storage] Initializing %d storage(s)…", len(config.Storages))

	storages := make(map[string]Storage)
	for _, cfg := range config.Storages {
		store, err := NewStorage(cfg)
		if err != nil {
			return nil, fmt.Errorf("storage '%s': %w", cfg.Name, err)
		}
		storages[cfg.Name] = store
	}

	log.Printf("[Storage] All storages initialized successfully")
	return storages, nil
}
