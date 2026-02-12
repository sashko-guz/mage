package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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

	// Initialize all configured storages (with cache layers if configured)
	storages, err := storage.InitializeStorages(storageConfig)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize storages: %v", err)
	}

	// Build index of storage configs by name for easy lookup
	storageCfgByName := make(map[string]storage.StorageItem)
	for _, item := range storageConfig.Storages {
		storageCfgByName[item.Name] = item
	}

	// Collect signature keys per storage
	signatureKeys := make(map[string]string)
	for name := range storages {
		if item, ok := storageCfgByName[name]; ok {
			signatureKeys[name] = item.SignatureSecretKey
		}
	}

	// Initialize image processor
	imageProcessor := processor.NewImageProcessor()

	// Initialize handler
	thumbnailHandler, err := handler.NewThumbnailHandler(storages, imageProcessor, signatureKeys)
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
