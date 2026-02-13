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
func NewStorage(cfg *StorageConfig) (Storage, string, error) {
	// Step 1: Create base storage (S3 or local)
	baseStorage, err := createBaseStorage(cfg)
	if err != nil {
		return nil, "", err
	}

	// Build log message parts
	var logParts []string
	logParts = append(logParts, fmt.Sprintf("Driver=%s", cfg.Driver))

	// Step 2: Check if caching is configured
	if cfg.Cache == nil {
		log.Printf("[Cache] No cache enabled")
		log.Printf("[Storage] Initialized (%s)", strings.Join(logParts, ", "))
		return baseStorage, cfg.SignatureSecret, nil
	}

	// Check if any cache layer is enabled
	diskEnabled := cfg.Cache.Disk != nil && cfg.Cache.Disk.Enabled != nil && *cfg.Cache.Disk.Enabled
	memoryEnabled := cfg.Cache.Memory != nil && cfg.Cache.Memory.Enabled != nil && *cfg.Cache.Memory.Enabled

	if !diskEnabled && !memoryEnabled {
		log.Printf("[Cache] No cache enabled")
		log.Printf("[Storage] Initialized (%s)", strings.Join(logParts, ", "))
		return baseStorage, cfg.SignatureSecret, nil
	}

	// Add cache info to log
	var cacheInfo []string
	if memoryEnabled && cfg.Cache.Memory.MaxSizeMB > 0 {
		cacheInfo = append(cacheInfo, fmt.Sprintf("Memory:%dMB", cfg.Cache.Memory.MaxSizeMB))
	}
	if diskEnabled {
		if cfg.Cache.Disk.MaxSizeMB > 0 {
			cacheInfo = append(cacheInfo, fmt.Sprintf("Disk:%dMB", cfg.Cache.Disk.MaxSizeMB))
		} else {
			cacheInfo = append(cacheInfo, "Disk:unlimited")
		}
	}
	if len(cacheInfo) > 0 {
		logParts = append(logParts, fmt.Sprintf("Cache=[%s]", strings.Join(cacheInfo, "+")))
	}

	// Step 3: Wrap with cache layers
	cachedStorage, err := wrapWithCache(baseStorage, cfg)
	if err != nil {
		return nil, "", err
	}

	log.Printf("[Storage] Initialized (%s)", strings.Join(logParts, ", "))
	return cachedStorage, cfg.SignatureSecret, nil
}

// createBaseStorage creates the underlying storage driver (S3 or local)
func createBaseStorage(cfg *StorageConfig) (Storage, error) {
	switch cfg.Driver {
	case DriverS3:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("bucket is required for S3 driver")
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1" // default region
		}
		// Require credentials when using custom base_url (S3-compatible storage)
		if cfg.BaseURL != "" && (cfg.AccessKey == "" || cfg.SecretKey == "") {
			return nil, fmt.Errorf("access_key and secret_key are required when using base_url for S3-compatible storage")
		}
		return drivers.NewS3Client(cfg.Region, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.BaseURL, cfg.S3HTTPConfig)

	case DriverLocal:
		if cfg.Root == "" {
			return nil, fmt.Errorf("root is required for local driver")
		}
		return drivers.NewLocalStorage(cfg.Root)

	default:
		return nil, fmt.Errorf("unknown driver '%s'", cfg.Driver)
	}
}

// wrapWithCache wraps the base storage with cache layers (disk and/or memory)
func wrapWithCache(baseStorage Storage, cfg *StorageConfig) (Storage, error) {
	diskEnabled := cfg.Cache.Disk != nil && cfg.Cache.Disk.Enabled != nil && *cfg.Cache.Disk.Enabled
	memoryEnabled := cfg.Cache.Memory != nil && cfg.Cache.Memory.Enabled != nil && *cfg.Cache.Memory.Enabled

	// If neither cache is enabled, return base storage without caching
	if !diskEnabled && !memoryEnabled {
		return baseStorage, nil
	}

	// Log cache mode
	if diskEnabled && memoryEnabled {
		log.Printf("[Cache] Memory & Disk cache enabled")
	} else if memoryEnabled {
		log.Printf("[Cache] Memory-only cache enabled")
	} else if diskEnabled {
		log.Printf("[Cache] Disk-only cache enabled")
	}

	cacheConfig := CachedStorageConfig{}

	// Determine default TTL
	defaultTTL := 5 * time.Minute

	// Configure disk cache if enabled
	if diskEnabled {
		if cfg.Cache.Disk.Dir == "" {
			return nil, fmt.Errorf("cache dir is required when disk cache is enabled")
		}

		// Create cache directory if it doesn't exist
		if err := os.MkdirAll(cfg.Cache.Disk.Dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}

		// Build disk cache configuration
		diskTTL := defaultTTL
		if cfg.Cache.Disk.TTLSeconds > 0 {
			diskTTL = time.Duration(cfg.Cache.Disk.TTLSeconds) * time.Second
		}

		clearOnStartup := false
		if cfg.Cache.Disk.ClearOnStartup != nil {
			clearOnStartup = *cfg.Cache.Disk.ClearOnStartup
		}

		cacheConfig.DiskCache = &DiskCacheConfig{
			Enabled:        true,
			BasePath:       cfg.Cache.Disk.Dir,
			TTL:            diskTTL,
			ClearOnStartup: clearOnStartup,
			MaxSizeMB:      cfg.Cache.Disk.MaxSizeMB,
		}
	}

	// Add memory cache configuration if enabled
	if memoryEnabled {
		maxItems := cfg.Cache.Memory.MaxItems
		if maxItems == 0 {
			maxItems = 1000 // default
		}

		// Configure memory cache TTL
		// Priority: memory config > disk config > default
		memoryTTL := defaultTTL
		if cfg.Cache.Memory.TTLSeconds > 0 {
			memoryTTL = time.Duration(cfg.Cache.Memory.TTLSeconds) * time.Second
		} else if diskEnabled && cfg.Cache.Disk.TTLSeconds > 0 {
			memoryTTL = time.Duration(cfg.Cache.Disk.TTLSeconds) * time.Second
		}

		cacheConfig.MemoryCache = &MemoryCacheConfig{
			Enabled:   true,
			MaxSizeMB: cfg.Cache.Memory.MaxSizeMB,
			MaxItems:  maxItems,
			TTL:       memoryTTL,
		}
	}

	// Create cached storage with both layers
	return newCachedStorage(baseStorage, cacheConfig)
}

// newCachedStorage creates a wrapped storage with multi-layer caching
func newCachedStorage(underlying Storage, cfg CachedStorageConfig) (*CachedStorage, error) {
	// Validate that at least one cache is enabled
	if (cfg.DiskCache == nil || !cfg.DiskCache.Enabled) && (cfg.MemoryCache == nil || !cfg.MemoryCache.Enabled) {
		return nil, fmt.Errorf("at least one cache (disk or memory) must be enabled")
	}

	// Validate disk cache configuration if enabled
	if cfg.DiskCache != nil && cfg.DiskCache.Enabled && cfg.DiskCache.BasePath == "" {
		return nil, fmt.Errorf("disk cache base path is required when disk cache is enabled")
	}

	cs := &CachedStorage{
		underlying: underlying,
	}

	// Determine TTL - prefer memory cache TTL, then disk cache TTL, otherwise use default
	cacheTTL := 5 * time.Minute // default
	if cfg.MemoryCache != nil && cfg.MemoryCache.Enabled && cfg.MemoryCache.TTL > 0 {
		cacheTTL = cfg.MemoryCache.TTL
	} else if cfg.DiskCache != nil && cfg.DiskCache.Enabled {
		cacheTTL = cfg.DiskCache.TTL
	}
	cs.ttl = cacheTTL

	// Initialize memory cache first (if enabled and configured)
	if cfg.MemoryCache != nil && cfg.MemoryCache.Enabled && cfg.MemoryCache.MaxSizeMB > 0 {
		memorySizeBytes := int64(cfg.MemoryCache.MaxSizeMB) * 1024 * 1024
		maxItems := int64(cfg.MemoryCache.MaxItems)

		// Use reasonable default for MaxItems if not specified
		if maxItems == 0 {
			maxItems = int64(cfg.MemoryCache.MaxSizeMB) // Estimate: ~1MB per item
		}

		memCache, err := cache.NewMemoryCache(cache.MemoryCacheConfig{
			MaxSize:  memorySizeBytes,
			MaxItems: maxItems,
			TTL:      cfg.MemoryCache.TTL,
		})
		if err != nil {
			log.Printf("[CachedStorage] Failed to init memory cache: %v", err)
		} else {
			cs.memoryCache = memCache
			log.Printf("[CachedStorage] Enabled in-memory cache: MaxSize=%dMB, MaxItems=%d, TTL=%v", cfg.MemoryCache.MaxSizeMB, maxItems, cfg.MemoryCache.TTL)
		}
	}

	// Initialize disk cache second (if enabled)
	if cfg.DiskCache != nil && cfg.DiskCache.Enabled {
		// Convert MB to bytes (0 = unlimited)
		diskCacheMaxBytes := int64(0)
		if cfg.DiskCache.MaxSizeMB > 0 {
			diskCacheMaxBytes = int64(cfg.DiskCache.MaxSizeMB) * 1024 * 1024
		}

		diskCache, err := cache.NewDiskCache(
			cfg.DiskCache.BasePath,
			cfg.DiskCache.TTL,
			cfg.DiskCache.ClearOnStartup,
			diskCacheMaxBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create disk cache: %w", err)
		}

		cs.diskCache = diskCache
	}

	return cs, nil
}
