package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/metrics"
	"github.com/chashtager/orchestron/internal/models"
)

// DeadLetterServiceImpl implements the DeadLetterService interface
type DeadLetterServiceImpl struct {
	repository DeadLetterRepository
}

// NewDeadLetterService creates a new dead letter service
func NewDeadLetterService(repository DeadLetterRepository) DeadLetterService {
	return &DeadLetterServiceImpl{
		repository: repository,
	}
}

// SendToDeadLetterQueue sends a failed event to the dead letter queue
func (s *DeadLetterServiceImpl) SendToDeadLetterQueue(ctx context.Context, event *models.Event, reason string) error {
	log.Printf("Sending event %s to dead letter queue: %s", event.ID, reason)
	

	// Increment the counter for failed events with event type and broker labels
	metrics.DLQEventsTotal.WithLabelValues(event.Type, reason).Inc()

	// Serialize event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create dead letter entry
	entry := &models.DeadLetterEntry{
		EventID:       event.ID,
		EventType:     event.Type,
		Payload:       string(payload),
		FailureReason: reason,
		CreatedAt:     time.Now(),
	}
	
	// Save to dead letter repository
	if err := s.repository.Save(ctx, entry); err != nil {
		return fmt.Errorf("failed to save to dead letter queue: %w", err)
	}
	
	return nil
}

// GetDeadLetterEntries returns dead letter entries
func (s *DeadLetterServiceImpl) GetDeadLetterEntries(ctx context.Context, limit, offset int) ([]*models.DeadLetterEntry, error) {
	return s.repository.FindEntries(ctx, limit, offset)
}

// ReprocessEntry reprocesses a dead letter entry
func (s *DeadLetterServiceImpl) ReprocessEntry(ctx context.Context, id int64) error {
	// In a real implementation, this would:
	// 1. Get the entry by ID
	// 2. Parse the event
	// 3. Send it to the event processor
	// 4. Mark as processed
	
	// For now, just mark as processed
	return s.repository.MarkAsProcessed(ctx, id, "Manually reprocessed")
}