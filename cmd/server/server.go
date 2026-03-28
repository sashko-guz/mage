package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/handler"
	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/processor"
	"github.com/sashko-guz/mage/internal/signature"
	storageDrivers "github.com/sashko-guz/mage/internal/storage/drivers"
)

func setupServer(cfg *config.Config, stor storageDrivers.Storage) *http.Server {
	imageProcessor := processor.NewImageProcessor()
	thumbnailHandler, err := handler.NewThumbnailHandler(stor, imageProcessor, handler.ThumbnailHandlerConfig{
		SignatureCfg: signature.Config{
			SecretKey:     cfg.Signature.Secret,
			Algorithm:     cfg.Signature.Algorithm,
			ExtractStart:  cfg.Signature.Start,
			ExtractLength: cfg.Signature.Length,
		},
		MaxInputSize:               cfg.Resize.MaxInputSize,
		CacheControlResponseHeader: cfg.CacheControlResponseHeader,
	})
	if err != nil {
		logger.Fatalf("[Server] Failed to initialize thumbnail handler: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           buildRoutes(thumbnailHandler, cfg),
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}

	log.Printf("[Server] HTTP server configured:")
	log.Printf("  - ReadTimeout: %v", srv.ReadTimeout)
	log.Printf("  - ReadHeaderTimeout: %v", srv.ReadHeaderTimeout)
	log.Printf("  - WriteTimeout: %v", srv.WriteTimeout)
	log.Printf("  - IdleTimeout: %v", srv.IdleTimeout)
	log.Printf("  - MaxHeaderBytes: %d bytes", srv.MaxHeaderBytes)

	return srv
}

func buildRoutes(thumbnailHandler *handler.ThumbnailHandler, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyCORSHeaders(w, cfg)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

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

func applyCORSHeaders(w http.ResponseWriter, cfg *config.Config) {
	allowOrigin := cfg.CORS.AllowOrigin
	if allowOrigin == "" {
		allowOrigin = "*"
	}

	if allowOrigin == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	w.Header().Set("Access-Control-Allow-Methods", cfg.CORS.AllowMethods)
	w.Header().Set("Access-Control-Allow-Headers", cfg.CORS.AllowHeaders)
	w.Header().Set("Access-Control-Expose-Headers", cfg.CORS.ExposeHeaders)
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.CORS.MaxAge))
}

func isThumbnailPath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/")
	return strings.HasPrefix(trimmed, "thumbs/") || trimmed == "thumbs"
}
