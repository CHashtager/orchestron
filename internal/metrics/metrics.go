// metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Event metrics
	EventsReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_received_total",
			Help: "The total number of events received",
		},
		[]string{"event_type"},
	)
	
	EventsPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_published_total",
			Help: "The total number of events published to broker",
		},
		[]string{"event_type", "broker"},
	)
	
	EventsProcessingTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_processing_duration_seconds",
			Help:    "Time taken to process events",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
	
	// Validation metrics
	ValidationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_validation_errors_total",
			Help: "The total number of validation errors",
		},
		[]string{"event_type", "error_type"},
	)
	
	// Dead letter queue metrics
	DLQEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlq_events_total",
			Help: "The total number of events sent to DLQ",
		},
		[]string{"event_type", "reason"},
	)
	
	// Outbox metrics
	OutboxSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "outbox_pending_size",
			Help: "The current number of pending events in the outbox",
		},
	)
	
	OutboxProcessingTime = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "outbox_processing_duration_seconds",
			Help:    "Time taken to process outbox batch",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Transformation metrics
	TransformationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_transformation_errors_total",
			Help: "The total number of transformation errors",
		},
		[]string{"event_type", "error_type"},
	)

	TransformationTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_transformation_duration_seconds",
			Help:    "Time taken to transform events",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	// Enrichment metrics
	EnrichmentErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_enrichment_errors_total",
			Help: "The total number of enrichment errors",
		},
		[]string{"event_type", "error_type"},
	)

	EnrichmentTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_enrichment_duration_seconds",
			Help:    "Time taken to enrich events",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
)