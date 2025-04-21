package service

import (
	"context"

	"github.com/chashtager/orchestron/internal/models"
)

// EventValidator validates events against schemas
type EventValidator interface {
	Validate(ctx context.Context, event *models.Event) (*models.ValidationResult, error)
}

// EventPublisher publishes events to a message broker
type EventPublisher interface {
	PublishEvent(ctx context.Context, event *models.Event) error
	PublishEventBatch(ctx context.Context, events []*models.Event) error
}

// DeadLetterService handles failed events
type DeadLetterService interface {
	SendToDeadLetterQueue(ctx context.Context, event *models.Event, reason string) error
	GetDeadLetterEntries(ctx context.Context, limit, offset int) ([]*models.DeadLetterEntry, error)
	ReprocessEntry(ctx context.Context, id int64) error
}

// RoutingService determines how events should be routed
type RoutingService interface {
	GetTopicForEventType(eventType string) string
	GetExchangeForEventType(eventType string) string
	GetRoutingKeyForEventType(eventType string) string
	GetBrokerForEventType(eventType string) string
	GetMaxRetries(eventType string) int
}

// SchemaProvider provides schemas for event types
type SchemaProvider interface {
	GetSchemaForEventType(ctx context.Context, eventType string) (*models.EventSchema, error)
}

// OutboxRepository manages outbox entries
type OutboxRepository interface {
	Save(ctx context.Context, entry *models.OutboxEntry) error
	FindPendingEntries(ctx context.Context, limit int) ([]*models.OutboxEntry, error)
	MarkAsPublished(ctx context.Context, id int64) error
	IncrementRetryCount(ctx context.Context, id int64) error
	MarkAsFailed(ctx context.Context, id int64) error
}

// DeadLetterRepository manages dead letter entries
type DeadLetterRepository interface {
	Save(ctx context.Context, entry *models.DeadLetterEntry) error
	FindEntries(ctx context.Context, limit, offset int) ([]*models.DeadLetterEntry, error)
	MarkAsProcessed(ctx context.Context, id int64, result string) error
}

// SchemaRepository manages event schemas
type SchemaRepository interface {
	FindByTypeAndVersion(ctx context.Context, eventType, version string) (*models.EventSchema, error)
	FindByType(ctx context.Context, eventType string) (*models.EventSchema, error)
	Save(ctx context.Context, schema *models.EventSchema) error
}

// EventProcessor processes incoming events
type EventProcessor interface {
	ProcessEvent(ctx context.Context, event *models.Event) error
}

// OutboxProcessor processes outbox entries
type OutboxProcessor interface {
	Start(ctx context.Context)
	Stop() error
	ProcessPendingEntries(ctx context.Context) error
}