package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/config"
	"github.com/chashtager/orchestron/internal/models"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// KafkaPublisher implements the EventPublisher interface for Kafka
type KafkaPublisher struct {
	producer *kafka.Producer
	config   *config.KafkaConfig
}

// NewKafkaPublisher creates a new Kafka publisher
func NewKafkaPublisher(config *config.KafkaConfig) (*KafkaPublisher, error) {
	// Kafka configuration
	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":        config.BootstrapServers,
		"client.id":                config.ClientID,
		"acks":                     "all",
		"retries":                  config.MaxRetries,
		"retry.backoff.ms":         config.RetryBackoffMs,
		"delivery.timeout.ms":      config.DeliveryTimeoutMs,
		"enable.idempotence":       true,
		"queue.buffering.max.ms":   config.QueueBufferingMaxMs,
		"queue.buffering.max.kbytes": config.QueueBufferingMaxKbytes,
	}
	
	// Create producer
	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	
	// Start a goroutine to handle delivery reports
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Printf("Failed to deliver message to %v: %v", 
						ev.TopicPartition, ev.TopicPartition.Error)
				} else {
					log.Printf("Successfully delivered message to %v", ev.TopicPartition)
				}
			}
		}
	}()
	
	return &KafkaPublisher{
		producer: producer,
		config:   config,
	}, nil
}

// PublishToKafka publishes an event to Kafka
func (p *KafkaPublisher) PublishToKafka(ctx context.Context, topic string, key string, event *models.Event) error {
	// Serialize event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create message
	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(key),
		Value: eventBytes,
		Headers: []kafka.Header{
			{
				Key:   "event_type",
				Value: []byte(event.Type),
			},
			{
				Key:   "event_id",
				Value: []byte(event.ID),
			},
			{
				Key:   "timestamp",
				Value: []byte(event.Timestamp.Format(time.RFC3339)),
			},
		},
	}
	
	// Add context timeout
	deliveryChan := make(chan kafka.Event)
	
	// Produce message
	if err := p.producer.Produce(message, deliveryChan); err != nil {
		return fmt.Errorf("failed to produce message to Kafka: %w", err)
	}
	
	// Wait for delivery report or context cancellation
	select {
	case ev := <-deliveryChan:
		m := ev.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			return fmt.Errorf("failed to deliver message to Kafka: %w", m.TopicPartition.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	
	return nil
}

// PublishToRabbitMQ is a no-op for KafkaPublisher
func (p *KafkaPublisher) PublishToRabbitMQ(ctx context.Context, exchange string, routingKey string, event *models.Event) error {
	return fmt.Errorf("RabbitMQ publishing not supported by KafkaPublisher")
}

// Close closes the Kafka producer
func (p *KafkaPublisher) Close() error {
	p.producer.Flush(p.config.FlushTimeoutMs)
	p.producer.Close()
	return nil
}
