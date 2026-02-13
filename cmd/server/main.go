package main

import (
	"fmt"
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
	setupLogging()

	if err := run(); err != nil {
		log.Fatalf("[Server] Fatal error: %v", err)
	}
}

func run() error {
	cfg := loadConfig()

	log.Printf("[Server] Starting…")
	log.Printf("[Server] Storage config loaded from: %s", cfg.StorageConfigPath)

	vipsCfg := configureVips()
	vips.Startup(vipsCfg)
	defer vips.Shutdown()

	stor, signatureKey, err := initializeStorage(cfg.StorageConfigPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	srv := setupServer(cfg, stor, signatureKey)

	logServerInfo(cfg.Port, signatureKey)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func setupLogging() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func loadConfig() *config.Config {
	_ = godotenv.Load()
	return config.Load()
}

func configureVips() *vips.Config {
	vipsConcurrency := os.Getenv("VIPS_CONCURRENCY")
	if vipsConcurrency == "" {
		return nil
	}

	conc, err := strconv.Atoi(vipsConcurrency)
	if err != nil || conc <= 0 {
		log.Printf("[Server] Ignoring VIPS_CONCURRENCY=%q (must be positive integer)", vipsConcurrency)
		return nil
	}

	log.Printf("[Server] libvips concurrency set to %d via VIPS_CONCURRENCY", conc)
	return &vips.Config{ConcurrencyLevel: conc}
}

func initializeStorage(configPath string) (storage.Storage, string, error) {
	storageConfig, err := storage.LoadConfig(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load storage config: %w", err)
	}

	stor, signatureKey, err := storage.NewStorage(storageConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create storage: %w", err)
	}

	return stor, signatureKey, nil
}

func setupServer(cfg *config.Config, stor storage.Storage, signatureKey string) *http.Server {
	imageProcessor := processor.NewImageProcessor()

	thumbnailHandler, err := handler.NewThumbnailHandler(stor, imageProcessor, signatureKey)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize thumbnail handler: %v", err)
	}

	mux := buildRoutes(thumbnailHandler)

	return &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}
}

func buildRoutes(thumbnailHandler *handler.ThumbnailHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
			case isThumbnailPath(r.URL.Path):
				thumbnailHandler.ServeHTTP(w, r)
			case r.URL.Path == "/health":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			default:
				http.NotFound(w, r)
		}
	})
}

func logServerInfo(port, signatureKey string) {
	addr := ":" + port
	log.Printf("[Server] Server listening on %s", addr)
	log.Printf("[Server] Thumbnail endpoint: http://localhost%s/thumbs/[{signature}/]{size}/[filters:{filters}/]{path}", addr)

	if signatureKey != "" {
		log.Printf("[Server] Example: http://localhost%s/thumbs/a1b2c3d4e5f6g7h8/400x300/filters:format(webp);quality(88)/image.jpg", addr)
	} else {
		log.Printf("[Server] Example: http://localhost%s/thumbs/400x300/filters:format(webp);quality(88)/image.jpg", addr)
	}
}

func isThumbnailPath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/")
	return strings.HasPrefix(trimmed, "thumbs/") || trimmed == "thumbs"
}
