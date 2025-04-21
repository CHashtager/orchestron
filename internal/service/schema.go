package service

import (
	"context"

	"github.com/chashtager/orchestron/internal/models"
)

// SchemaProviderImpl implements the SchemaProvider interface
type SchemaProviderImpl struct {
    repository SchemaRepository
}

// NewSchemaProvider creates a new schema provider
func NewSchemaProvider(repository SchemaRepository) SchemaProvider {
    return &SchemaProviderImpl{
        repository: repository,
    }
}

// GetSchemaForEventType returns the schema for an event type
func (p *SchemaProviderImpl) GetSchemaForEventType(ctx context.Context, eventType string) (*models.EventSchema, error) {
    // Find the latest schema version for the event type
    return p.repository.FindByType(ctx, eventType)
}