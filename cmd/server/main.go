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
	log.Printf("[Server] Storage config loaded from: %s", cfg.StorageConfigPath)

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

	// Load storage configuration from JSON
	storageConfig, err := storage.LoadConfig(cfg.StorageConfigPath)
	if err != nil {
		log.Fatalf("[Server] Failed to load storage config: %v", err)
	}

	// Initialize storage (with cache layers if configured)
	stor, signatureKey, err := storage.NewStorage(storageConfig)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize storage: %v", err)
	}

	// Initialize image processor
	imageProcessor := processor.NewImageProcessor()

	// Initialize handler
	thumbnailHandler, err := handler.NewThumbnailHandler(stor, imageProcessor, signatureKey)
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
	log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/[{signature}/]{size}/[filters:{filters}/]{path}", addr)
	if signatureKey != "" {
		log.Printf("[Server] Example: http://localhost%s/thumbs/a1b2c3d4e5f6g7h8/400x300/filters:format(webp);quality(88)/image.jpg", addr)
	} else {
		log.Printf("[Server] Example: http://localhost%s/thumbs/400x300/filters:format(webp);quality(88)/image.jpg", addr)
	}

	if err := http.ListenAndServe(addr, rootHandler); err != nil {
		log.Fatalf("[Server] Server failed to start: %v", err)
	}
}

func isThumbnailPath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/")
	return strings.HasPrefix(trimmed, "thumbs/") || trimmed == "thumbs"
}
