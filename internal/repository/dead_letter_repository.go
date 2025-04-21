package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/chashtager/orchestron/internal/models"
	"github.com/jmoiron/sqlx"
)

// DeadLetterRepositoryImpl implements the DeadLetterRepository interface
type DeadLetterRepositoryImpl struct {
	db *sqlx.DB
}

// NewDeadLetterRepository creates a new dead letter repository
func NewDeadLetterRepository(database *Database) *DeadLetterRepositoryImpl {
	return &DeadLetterRepositoryImpl{
		db: database.GetDB(),
	}
}

// Save saves a dead letter entry
func (r *DeadLetterRepositoryImpl) Save(ctx context.Context, entry *models.DeadLetterEntry) error {
	query := `
		INSERT INTO dead_letter_queue (
			event_id, event_type, payload, failure_reason, created_at
		) VALUES (
			:event_id, :event_type, :payload, :failure_reason, :created_at
		) RETURNING id
	`
	
	// Named exec with context
	rows, err := r.db.NamedQueryContext(ctx, query, map[string]interface{}{
		"event_id":       entry.EventID,
		"event_type":     entry.EventType,
		"payload":        entry.Payload,
		"failure_reason": entry.FailureReason,
		"created_at":     entry.CreatedAt,
	})
	
	if err != nil {
		return fmt.Errorf("failed to insert dead letter entry: %w", err)
	}
	defer rows.Close()
	
	// Get the inserted ID
	if rows.Next() {
		if err := rows.Scan(&entry.ID); err != nil {
			return fmt.Errorf("failed to scan dead letter entry ID: %w", err)
		}
	}
	
	return nil
}

// FindEntries finds dead letter entries
func (r *DeadLetterRepositoryImpl) FindEntries(ctx context.Context, limit, offset int) ([]*models.DeadLetterEntry, error) {
	query := `
		SELECT 
			id, event_id, event_type, payload, failure_reason, created_at, processed_at, processing_result
		FROM 
			dead_letter_queue
		ORDER BY 
			created_at DESC
		LIMIT $1 OFFSET $2
	`
	
	var entries []*models.DeadLetterEntry
	err := r.db.SelectContext(
		ctx,
		&entries,
		query,
		limit,
		offset,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to find dead letter entries: %w", err)
	}
	
	return entries, nil
}

// MarkAsProcessed marks a dead letter entry as processed
func (r *DeadLetterRepositoryImpl) MarkAsProcessed(ctx context.Context, id int64, result string) error {
	query := `
		UPDATE 
			dead_letter_queue
		SET 
			processed_at = $1,
			processing_result = $2
		WHERE 
			id = $3
	`
	
	_, err := r.db.ExecContext(
		ctx,
		query,
		time.Now(),
		result,
		id,
	)
	
	if err != nil {
		return fmt.Errorf("failed to mark dead letter entry as processed: %w", err)
	}
	
	return nil
}
