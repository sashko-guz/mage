package http

import (
	"log"
	"net/http"

	"github.com/sashko-guz/mage/internal/config"
)

// NewServer creates a configured HTTP server
func NewServer(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	log.Printf("[HTTP] Server configured:")
	log.Printf("  - ReadTimeout: %v", srv.ReadTimeout)
	log.Printf("  - ReadHeaderTimeout: %v", srv.ReadHeaderTimeout)
	log.Printf("  - WriteTimeout: %v", srv.WriteTimeout)
	log.Printf("  - IdleTimeout: %v", srv.IdleTimeout)
	log.Printf("  - MaxHeaderBytes: %d bytes", srv.MaxHeaderBytes)

	return srv
}
