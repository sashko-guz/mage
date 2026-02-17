package storage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sashko-guz/mage/internal/cache"
	"github.com/sashko-guz/mage/internal/logger"
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
		logger.Infof("[Storage] Initialized (%s)", strings.Join(logParts, ", "))
		return baseStorage, cfg.SignatureSecret, nil
	}

	// Step 3: Wrap with cache layers
	cachedStorage, err := wrapWithCache(baseStorage, cfg)
	if err != nil {
		return nil, "", err
	}

	logger.Infof("[Storage] Initialized (%s)", strings.Join(logParts, ", "))
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

// wrapWithCache wraps the base storage with separate cache layers for sources and thumbnails
func wrapWithCache(baseStorage Storage, cfg *StorageConfig) (Storage, error) {
	if cfg.Cache == nil {
		return baseStorage, nil
	}

	sourcesEnabled := (cfg.Cache.Sources != nil &&
		((cfg.Cache.Sources.Disk != nil && cfg.Cache.Sources.Disk.Enabled != nil && *cfg.Cache.Sources.Disk.Enabled) ||
			(cfg.Cache.Sources.Memory != nil && cfg.Cache.Sources.Memory.Enabled != nil && *cfg.Cache.Sources.Memory.Enabled)))

	thumbsEnabled := (cfg.Cache.Thumbs != nil &&
		((cfg.Cache.Thumbs.Disk != nil && cfg.Cache.Thumbs.Disk.Enabled != nil && *cfg.Cache.Thumbs.Disk.Enabled) ||
			(cfg.Cache.Thumbs.Memory != nil && cfg.Cache.Thumbs.Memory.Enabled != nil && *cfg.Cache.Thumbs.Memory.Enabled)))

	if !sourcesEnabled && !thumbsEnabled {
		logger.Infof("[Cache] No cache enabled")
		return baseStorage, nil
	}

	// Build cache info for logging
	var cacheInfo []string
	if sourcesEnabled {
		cacheInfo = append(cacheInfo, "Sources")
	}
	if thumbsEnabled {
		cacheInfo = append(cacheInfo, "Thumbs")
	}
	logger.Infof("[Cache] Enabled for: %s", strings.Join(cacheInfo, ", "))

	cacheConfig := CachedStorageConfig{}
	defaultTTL := 5 * time.Minute

	// Configure source caches
	if sourcesEnabled {
		sourceCfg := cfg.Cache.Sources

		// Memory cache for sources
		if sourceCfg.Memory != nil && sourceCfg.Memory.Enabled != nil && *sourceCfg.Memory.Enabled {
			maxItems := sourceCfg.Memory.MaxItems
			if maxItems == 0 {
				maxItems = 1000
			}
			memTTL := defaultTTL
			if sourceCfg.Memory.TTLSeconds > 0 {
				memTTL = time.Duration(sourceCfg.Memory.TTLSeconds) * time.Second
			}
			cacheConfig.SourceMemoryCache = &MemoryCacheConfig{
				Enabled:   true,
				MaxSizeMB: sourceCfg.Memory.MaxSizeMB,
				MaxItems:  maxItems,
				TTL:       memTTL,
			}
		}

		// Disk cache for sources
		if sourceCfg.Disk != nil && sourceCfg.Disk.Enabled != nil && *sourceCfg.Disk.Enabled {
			if sourceCfg.Disk.Dir == "" {
				return nil, fmt.Errorf("sources cache dir is required when disk cache is enabled")
			}
			if err := os.MkdirAll(sourceCfg.Disk.Dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create sources cache directory: %w", err)
			}
			diskTTL := defaultTTL
			if sourceCfg.Disk.TTLSeconds > 0 {
				diskTTL = time.Duration(sourceCfg.Disk.TTLSeconds) * time.Second
			}
			clearOnStartup := false
			if sourceCfg.Disk.ClearOnStartup != nil {
				clearOnStartup = *sourceCfg.Disk.ClearOnStartup
			}
			maxItems := sourceCfg.Disk.MaxItems
			if maxItems <= 0 {
				maxItems = 100_000
			}
			cacheConfig.SourceDiskCache = &DiskCacheConfig{
				Enabled:        true,
				BasePath:       sourceCfg.Disk.Dir,
				TTL:            diskTTL,
				ClearOnStartup: clearOnStartup,
				MaxSizeMB:      sourceCfg.Disk.MaxSizeMB,
				MaxItems:       maxItems,
			}

			asyncEnabled := true // Default to enabled
			numWorkers := 4      // Default workers
			queueSize := 1000    // Default queue size
			if sourceCfg.Disk.AsyncWrite != nil {
				if sourceCfg.Disk.AsyncWrite.Enabled != nil {
					asyncEnabled = *sourceCfg.Disk.AsyncWrite.Enabled
				}
				if sourceCfg.Disk.AsyncWrite.NumWorkers > 0 {
					numWorkers = sourceCfg.Disk.AsyncWrite.NumWorkers
				}
				if sourceCfg.Disk.AsyncWrite.QueueSize > 0 {
					queueSize = sourceCfg.Disk.AsyncWrite.QueueSize
				}
			}
			cacheConfig.SourceAsyncWrite = &AsyncWriteConfig{
				Enabled:    asyncEnabled,
				NumWorkers: numWorkers,
				QueueSize:  queueSize,
			}
		}
	}

	// Configure thumbnail caches
	if thumbsEnabled {
		thumbsCfg := cfg.Cache.Thumbs

		// Memory cache for thumbs
		if thumbsCfg.Memory != nil && thumbsCfg.Memory.Enabled != nil && *thumbsCfg.Memory.Enabled {
			maxItems := thumbsCfg.Memory.MaxItems
			if maxItems == 0 {
				maxItems = 1000
			}
			memTTL := defaultTTL
			if thumbsCfg.Memory.TTLSeconds > 0 {
				memTTL = time.Duration(thumbsCfg.Memory.TTLSeconds) * time.Second
			}
			cacheConfig.ThumbMemoryCache = &MemoryCacheConfig{
				Enabled:   true,
				MaxSizeMB: thumbsCfg.Memory.MaxSizeMB,
				MaxItems:  maxItems,
				TTL:       memTTL,
			}
		}

		// Disk cache for thumbs
		if thumbsCfg.Disk != nil && thumbsCfg.Disk.Enabled != nil && *thumbsCfg.Disk.Enabled {
			if thumbsCfg.Disk.Dir == "" {
				return nil, fmt.Errorf("thumbs cache dir is required when disk cache is enabled")
			}
			if err := os.MkdirAll(thumbsCfg.Disk.Dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create thumbs cache directory: %w", err)
			}
			diskTTL := defaultTTL
			if thumbsCfg.Disk.TTLSeconds > 0 {
				diskTTL = time.Duration(thumbsCfg.Disk.TTLSeconds) * time.Second
			}
			clearOnStartup := false
			if thumbsCfg.Disk.ClearOnStartup != nil {
				clearOnStartup = *thumbsCfg.Disk.ClearOnStartup
			}
			maxItems := thumbsCfg.Disk.MaxItems
			if maxItems <= 0 {
				maxItems = 100_000
			}
			cacheConfig.ThumbDiskCache = &DiskCacheConfig{
				Enabled:        true,
				BasePath:       thumbsCfg.Disk.Dir,
				TTL:            diskTTL,
				ClearOnStartup: clearOnStartup,
				MaxSizeMB:      thumbsCfg.Disk.MaxSizeMB,
				MaxItems:       maxItems,
			}

			asyncEnabled := true // Default to enabled
			numWorkers := 4      // Default workers
			queueSize := 1000    // Default queue size
			if thumbsCfg.Disk.AsyncWrite != nil {
				if thumbsCfg.Disk.AsyncWrite.Enabled != nil {
					asyncEnabled = *thumbsCfg.Disk.AsyncWrite.Enabled
				}
				if thumbsCfg.Disk.AsyncWrite.NumWorkers > 0 {
					numWorkers = thumbsCfg.Disk.AsyncWrite.NumWorkers
				}
				if thumbsCfg.Disk.AsyncWrite.QueueSize > 0 {
					queueSize = thumbsCfg.Disk.AsyncWrite.QueueSize
				}
			}
			cacheConfig.ThumbAsyncWrite = &AsyncWriteConfig{
				Enabled:    asyncEnabled,
				NumWorkers: numWorkers,
				QueueSize:  queueSize,
			}
		}
	}

	return newCachedStorage(baseStorage, cacheConfig)
}

// newCachedStorage creates a wrapped storage with separate caching for sources and thumbnails
func newCachedStorage(underlying Storage, cfg CachedStorageConfig) (*CachedStorage, error) {
	// Validate that at least one cache is enabled
	sourcesCacheEnabled := (cfg.SourceMemoryCache != nil && cfg.SourceMemoryCache.Enabled) ||
		(cfg.SourceDiskCache != nil && cfg.SourceDiskCache.Enabled)

	thumbsCacheEnabled := (cfg.ThumbMemoryCache != nil && cfg.ThumbMemoryCache.Enabled) ||
		(cfg.ThumbDiskCache != nil && cfg.ThumbDiskCache.Enabled)

	if !sourcesCacheEnabled && !thumbsCacheEnabled {
		return nil, fmt.Errorf("at least one cache (sources or thumbs) must be enabled")
	}

	cs := &CachedStorage{
		underlying: underlying,
	}

	// Determine TTLs for each cache type
	sourceTTL := 5 * time.Minute
	if cfg.SourceMemoryCache != nil && cfg.SourceMemoryCache.TTL > 0 {
		sourceTTL = cfg.SourceMemoryCache.TTL
	} else if cfg.SourceDiskCache != nil && cfg.SourceDiskCache.TTL > 0 {
		sourceTTL = cfg.SourceDiskCache.TTL
	}
	cs.sourceTTL = sourceTTL

	thumbTTL := 5 * time.Minute
	if cfg.ThumbMemoryCache != nil && cfg.ThumbMemoryCache.TTL > 0 {
		thumbTTL = cfg.ThumbMemoryCache.TTL
	} else if cfg.ThumbDiskCache != nil && cfg.ThumbDiskCache.TTL > 0 {
		thumbTTL = cfg.ThumbDiskCache.TTL
	}
	cs.thumbTTL = thumbTTL

	// Initialize source caches
	if sourcesCacheEnabled {
		if cfg.SourceMemoryCache != nil && cfg.SourceMemoryCache.Enabled {
			memorySizeBytes := int64(cfg.SourceMemoryCache.MaxSizeMB) * 1024 * 1024
			maxItems := int64(cfg.SourceMemoryCache.MaxItems)
			if maxItems == 0 {
				maxItems = int64(cfg.SourceMemoryCache.MaxSizeMB)
			}
			memCache, err := cache.NewMemoryCache(cache.MemoryCacheConfig{
				MaxSize:  memorySizeBytes,
				MaxItems: maxItems,
				TTL:      cfg.SourceMemoryCache.TTL,
			})
			if err != nil {
				logger.Warnf("[CachedStorage] Failed to init source memory cache: %v", err)
			} else {
				cs.sourceMemoryCache = memCache
				logger.Infof("[CachedStorage] Source memory cache: MaxSize=%dMB, MaxItems=%d, TTL=%v",
					cfg.SourceMemoryCache.MaxSizeMB, maxItems, cfg.SourceMemoryCache.TTL)
			}
		}

		if cfg.SourceDiskCache != nil && cfg.SourceDiskCache.Enabled {
			diskCacheMaxBytes := int64(0)
			if cfg.SourceDiskCache.MaxSizeMB > 0 {
				diskCacheMaxBytes = int64(cfg.SourceDiskCache.MaxSizeMB) * 1024 * 1024
			}
			diskCache, err := cache.NewDiskCache(
				cfg.SourceDiskCache.BasePath,
				cfg.SourceDiskCache.TTL,
				cfg.SourceDiskCache.ClearOnStartup,
				diskCacheMaxBytes,
				cfg.SourceDiskCache.MaxItems,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create source disk cache: %w", err)
			}
			cs.sourceDiskCache = diskCache
			logger.Infof("[CachedStorage] Source disk cache: Dir=%s, MaxSize=%dMB, TTL=%v",
				cfg.SourceDiskCache.BasePath, cfg.SourceDiskCache.MaxSizeMB, cfg.SourceDiskCache.TTL)
		}
	}

	// Initialize thumbnail caches
	if thumbsCacheEnabled {
		if cfg.ThumbMemoryCache != nil && cfg.ThumbMemoryCache.Enabled {
			memorySizeBytes := int64(cfg.ThumbMemoryCache.MaxSizeMB) * 1024 * 1024
			maxItems := int64(cfg.ThumbMemoryCache.MaxItems)
			if maxItems == 0 {
				maxItems = int64(cfg.ThumbMemoryCache.MaxSizeMB)
			}
			memCache, err := cache.NewMemoryCache(cache.MemoryCacheConfig{
				MaxSize:  memorySizeBytes,
				MaxItems: maxItems,
				TTL:      cfg.ThumbMemoryCache.TTL,
			})
			if err != nil {
				logger.Warnf("[CachedStorage] Failed to init thumb memory cache: %v", err)
			} else {
				cs.thumbMemoryCache = memCache
				logger.Infof("[CachedStorage] Thumb memory cache: MaxSize=%dMB, MaxItems=%d, TTL=%v",
					cfg.ThumbMemoryCache.MaxSizeMB, maxItems, cfg.ThumbMemoryCache.TTL)
			}
		}

		if cfg.ThumbDiskCache != nil && cfg.ThumbDiskCache.Enabled {
			diskCacheMaxBytes := int64(0)
			if cfg.ThumbDiskCache.MaxSizeMB > 0 {
				diskCacheMaxBytes = int64(cfg.ThumbDiskCache.MaxSizeMB) * 1024 * 1024
			}
			diskCache, err := cache.NewDiskCache(
				cfg.ThumbDiskCache.BasePath,
				cfg.ThumbDiskCache.TTL,
				cfg.ThumbDiskCache.ClearOnStartup,
				diskCacheMaxBytes,
				cfg.ThumbDiskCache.MaxItems,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create thumb disk cache: %w", err)
			}
			cs.thumbDiskCache = diskCache
			logger.Infof("[CachedStorage] Thumb disk cache: Dir=%s, MaxSize=%dMB, TTL=%v",
				cfg.ThumbDiskCache.BasePath, cfg.ThumbDiskCache.MaxSizeMB, cfg.ThumbDiskCache.TTL)
		}
	}

	// Initialize async write workers for sources and thumbs
	if cfg.SourceAsyncWrite != nil && cfg.SourceAsyncWrite.Enabled && cs.sourceDiskCache != nil {
		cs.initSourceWorkers(cfg.SourceAsyncWrite.NumWorkers, cfg.SourceAsyncWrite.QueueSize)
		logger.Infof("[CachedStorage] Source async write: %d workers, queue size %d",
			cfg.SourceAsyncWrite.NumWorkers, cfg.SourceAsyncWrite.QueueSize)
	}

	if cfg.ThumbAsyncWrite != nil && cfg.ThumbAsyncWrite.Enabled && cs.thumbDiskCache != nil {
		cs.initThumbWorkers(cfg.ThumbAsyncWrite.NumWorkers, cfg.ThumbAsyncWrite.QueueSize)
		logger.Infof("[CachedStorage] Thumb async write: %d workers, queue size %d",
			cfg.ThumbAsyncWrite.NumWorkers, cfg.ThumbAsyncWrite.QueueSize)
	}

	return cs, nil
}
