package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/metrics"
	"github.com/chashtager/orchestron/internal/models"
	"github.com/chashtager/orchestron/internal/service"
	"github.com/sethvargo/go-retry"
)

// Custom error types for different failure scenarios
type (
	PayloadSizeError struct {
		ActualSize   int64
		MaxSize      int64
		ErrorMessage string
	}
	SchemaValidationError struct {
		EventType string
		Errors    []string
	}
	ProcessingError struct {
		Step    string
		EventID string
		Err     error
	}
)

func (e *PayloadSizeError) Error() string {
	return fmt.Sprintf("payload size %d exceeds maximum allowed size %d: %s", 
		e.ActualSize, e.MaxSize, e.ErrorMessage)
}

func (e *SchemaValidationError) Error() string {
	return fmt.Sprintf("validation failed for event type %s: %v", 
		e.EventType, e.Errors)
}

func (e *ProcessingError) Error() string {
	return fmt.Sprintf("error in %s step for event %s: %v", 
		e.Step, e.EventID, e.Err)
}

// EventProcessorImpl implements the EventProcessor interface
type EventProcessorImpl struct {
	validator      service.EventValidator
	publisher      service.EventPublisher
	dlqService     service.DeadLetterService
	schemaProvider service.SchemaProvider
	transformer    Transformer
	enricher       Enricher
	maxPayloadSize int64
	retryBaseDelay time.Duration
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	validator service.EventValidator,
	publisher service.EventPublisher,
	dlqService service.DeadLetterService,
	schemaProvider service.SchemaProvider,
	maxPayloadSize int64,
	retryBaseDelay time.Duration,
) service.EventProcessor {
	processor := &EventProcessorImpl{
		validator:      validator,
		publisher:      publisher,
		dlqService:     dlqService,
		schemaProvider: schemaProvider,
		transformer:    NewEventTransformer(),
		enricher:       NewEventEnricher(),
		maxPayloadSize: maxPayloadSize,
		retryBaseDelay: retryBaseDelay,
	}

	// Register default transformations and enrichers
	for eventType, transform := range DefaultTransformations() {
		processor.transformer.(*EventTransformer).RegisterTransformation(eventType, transform)
	}

	for eventType, enrich := range DefaultEnrichers() {
		processor.enricher.(*EventEnricher).RegisterEnricher(eventType, enrich)
	}

	return processor
}

// ProcessEvent processes an incoming event
func (p *EventProcessorImpl) ProcessEvent(ctx context.Context, event *models.Event) error {
	log.Printf("Starting to process event %s of type %s", event.ID, event.Type)
	
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.EventsProcessingTime.WithLabelValues(event.Type).Observe(duration)
		log.Printf("Event %s processing completed in %.2f seconds", event.ID, duration)
	}()

	// 1. Validate payload size
	if err := p.validatePayloadSize(event); err != nil {
		log.Printf("Payload size validation failed for event %s: %v", event.ID, err)
		return p.handleError(ctx, event, "payload_size_exceeded", err)
	}
	log.Printf("Payload size validation passed for event %s", event.ID)

	// 2. Get and validate schema
	if err := p.validateSchema(ctx, event); err != nil {
		log.Printf("Schema validation failed for event %s: %v", event.ID, err)
		return p.handleError(ctx, event, "schema_validation_failed", err)
	}
	log.Printf("Schema validation passed for event %s", event.ID)

	// 3. Transform event
	transformedEvent, err := p.transformEvent(ctx, event)
	if err != nil {
		log.Printf("Event transformation failed for event %s: %v", event.ID, err)
		return p.handleError(ctx, event, "transformation_failed", err)
	}
	log.Printf("Event transformation completed for event %s", event.ID)

	// 4. Enrich event
	enrichedEvent, err := p.enrichEvent(ctx, transformedEvent)
	if err != nil {
		log.Printf("Event enrichment failed for event %s: %v", event.ID, err)
		return p.handleError(ctx, event, "enrichment_failed", err)
	}
	log.Printf("Event enrichment completed for event %s", event.ID)

	// 5. Publish event
	if err := p.publishEvent(ctx, enrichedEvent); err != nil {
		log.Printf("Event publishing failed for event %s: %v", event.ID, err)
		return p.handleError(ctx, event, "publishing_failed", err)
	}
	log.Printf("Event %s published successfully", event.ID)

	metrics.EventsPublished.WithLabelValues(event.Type, "success").Inc()
	return nil
}

// validateSchema validates the event against its schema
func (p *EventProcessorImpl) validateSchema(ctx context.Context, event *models.Event) error {
	log.Printf("Starting schema validation for event %s", event.ID)
	
	// Get schema with retry
	err := retry.Do(ctx, retry.NewExponential(p.retryBaseDelay), func(ctx context.Context) error {
		log.Printf("Attempting to get schema for event type %s", event.Type)
		schema, err := p.schemaProvider.GetSchemaForEventType(ctx, event.Type)
		if err != nil {
			log.Printf("Error getting schema for event type %s: %v", event.Type, err)
			return retry.RetryableError(err)
		}
		if schema == nil {
			log.Printf("No schema found for event type %s", event.Type)
			return fmt.Errorf("no schema found for event type: %s", event.Type)
		}
		log.Printf("Successfully retrieved schema for event type %s", event.Type)
		return nil
	})
	if err != nil {
		log.Printf("Failed to get schema for event %s: %v", event.ID, err)
		return fmt.Errorf("failed to get schema: %w", err)
	}

	// Validate event against schema with retry
	var validationResult *models.ValidationResult
	err = retry.Do(ctx, retry.NewExponential(p.retryBaseDelay), func(ctx context.Context) error {
		log.Printf("Validating event %s against schema", event.ID)
		var err error
		validationResult, err = p.validator.Validate(ctx, event)
		if err != nil {
			log.Printf("Error validating event %s: %v", event.ID, err)
			return retry.RetryableError(err)
		}
		log.Printf("Event %s validation result: %v", event.ID, validationResult.Valid)
		return nil
	})
	if err != nil {
		log.Printf("Failed to validate event %s: %v", event.ID, err)
		return fmt.Errorf("failed to validate event: %w", err)
	}

	if !validationResult.Valid {
		log.Printf("Event %s failed validation: %v", event.ID, validationResult.Errors)
		return &SchemaValidationError{
			EventType: event.Type,
			Errors:    validationResult.Errors,
		}
	}

	log.Printf("Schema validation passed for event %s", event.ID)
	return nil
}

// transformEvent transforms the event using the registered transformer
func (p *EventProcessorImpl) transformEvent(ctx context.Context, event *models.Event) (*models.Event, error) {
	start := time.Now()
	defer func() {
		metrics.TransformationTime.WithLabelValues(event.Type).Observe(
			time.Since(start).Seconds(),
		)
	}()

	transformedEvent, err := p.transformer.Transform(ctx, event)
	if err != nil {
		metrics.TransformationErrors.WithLabelValues(event.Type, "transformation_error").Inc()
		return nil, &ProcessingError{
			Step:    "transformation",
			EventID: event.ID,
			Err:     err,
		}
	}

	return transformedEvent, nil
}

// enrichEvent enriches the event using the registered enricher
func (p *EventProcessorImpl) enrichEvent(ctx context.Context, event *models.Event) (*models.Event, error) {
	start := time.Now()
	defer func() {
		metrics.EnrichmentTime.WithLabelValues(event.Type).Observe(
			time.Since(start).Seconds(),
		)
	}()

	enrichedEvent, err := p.enricher.Enrich(ctx, event)
	if err != nil {
		metrics.EnrichmentErrors.WithLabelValues(event.Type, "enrichment_error").Inc()
		return nil, &ProcessingError{
			Step:    "enrichment",
			EventID: event.ID,
			Err:     err,
		}
	}

	return enrichedEvent, nil
}

// publishEvent publishes the event to the broker with retry
func (p *EventProcessorImpl) publishEvent(ctx context.Context, event *models.Event) error {
	return retry.Do(ctx, retry.NewExponential(p.retryBaseDelay), func(ctx context.Context) error {
		if err := p.publisher.PublishEvent(ctx, event); err != nil {
			return retry.RetryableError(err)
		}
		return nil
	})
}

// handleError handles processing errors by sending events to DLQ
func (p *EventProcessorImpl) handleError(
	ctx context.Context,
	event *models.Event,
	errorType string,
	err error,
) error {
	metrics.ValidationErrors.WithLabelValues(event.Type, errorType).Inc()
	
	// Send to DLQ
	if dlqErr := p.dlqService.SendToDeadLetterQueue(ctx, event, err.Error()); dlqErr != nil {
		return fmt.Errorf("failed to send to DLQ: %w (original error: %w)", dlqErr, err)
	}

	metrics.DLQEventsTotal.WithLabelValues(event.Type, errorType).Inc()
	return err
}

// validatePayloadSize validates the event payload size
func (p *EventProcessorImpl) validatePayloadSize(event *models.Event) error {
	// Marshal payload to JSON to get actual size
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Check if payload size exceeds limit
	if int64(len(payloadBytes)) > p.maxPayloadSize {
		return &PayloadSizeError{
			ActualSize:   int64(len(payloadBytes)),
			MaxSize:      p.maxPayloadSize,
			ErrorMessage: "payload size exceeds maximum allowed size",
		}
	}

	return nil
}