package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/models"
	"github.com/jmoiron/sqlx"
)

// OutboxRepositoryImpl implements the OutboxRepository interface
type OutboxRepositoryImpl struct {
	db *sqlx.DB
}

// NewOutboxRepository creates a new outbox repository
func NewOutboxRepository(database *Database) *OutboxRepositoryImpl {
	return &OutboxRepositoryImpl{
		db: database.GetDB(),
	}
}

// Save saves an outbox entry
func (r *OutboxRepositoryImpl) Save(ctx context.Context, entry *models.OutboxEntry) error {
	log.Printf("Saving outbox entry for event %s", entry.EventID)
	
	query := `
		INSERT INTO event_outbox (
			event_id, event_type, payload, status, retry_count, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		entry.EventID,
		entry.EventType,
		entry.Payload,
		entry.Status,
		entry.RetryCount,
		entry.CreatedAt,
	)
	
	if err != nil {
		log.Printf("Failed to save outbox entry for event %s: %v", entry.EventID, err)
		return fmt.Errorf("failed to save outbox entry: %w", err)
	}
	
	log.Printf("Successfully saved outbox entry for event %s", entry.EventID)
	return nil
}

// FindPendingEntries finds pending outbox entries
func (r *OutboxRepositoryImpl) FindPendingEntries(ctx context.Context, limit int) ([]*models.OutboxEntry, error) {
	log.Printf("Finding pending outbox entries (limit: %d)", limit)
	
	query := `
		SELECT 
			id, event_id, event_type, payload, status, retry_count, created_at, published_at
		FROM 
			event_outbox
		WHERE 
			status = $1
		ORDER BY 
			created_at ASC
		LIMIT $2
	`
	
	var entries []*models.OutboxEntry
	err := r.db.SelectContext(ctx, &entries, query, models.OutboxStatusPending, limit)
	
	if err != nil {
		log.Printf("Failed to find pending outbox entries: %v", err)
		return nil, fmt.Errorf("failed to find pending entries: %w", err)
	}
	
	log.Printf("Found %d pending outbox entries", len(entries))
	return entries, nil
}

// MarkAsPublished marks an outbox entry as published within a transaction
func (r *OutboxRepositoryImpl) MarkAsPublished(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	query := `
		UPDATE 
			event_outbox
		SET 
			status = $1, 
			published_at = $2
		WHERE 
			id = $3
	`
	
	_, err = tx.ExecContext(
		ctx,
		query,
		models.OutboxStatusPublished,
		time.Now(),
		id,
	)
	
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark outbox entry as published: %w", err)
	}
	
	return tx.Commit()
}

// IncrementRetryCount increments the retry count for an outbox entry within a transaction
func (r *OutboxRepositoryImpl) IncrementRetryCount(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	query := `
		UPDATE 
			event_outbox
		SET 
			retry_count = retry_count + 1
		WHERE 
			id = $1
	`
	
	_, err = tx.ExecContext(ctx, query, id)
	
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to increment retry count for outbox entry: %w", err)
	}
	
	return tx.Commit()
}

// MarkAsFailed marks an outbox entry as failed within a transaction
func (r *OutboxRepositoryImpl) MarkAsFailed(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	query := `
		UPDATE 
			event_outbox
		SET 
			status = $1
		WHERE 
			id = $2
	`
	
	_, err = tx.ExecContext(
		ctx,
		query,
		models.OutboxStatusFailed,
		id,
	)
	
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark outbox entry as failed: %w", err)
	}
	
	return tx.Commit()
}

// CleanupOldEntries removes old published entries
func (r *OutboxRepositoryImpl) CleanupOldEntries(ctx context.Context, olderThan time.Duration) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	query := `
		DELETE FROM 
			event_outbox
		WHERE 
			status = $1 AND 
			published_at < $2
	`
	
	_, err = tx.ExecContext(
		ctx,
		query,
		models.OutboxStatusPublished,
		time.Now().Add(-olderThan),
	)
	
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to cleanup old outbox entries: %w", err)
	}
	
	return tx.Commit()
}

// CreateIndexes creates necessary indexes for the outbox table
func (r *OutboxRepositoryImpl) CreateIndexes(ctx context.Context) error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_event_outbox_status ON event_outbox(status)`,
		`CREATE INDEX IF NOT EXISTS idx_event_outbox_created_at ON event_outbox(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_event_outbox_event_type ON event_outbox(event_type)`,
	}
	
	for _, index := range indexes {
		if _, err := r.db.ExecContext(ctx, index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	
	return nil
}
