package broker

import (
	"context"

	"github.com/chashtager/orchestron/internal/models"
)

// EventPublisher defines the interface for publishing events to a message broker
type EventPublisher interface {
	PublishToKafka(ctx context.Context, topic string, key string, event *models.Event) error
	PublishToRabbitMQ(ctx context.Context, exchange string, routingKey string, event *models.Event) error
	Close() error
}
