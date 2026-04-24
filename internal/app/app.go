package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cshum/vipsgen/vips"
	"github.com/sashko-guz/mage/internal/auth/signature"
	"github.com/sashko-guz/mage/internal/config"
	magehttp "github.com/sashko-guz/mage/internal/http"
	"github.com/sashko-guz/mage/internal/observability/health"
	"github.com/sashko-guz/mage/internal/observability/metrics"
	"github.com/sashko-guz/mage/internal/pkg/logger"
	"github.com/sashko-guz/mage/internal/storage"
	storageDrivers "github.com/sashko-guz/mage/internal/storage/drivers"
	"github.com/sashko-guz/mage/internal/thumbnail/handler"
	"github.com/sashko-guz/mage/internal/thumbnail/parser"
	"github.com/sashko-guz/mage/internal/thumbnail/processor"
)

// App represents the application and its dependencies
type App struct {
	cfg     *config.Config
	server  *http.Server
	storage storageDrivers.Storage
	metrics *metrics.Metrics
}

// New creates a new application instance
func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

// Run starts the application and blocks until shutdown
func (a *App) Run() error {
	// Initialize metrics
	if a.cfg.Metrics.Enabled {
		a.metrics = metrics.New()
		log.Printf("[App] Metrics enabled at %s", a.cfg.Metrics.Path)
	}

	// Initialize parser
	parser.Init(a.cfg.Resize.MaxWidth, a.cfg.Resize.MaxHeight, a.cfg.Resize.MaxResolution)
	parser.SetSignatureLength(a.cfg.Signature.Length)
	parser.SetSignatureValidationEnabled(a.cfg.Signature.Secret != "")

	a.logStartup()

	// Initialize vips
	vips.Startup(configureVips())
	defer vips.Shutdown()

	// Initialize storage
	if err := a.initStorage(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize HTTP server
	if err := a.initServer(); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Start server
	return a.startAndWait()
}

func (a *App) logStartup() {
	log.Printf("[App] Starting...")
	log.Printf("[App] Log level: %s", logger.CurrentLevelString())
	log.Printf("[App] Resize limits: max width=%d px, max height=%d px, max resolution=%d px",
		a.cfg.Resize.MaxWidth, a.cfg.Resize.MaxHeight, a.cfg.Resize.MaxResolution)
	log.Printf("[App] Max input image size: %d MB", a.cfg.Resize.MaxInputSize/(1024*1024))
}

func (a *App) initStorage() error {
	storageConfig := storage.LoadConfig()
	stor, err := storage.NewStorage(storageConfig)
	if err != nil {
		return err
	}

	// Wire metrics to cached storage if available
	if a.metrics != nil {
		if cached, ok := stor.(*storage.CachedStorage); ok {
			cached.SetMetrics(a.metrics, string(storageConfig.Driver))
		}
	}

	a.storage = stor
	return nil
}

func (a *App) initServer() error {
	imageProcessor := processor.NewImageProcessor()

	config := handler.ThumbnailHandlerConfig{
		SignatureCfg: signature.Config{
			SecretKey:     a.cfg.Signature.Secret,
			Algorithm:     a.cfg.Signature.Algorithm,
			ExtractStart:  a.cfg.Signature.Start,
			ExtractLength: a.cfg.Signature.Length,
		},
		MaxInputSize:               a.cfg.Resize.MaxInputSize,
		CacheControlResponseHeader: a.cfg.CacheControlResponseHeader,
	}

	// Only assign metrics if it's non-nil to avoid interface containing nil pointer
	if a.metrics != nil {
		config.Metrics = a.metrics
	}

	thumbnailHandler, err := handler.NewThumbnailHandler(a.storage, imageProcessor, config)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail handler: %w", err)
	}

	// Create health handler with storage checker
	var healthCheckers []health.Checker
	if pingable, ok := a.storage.(health.Pingable); ok {
		healthCheckers = append(healthCheckers, health.NewStorageChecker(pingable))
	}
	healthHandler := health.NewHandler(a.cfg.Health.ReadinessTimeout, healthCheckers...)

	// Build router and server
	router := magehttp.NewRouter(a.cfg.CORS, a.metrics)
	router.RegisterRoutes(thumbnailHandler, healthHandler, a.cfg.Metrics.Enabled, a.cfg.Metrics.Path)

	a.server = magehttp.NewServer(a.cfg.HTTP, router)

	return nil
}

func (a *App) startAndWait() error {
	a.logServerInfo()

	errCh := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed: %w", err)
	case sig := <-quit:
		log.Printf("[App] Received %s, shutting down gracefully...", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Printf("[App] Shutdown complete")
	return <-errCh
}

func (a *App) logServerInfo() {
	addr := ":" + a.cfg.HTTP.Port
	log.Printf("[App] Server listening on %s", addr)

	if a.cfg.Signature.Secret != "" {
		log.Printf("[App] Signature validation: ENABLED")
		log.Printf("[App] Signature config: algo=%s, extract_start=%d, length=%d",
			a.cfg.Signature.Algorithm, a.cfg.Signature.Start, a.cfg.Signature.Length)
	} else {
		log.Printf("[App] Signature validation: DISABLED")
	}
}
