package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chashtager/orchestron/internal/models"
	"github.com/xeipuuv/gojsonschema"
)

// EventValidatorImpl implements the EventValidator interface
type EventValidatorImpl struct {
	schemaRepository SchemaRepository
}

// NewEventValidator creates a new event validator
func NewEventValidator(schemaRepository SchemaRepository) EventValidator {
	return &EventValidatorImpl{
		schemaRepository: schemaRepository,
	}
}

// Validate validates an event against its schema
func (v *EventValidatorImpl) Validate(ctx context.Context, event *models.Event) (*models.ValidationResult, error) {
	// Get schema for event type
	schema, err := v.schemaRepository.FindByType(ctx, event.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve schema: %w", err)
	}
	
	if schema == nil {
		return nil, fmt.Errorf("no schema found for event type: %s", event.Type)
	}
	
	// Load JSON schema
	schemaLoader := gojsonschema.NewStringLoader(schema.SchemaData)
	
	// Convert event payload to JSON
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	// Load event payload
	documentLoader := gojsonschema.NewBytesLoader(payloadBytes)
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	
	// Check result
	if result.Valid() {
		return &models.ValidationResult{Valid: true}, nil
	}
	
	// Collect validation errors
	errors := make([]string, 0, len(result.Errors()))
	for _, err := range result.Errors() {
		errors = append(errors, err.String())
	}
	
	return &models.ValidationResult{
		Valid:  false,
		Errors: errors,
	}, nil
}
