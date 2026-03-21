package main

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
	"github.com/joho/godotenv"
	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/parser"
)

func main() {
	_ = godotenv.Load()

	setupLogging()

	if err := run(); err != nil {
		logger.Fatalf("[Server] Fatal error: %v", err)
	}
}

func run() error {
	cfg := config.Load()

	parser.Init(cfg.MaxResizeWidth, cfg.MaxResizeHeight, cfg.MaxResizeResolution)
	parser.SetSignatureLength(cfg.SignatureLength)
	parser.SetSignatureValidationEnabled(cfg.SignatureSecret != "")

	log.Printf("[Server] Starting…")
	log.Printf("[Server] Log level: %s", logger.CurrentLevelString())
	log.Printf("[Server] Storage config: %s", cfg.StorageConfigPath)
	log.Printf("[Server] Resize limits: max width=%d px, max height=%d px, max resolution=%d px",
		cfg.MaxResizeWidth, cfg.MaxResizeHeight, cfg.MaxResizeResolution)
	log.Printf("[Server] Max input image size: %d MB", cfg.MaxInputImageSize/(1024*1024))

	vips.Startup(configureVips())
	defer vips.Shutdown()

	stor, err := initializeStorage(cfg.StorageConfigPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	srv := setupServer(cfg, stor)
	logServerInfo(cfg)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed: %w", err)
	case sig := <-quit:
		log.Printf("[Server] Received %s, shutting down gracefully…", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Printf("[Server] Shutdown complete")
	return <-errCh
}
