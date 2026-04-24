package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/sashko-guz/mage/internal/config"
	"github.com/sashko-guz/mage/internal/http/middleware"
	"github.com/sashko-guz/mage/internal/observability/health"
	"github.com/sashko-guz/mage/internal/observability/metrics"
	"github.com/sashko-guz/mage/internal/routes"
	"github.com/sashko-guz/mage/internal/thumbnail/handler"
)

type contextKey string

const routePrefixKey contextKey = "routePrefix"

// GetRoutePrefix returns the matched route prefix from request context
func GetRoutePrefix(r *http.Request) string {
	if v := r.Context().Value(routePrefixKey); v != nil {
		return v.(string)
	}
	return ""
}

// Router handles HTTP routing with middleware
type Router struct {
	cors    config.CORSConfig
	metrics *metrics.Metrics
}

// NewRouter creates a new router with CORS config and optional metrics
func NewRouter(cors config.CORSConfig, m *metrics.Metrics) *Router {
	return &Router{cors: cors, metrics: m}
}

// RegisterRoutes registers all application routes
func (rt *Router) RegisterRoutes(thumbnailHandler *handler.ThumbnailHandler, healthHandler *health.Handler, metricsEnabled bool, metricsPath string) {
	// Thumbnail endpoints
	routes.Add("/thumbs/", thumbnailHandler)
	routes.Add("/t/", thumbnailHandler)

	// Health endpoints
	routes.Add("/health", http.HandlerFunc(healthHandler.Liveness))
	routes.Add("/ready", http.HandlerFunc(healthHandler.Readiness))

	// Metrics endpoint
	if metricsEnabled && rt.metrics != nil {
		routes.Add(metricsPath, rt.metrics.Handler())
	}
}

// ServeHTTP implements http.Handler
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply CORS middleware
	middleware.CORS(w, rt.cors)

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Match route and serve (with metrics middleware if enabled)
	match := routes.Match(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}

	// Store matched prefix in context for metrics
	ctx := context.WithValue(r.Context(), routePrefixKey, match.Prefix)
	r = r.WithContext(ctx)

	// Strip matched prefix from path for prefix-match routes (ending with /)
	if strings.HasSuffix(match.Prefix, "/") {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(match.Prefix, "/"))
	}

	if rt.metrics != nil {
		rt.metrics.MiddlewareWithPrefixGetter(match.Handler, GetRoutePrefix).ServeHTTP(w, r)
	} else {
		match.Handler.ServeHTTP(w, r)
	}
}
