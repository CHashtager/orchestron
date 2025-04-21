package processor

import (
	"context"
	"time"

	"github.com/chashtager/orchestron/internal/models"
)

// Transformer defines the interface for event transformation
type Transformer interface {
	Transform(ctx context.Context, event *models.Event) (*models.Event, error)
}

// Enricher defines the interface for event enrichment
type Enricher interface {
	Enrich(ctx context.Context, event *models.Event) (*models.Event, error)
}

// EventTransformer implements the Transformer interface
type EventTransformer struct {
	transformations map[string]func(*models.Event) (*models.Event, error)
}

// NewEventTransformer creates a new event transformer
func NewEventTransformer() *EventTransformer {
	return &EventTransformer{
		transformations: make(map[string]func(*models.Event) (*models.Event, error)),
	}
}

// RegisterTransformation registers a transformation function for an event type
func (t *EventTransformer) RegisterTransformation(
	eventType string,
	transform func(*models.Event) (*models.Event, error),
) {
	t.transformations[eventType] = transform
}

// Transform applies the registered transformation to an event
func (t *EventTransformer) Transform(
	ctx context.Context,
	event *models.Event,
) (*models.Event, error) {
	transform, exists := t.transformations[event.Type]
	if !exists {
		return event, nil // No transformation needed
	}

	return transform(event)
}

// EventEnricher implements the Enricher interface
type EventEnricher struct {
	enrichers map[string]func(*models.Event) (*models.Event, error)
}

// NewEventEnricher creates a new event enricher
func NewEventEnricher() *EventEnricher {
	return &EventEnricher{
		enrichers: make(map[string]func(*models.Event) (*models.Event, error)),
	}
}

// RegisterEnricher registers an enrichment function for an event type
func (e *EventEnricher) RegisterEnricher(
	eventType string,
	enrich func(*models.Event) (*models.Event, error),
) {
	e.enrichers[eventType] = enrich
}

// Enrich applies the registered enrichment to an event
func (e *EventEnricher) Enrich(
	ctx context.Context,
	event *models.Event,
) (*models.Event, error) {
	enrich, exists := e.enrichers[event.Type]
	if !exists {
		return event, nil // No enrichment needed
	}

	return enrich(event)
}

// DefaultTransformations returns a map of default transformations
func DefaultTransformations() map[string]func(*models.Event) (*models.Event, error) {
	return map[string]func(*models.Event) (*models.Event, error){
		// Example: Convert all timestamps to ISO 8601 format
		"*": func(event *models.Event) (*models.Event, error) {
			// Create a copy of the event to avoid modifying the original
			transformed := &models.Event{
				ID:        event.ID,
				Type:      event.Type,
				Payload:   make(map[string]interface{}),
				Metadata:  make(map[string]string),
				Timestamp: event.Timestamp,
			}

			// Copy metadata
			for k, v := range event.Metadata {
				transformed.Metadata[k] = v
			}

			// Transform payload
			for k, v := range event.Payload {
				if t, ok := v.(time.Time); ok {
					transformed.Payload[k] = t.Format(time.RFC3339)
				} else {
					transformed.Payload[k] = v
				}
			}

			return transformed, nil
		},
	}
}

// DefaultEnrichers returns a map of default enrichers
func DefaultEnrichers() map[string]func(*models.Event) (*models.Event, error) {
	return map[string]func(*models.Event) (*models.Event, error){
		// Example: Add processing timestamp to all events
		"*": func(event *models.Event) (*models.Event, error) {
			enriched := &models.Event{
				ID:        event.ID,
				Type:      event.Type,
				Payload:   event.Payload,
				Metadata:  make(map[string]string),
				Timestamp: event.Timestamp,
			}

			// Copy existing metadata
			for k, v := range event.Metadata {
				enriched.Metadata[k] = v
			}

			// Add processing timestamp
			enriched.Metadata["processed_at"] = time.Now().Format(time.RFC3339)

			return enriched, nil
		},
	}
} 