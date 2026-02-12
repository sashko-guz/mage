package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cshum/vipsgen/vips"
	"github.com/joho/godotenv"
	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/handler"
	"github.com/sashko-guz/mage/internal/processor"
	"github.com/sashko-guz/mage/internal/storage"
)

func main() {
	// Configure logging to stderr with timestamps
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load .env file if it exists (optional)
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	log.Printf("[Server] Starting image processing server…")
	log.Printf("[Server] Storages config loaded from: %s", cfg.StoragesConfigPath)

	// Configure libvips concurrency if provided
	var vipsCfg *vips.Config
	if vipsConcurrency := os.Getenv("VIPS_CONCURRENCY"); vipsConcurrency != "" {
		conc, err := strconv.Atoi(vipsConcurrency)
		if err != nil || conc <= 0 {
			log.Printf("[Server] Ignoring VIPS_CONCURRENCY=%q (must be positive integer)", vipsConcurrency)
		} else {
			vipsCfg = &vips.Config{ConcurrencyLevel: conc}
			log.Printf("[Server] libvips concurrency set to %d via VIPS_CONCURRENCY", conc)
		}
	}

	vips.Startup(vipsCfg)
	defer vips.Shutdown()

	// Load storages configuration from JSON
	storageConfig, err := storage.LoadStorageConfig(cfg.StoragesConfigPath)
	if err != nil {
		log.Fatalf("[Server] Failed to load storages config: %v", err)
	}

	// Initialize all configured storages
	storages, err := storage.InitializeStorages(storageConfig)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize storages: %v", err)
	}

	// Wrap storages with file-based caching layer
	storageCfgByName := make(map[string]storage.StorageItem)
	for _, item := range storageConfig.Storages {
		storageCfgByName[item.Name] = item
	}

	cachedStorages := make(map[string]storage.Storage)
	signatureKeys := make(map[string]string) // Collect signature keys per storage
	for name, store := range storages {
		cacheEnabled := false
		cacheTTL := 5 * time.Minute
		cacheDir := ""
		clearOnStartup := false
		memoryCacheSizeMB := 0
		diskCacheMaxSizeMB := 0 // 0 = unlimited

		if item, ok := storageCfgByName[name]; ok {
			// Collect signature key for this storage
			signatureKeys[name] = item.SignatureSecretKey

			if item.Cache != nil {
				// Check if disk caching is enabled
				diskEnabled := false
				if item.Cache.Disk != nil && item.Cache.Disk.Enabled != nil {
					diskEnabled = *item.Cache.Disk.Enabled
				}

				// Check if memory caching is enabled
				memoryEnabled := false
				if item.Cache.Memory != nil && item.Cache.Memory.Enabled != nil {
					memoryEnabled = *item.Cache.Memory.Enabled
				}

				cacheEnabled = diskEnabled || memoryEnabled

				// Read disk cache config
				if item.Cache.Disk != nil {
					if item.Cache.Disk.TTLSeconds > 0 {
						cacheTTL = time.Duration(item.Cache.Disk.TTLSeconds) * time.Second
					}
					cacheDir = item.Cache.Disk.Dir
					if item.Cache.Disk.ClearOnStartup != nil {
						clearOnStartup = *item.Cache.Disk.ClearOnStartup
					}
					diskCacheMaxSizeMB = item.Cache.Disk.MaxSizeMB
				}

				// Read memory cache config
				if item.Cache.Memory != nil && item.Cache.Memory.MaxSizeMB > 0 {
					memoryCacheSizeMB = item.Cache.Memory.MaxSizeMB
				}
			}
		}

		if !cacheEnabled {
			cachedStorages[name] = store
			continue
		}

		if cacheDir == "" {
			log.Fatalf("[Server] Cache dir is required for storage %s when cache is enabled", name)
		}
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			log.Fatalf("[Server] Failed to create cache directory for %s: %v", name, err)
		}

		// Build cache config
		cacheConfig := storage.CachedStorageConfig{
			StorageName: name,
			DiskCache: &storage.DiskCacheConfig{
				Enabled:        true,
				BasePath:       cacheDir,
				TTL:            cacheTTL,
				ClearOnStartup: clearOnStartup,
				MaxSizeMB:      diskCacheMaxSizeMB,
			},
		}

		// Add memory cache config if enabled
		if item, ok := storageCfgByName[name]; ok && item.Cache != nil && item.Cache.Memory != nil {
			if item.Cache.Memory.Enabled != nil && *item.Cache.Memory.Enabled {
				maxItems := item.Cache.Memory.MaxItems
				if maxItems == 0 {
					maxItems = 1000 // default
				}
				cacheConfig.MemoryCache = &storage.MemoryCacheConfig{
					Enabled:   true,
					MaxSizeMB: memoryCacheSizeMB,
					MaxItems:  maxItems,
				}
			}
		}

		cachedStore, err := storage.NewCachedStorage(store, cacheConfig)
		if err != nil {
			log.Fatalf("[Server] Failed to create cached storage for %s: %v", name, err)
		}
		cachedStorages[name] = cachedStore
	}

	// Log initialized storages
	log.Printf("[Server] Initialized %d storage(s):", len(cachedStorages))
	for name := range cachedStorages {
		logMsg := fmt.Sprintf("  - %s", name)
		if signatureKeys[name] != "" {
			logMsg += " (signature validation enabled)"
		} else {
			logMsg += " (signature validation disabled)"
		}

		// Add cache info
		if item, ok := storageCfgByName[name]; ok && item.Cache != nil {
			memoryEnabled := item.Cache.Memory != nil && item.Cache.Memory.Enabled != nil && *item.Cache.Memory.Enabled
			diskEnabled := item.Cache.Disk != nil && item.Cache.Disk.Enabled != nil && *item.Cache.Disk.Enabled

			if memoryEnabled || diskEnabled {
				var cacheInfo []string
				if memoryEnabled && item.Cache.Memory.MaxSizeMB > 0 {
					cacheInfo = append(cacheInfo, fmt.Sprintf("memory: %dMB", item.Cache.Memory.MaxSizeMB))
				}
				if diskEnabled {
					cacheInfo = append(cacheInfo, "disk")
				}
				if len(cacheInfo) > 0 {
					logMsg += fmt.Sprintf(" [cache: %s]", strings.Join(cacheInfo, ", "))
				}
			}
		}

		log.Printf("%s", logMsg)
	}

	// Initialize image processor
	imageProcessor := processor.NewImageProcessor()

	// Initialize handler
	thumbnailHandler, err := handler.NewThumbnailHandler(cachedStorages, imageProcessor, signatureKeys)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize thumbnail handler: %v", err)
	}

	// Setup routes
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isThumbnailPath(r.URL.Path) {
			thumbnailHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		http.NotFound(w, r)
	})

	// Start server
	addr := ":" + cfg.Port
	log.Printf("[Server] Server listening on %s", addr)
	log.Printf("[Server] Thumbnail endpoint: http://localhost%s/{storage}/thumbs/[{signature}/]{size}/[filters:{filters}/]{path}", addr)
	log.Printf("[Server] Example with s3 storage and signature: http://localhost%s/s3/thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(webp);quality(88)/my-awesome-image.jpeg", addr)
	log.Printf("[Server] Example with local storage without signature: http://localhost%s/local/thumbs/400x300/image.png", addr)

	if err := http.ListenAndServe(addr, rootHandler); err != nil {
		log.Fatalf("[Server] Server failed to start: %v", err)
	}
}

func isThumbnailPath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 3)
	return len(parts) >= 2 && parts[1] == "thumbs"
}
