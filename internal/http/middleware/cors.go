package middleware

import (
	"net/http"
	"strconv"

	"github.com/sashko-guz/mage/internal/config"
)

// CORS applies CORS headers to the response
func CORS(w http.ResponseWriter, cfg config.CORSConfig) {
	allowOrigin := cfg.AllowOrigin
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

	w.Header().Set("Access-Control-Allow-Methods", cfg.AllowMethods)
	w.Header().Set("Access-Control-Allow-Headers", cfg.AllowHeaders)
	w.Header().Set("Access-Control-Expose-Headers", cfg.ExposeHeaders)
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
}
