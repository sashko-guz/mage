package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RoutePrefixGetter extracts the matched route prefix from request context
type RoutePrefixGetter func(r *http.Request) string

// statusRecorder wraps ResponseWriter to capture status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware returns HTTP middleware that records request metrics
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return m.MiddlewareWithPrefixGetter(next, nil)
}

// MiddlewareWithPrefixGetter returns HTTP middleware with custom prefix getter
func (m *Metrics) MiddlewareWithPrefixGetter(next http.Handler, getPrefixFn RoutePrefixGetter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		wrapped := &statusRecorder{ResponseWriter: w, statusCode: 200}

		// Track active connections
		m.ActiveConnections.Inc()
		defer m.ActiveConnections.Dec()

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		path := normalizePath(r, getPrefixFn)

		m.RequestsTotal.WithLabelValues(path, r.Method, strconv.Itoa(wrapped.statusCode)).Inc()
		m.RequestDuration.WithLabelValues(path, r.Method).Observe(duration)
	})
}

// normalizePath normalizes URL paths to prevent label cardinality explosion
func normalizePath(r *http.Request, getPrefixFn RoutePrefixGetter) string {
	// Try to get matched prefix from context
	if getPrefixFn != nil {
		if prefix := getPrefixFn(r); prefix != "" {
			// For prefix routes (ending with /), use prefix/*
			if strings.HasSuffix(prefix, "/") {
				return strings.TrimSuffix(prefix, "/") + "/*"
			}
			// For exact routes, return as-is
			return prefix
		}
	}

	// Fallback for exact path matches
	return r.URL.Path
}
