// processor/outbox_processor.go
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chashtager/orchestron/internal/broker"
	"github.com/chashtager/orchestron/internal/models"
	"github.com/chashtager/orchestron/internal/service"
	"github.com/sethvargo/go-retry"
)

// OutboxProcessorImpl implements the OutboxProcessor interface
type OutboxProcessorImpl struct {
	outboxRepo      service.OutboxRepository
	broker          broker.EventPublisher
	routingService  service.RoutingService
	processInterval time.Duration
	batchSize       int
	stopCh          chan struct{}
	lock            sync.Mutex
	isProcessing    bool
}

// NewOutboxProcessor creates a new outbox processor
func NewOutboxProcessor(
	outboxRepo service.OutboxRepository,
	broker broker.EventPublisher,
	routingService service.RoutingService,
	processInterval time.Duration,
	batchSize int,
) service.OutboxProcessor {
	return &OutboxProcessorImpl{
		outboxRepo:      outboxRepo,
		broker:          broker,
		routingService:  routingService,
		processInterval: processInterval,
		batchSize:       batchSize,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the outbox processor
func (p *OutboxProcessorImpl) Start(ctx context.Context) {
	log.Println("Starting outbox processor")

	ticker := time.NewTicker(p.processInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Outbox processor tick - checking for pending entries")
			if err := p.ProcessPendingEntries(ctx); err != nil {
				log.Printf("Error processing outbox entries: %v", err)
			}
		case <-ctx.Done():
			log.Println("Stopping outbox processor due to context cancellation")
			return
		case <-p.stopCh:
			log.Println("Stopping outbox processor")
			return
		}
	}
}

// Stop stops the outbox processor
func (p *OutboxProcessorImpl) Stop() error {
	close(p.stopCh)
	return nil
}

// ProcessPendingEntries processes pending outbox entries
func (p *OutboxProcessorImpl) ProcessPendingEntries(ctx context.Context) error {
	log.Println("Processing pending outbox entries")

	// Get pending entries
	entries, err := p.outboxRepo.FindPendingEntries(ctx, p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending entries: %w", err)
	}

	if len(entries) == 0 {
		log.Println("No pending entries found")
		return nil
	}

	log.Printf("Found %d pending entries to process", len(entries))
	return p.processBatch(ctx, entries)
}

// processBatch processes a batch of outbox entries
func (p *OutboxProcessorImpl) processBatch(ctx context.Context, entries []*models.OutboxEntry) error {
	for _, entry := range entries {
		// Parse event
		var event models.Event
		if err := json.Unmarshal([]byte(entry.Payload), &event); err != nil {
			log.Printf("Error parsing event %s: %v", entry.EventID, err)
			p.outboxRepo.IncrementRetryCount(ctx, entry.ID)
			continue
		}

		// Determine broker type
		brokerType := p.routingService.GetBrokerForEventType(event.Type)

		// Publish to broker with retry
		backoff := retry.NewExponential(1 * time.Second)
		err := retry.Do(ctx, backoff, func(ctx context.Context) error {
			var publishErr error
			switch brokerType {
			case "kafka":
				publishErr = p.publishToKafka(ctx, &event, entry)
			case "rabbitmq":
				publishErr = p.publishToRabbitMQ(ctx, &event, entry)
			default:
				publishErr = p.publishToKafka(ctx, &event, entry) // Default to Kafka
			}

			if publishErr != nil {
				log.Printf("Error publishing event %s, retrying: %v", entry.EventID, publishErr)
				return retry.RetryableError(publishErr)
			}
			return nil
		})

		if err != nil {
			log.Printf("Error publishing event %s: %v", entry.EventID, err)

			// Increment retry count
			if err := p.outboxRepo.IncrementRetryCount(ctx, entry.ID); err != nil {
				log.Printf("Error incrementing retry count for entry %d: %v", entry.ID, err)
			}

			// Check if max retries reached
			if entry.RetryCount >= p.routingService.GetMaxRetries(entry.EventType) {
				log.Printf("Max retries reached for event %s, marking as failed", entry.EventID)
				if err := p.outboxRepo.MarkAsFailed(ctx, entry.ID); err != nil {
					log.Printf("Error marking entry %d as failed: %v", entry.ID, err)
				}
			}

			continue
		}

		// Mark as published
		if err := p.outboxRepo.MarkAsPublished(ctx, entry.ID); err != nil {
			log.Printf("Error marking entry %d as published: %v", entry.ID, err)
		}
	}

	return nil
}

// tryAcquireLock tries to acquire the processing lock
func (p *OutboxProcessorImpl) tryAcquireLock() bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.isProcessing {
		return false
	}

	p.isProcessing = true
	return true
}

// releaseLock releases the processing lock
func (p *OutboxProcessorImpl) releaseLock() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.isProcessing = false
}

// publishToKafka publishes an event to Kafka
func (p *OutboxProcessorImpl) publishToKafka(ctx context.Context, event *models.Event, entry *models.OutboxEntry) error {
	topic := p.routingService.GetTopicForEventType(event.Type)
	return p.broker.PublishToKafka(ctx, topic, event.ID, event)
}

// publishToRabbitMQ publishes an event to RabbitMQ
func (p *OutboxProcessorImpl) publishToRabbitMQ(ctx context.Context, event *models.Event, entry *models.OutboxEntry) error {
	exchange := p.routingService.GetExchangeForEventType(event.Type)
	routingKey := p.routingService.GetRoutingKeyForEventType(event.Type)
	return p.broker.PublishToRabbitMQ(ctx, exchange, routingKey, event)
}
