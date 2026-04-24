package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Recorder interface for recording metrics (avoids import cycles)
type Recorder interface {
	RecordCacheHit(cacheType, layer string)
	RecordCacheMiss(cacheType, layer string)
	RecordStorageOperation(operation, driver string, durationSeconds float64)
	RecordImageProcessing(format string, durationSeconds float64)
}

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	// HTTP metrics
	RequestsTotal     *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	ActiveConnections prometheus.Gauge

	// Image processing
	ProcessingDuration *prometheus.HistogramVec

	// Cache metrics
	CacheHits   *prometheus.CounterVec
	CacheMisses *prometheus.CounterVec

	// Storage metrics
	StorageDuration *prometheus.HistogramVec

	registry *prometheus.Registry
}

// New creates a new Metrics instance with all collectors registered
func New() *Metrics {
	registry := prometheus.NewRegistry()

	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mage_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"path", "method", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mage_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"path", "method"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "mage_http_active_connections",
				Help: "Number of active HTTP connections",
			},
		),
		ProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mage_image_processing_duration_seconds",
				Help:    "Image processing duration in seconds",
				Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"format"},
		),
		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mage_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"type", "layer"},
		),
		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mage_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"type", "layer"},
		),
		StorageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mage_storage_operation_duration_seconds",
				Help:    "Storage operation duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"operation", "driver"},
		),
		registry: registry,
	}

	// Register all collectors
	registry.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.ActiveConnections,
		m.ProcessingDuration,
		m.CacheHits,
		m.CacheMisses,
		m.StorageDuration,
	)

	// Register Go runtime metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return m
}

// Handler returns an HTTP handler for the /metrics endpoint
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(cacheType, layer string) {
	m.CacheHits.WithLabelValues(cacheType, layer).Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(cacheType, layer string) {
	m.CacheMisses.WithLabelValues(cacheType, layer).Inc()
}

// RecordStorageOperation records a storage operation duration
func (m *Metrics) RecordStorageOperation(operation, driver string, durationSeconds float64) {
	m.StorageDuration.WithLabelValues(operation, driver).Observe(durationSeconds)
}

// RecordImageProcessing records image processing duration
func (m *Metrics) RecordImageProcessing(format string, durationSeconds float64) {
	m.ProcessingDuration.WithLabelValues(format).Observe(durationSeconds)
}
