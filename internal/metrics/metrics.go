package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all the application metrics
type Metrics struct {
	// HTTP request metrics
	HTTPRequestTotal    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Storage operation metrics
	StorageOperationTotal    *prometheus.CounterVec
	StorageOperationDuration *prometheus.HistogramVec

	// Event publishing metrics
	EventPublishTotal    *prometheus.CounterVec
	EventPublishDuration *prometheus.HistogramVec

	// Schema validation metrics
	SchemaValidationTotal    *prometheus.CounterVec
	SchemaValidationDuration *prometheus.HistogramVec
}

// Global metrics instance with mutex for thread safety
var (
	globalMetrics *Metrics
	metricsMutex  sync.Mutex
)

// NewMetrics creates a new Metrics instance with all required metrics
func NewMetrics() *Metrics {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	
	// Return existing instance if already created
	if globalMetrics != nil {
		return globalMetrics
	}
	
	m := &Metrics{
		// HTTP request metrics
		HTTPRequestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),

		// Storage operation metrics
		StorageOperationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "storage_operations_total",
			Help: "Total number of storage operations",
		}, []string{"operation", "status"}),

		StorageOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "storage_operation_duration_seconds",
			Help:    "Storage operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation", "status"}),

		// Event publishing metrics
		EventPublishTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "event_publish_total",
			Help: "Total number of event publish operations",
		}, []string{"event_type", "status"}),

		EventPublishDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "event_publish_duration_seconds",
			Help:    "Event publish duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"event_type", "status"}),

		// Schema validation metrics
		SchemaValidationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "schema_validation_total",
			Help: "Total number of schema validation operations",
		}, []string{"collection", "status"}),

		SchemaValidationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "schema_validation_duration_seconds",
			Help:    "Schema validation duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"collection", "status"}),
	}
	
	// Register metrics with the default registry
	registerMetrics(m)
	
	// Store as global instance
	globalMetrics = m
	
	return m
}

// registerMetrics registers all metrics with the default registry
func registerMetrics(m *Metrics) {
	// Try to register each metric, ignore if already registered
	registerOrGet(m.HTTPRequestTotal)
	registerOrGet(m.HTTPRequestDuration)
	registerOrGet(m.StorageOperationTotal)
	registerOrGet(m.StorageOperationDuration)
	registerOrGet(m.EventPublishTotal)
	registerOrGet(m.EventPublishDuration)
	registerOrGet(m.SchemaValidationTotal)
	registerOrGet(m.SchemaValidationDuration)
}

// registerOrGet tries to register a metric, returns the existing one if already registered
func registerOrGet(c prometheus.Collector) prometheus.Collector {
	if err := prometheus.Register(c); err != nil {
		// If already registered, return the existing collector
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
	}
	return c
}
