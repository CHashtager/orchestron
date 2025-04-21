package repository

import (
	"context"
	"fmt"

	"github.com/chashtager/orchestron/internal/models"
	"github.com/jmoiron/sqlx"
)

// SchemaRepositoryImpl implements the SchemaRepository interface
type SchemaRepositoryImpl struct {
	db *sqlx.DB
}

// NewSchemaRepository creates a new schema repository
func NewSchemaRepository(database *Database) *SchemaRepositoryImpl {
	return &SchemaRepositoryImpl{
		db: database.GetDB(),
	}
}

// FindByTypeAndVersion finds a schema by event type and version
func (r *SchemaRepositoryImpl) FindByTypeAndVersion(ctx context.Context, eventType, version string) (*models.EventSchema, error) {
	query := `
		SELECT 
			id, event_type, version, schema_format, schema_data, created_at, updated_at
		FROM 
			event_schemas
		WHERE 
			event_type = $1 AND version = $2
	`
	
	var schema models.EventSchema
	err := r.db.GetContext(ctx, &schema, query, eventType, version)
	
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find schema: %w", err)
	}
	
	return &schema, nil
}

// FindByType finds the latest schema for an event type
func (r *SchemaRepositoryImpl) FindByType(ctx context.Context, eventType string) (*models.EventSchema, error) {
	query := `
		SELECT 
			id, event_type, version, schema_format, schema_data, created_at, updated_at
		FROM 
			event_schemas
		WHERE 
			event_type = $1
		ORDER BY 
			version DESC
		LIMIT 1
	`
	
	var schema models.EventSchema
	err := r.db.GetContext(ctx, &schema, query, eventType)
	
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find schema: %w", err)
	}
	
	return &schema, nil
}

// Save saves a new schema
func (r *SchemaRepositoryImpl) Save(ctx context.Context, schema *models.EventSchema) error {
	query := `
		INSERT INTO event_schemas (
			event_type, version, schema_format, schema_data, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, NOW(), NOW()
		)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		schema.EventType,
		schema.Version,
		schema.SchemaFormat,
		schema.SchemaData,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save schema: %w", err)
	}
	
	return nil
}