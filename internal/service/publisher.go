package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/broker"
	"github.com/chashtager/orchestron/internal/metrics"
	"github.com/chashtager/orchestron/internal/models"
)

// EventPublisherWithOutbox implements the EventPublisher interface with outbox pattern
type EventPublisherWithOutbox struct {
	broker        broker.EventPublisher
	outboxRepo    OutboxRepository
	routingService RoutingService
}

// NewEventPublisherWithOutbox creates a new event publisher with outbox pattern
func NewEventPublisherWithOutbox(
	broker broker.EventPublisher,
	outboxRepo OutboxRepository,
	routingService RoutingService,
) EventPublisher {
	return &EventPublisherWithOutbox{
		broker:        broker,
		outboxRepo:    outboxRepo,
		routingService: routingService,
	}
}

// PublishEvent publishes an event using the outbox pattern
func (p *EventPublisherWithOutbox) PublishEvent(ctx context.Context, event *models.Event) error {
	// Increment the counter for published events with event type and broker labels
	metrics.EventsPublished.WithLabelValues(event.Type, p.routingService.GetBrokerForEventType(event.Type)).Inc()
	
	// Serialize event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create outbox entry
	entry := &models.OutboxEntry{
		EventID:    event.ID,
		EventType:  event.Type,
		Payload:    string(payload),
		Status:     models.OutboxStatusPending,
		RetryCount: 0,
		CreatedAt:  time.Now(),
	}
	
	// Save to outbox
	if err := p.outboxRepo.Save(ctx, entry); err != nil {
		return fmt.Errorf("failed to save to outbox: %w", err)
	}
	
	log.Printf("Event %s saved to outbox", event.ID)
	return nil
}

// PublishEventBatch publishes a batch of events
func (p *EventPublisherWithOutbox) PublishEventBatch(ctx context.Context, events []*models.Event) error {
	for _, event := range events {
		if err := p.PublishEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
