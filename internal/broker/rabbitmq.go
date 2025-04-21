package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/config"
	"github.com/chashtager/orchestron/internal/models"
	"github.com/streadway/amqp"
)

// RabbitMQPublisher implements the EventPublisher interface for RabbitMQ
type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.RabbitMQConfig
}

// NewRabbitMQPublisher creates a new RabbitMQ publisher
func NewRabbitMQPublisher(config *config.RabbitMQConfig) (*RabbitMQPublisher, error) {
	// Create connection string
	connStr := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Vhost,
	)
	
	// Connect to RabbitMQ
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	
	// Create channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}
	
	// Set QoS
	if err := channel.Qos(
		config.PrefetchCount,
		0,     // prefetch size
		false, // global
	); err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}
	
	// Create publisher
	publisher := &RabbitMQPublisher{
		conn:    conn,
		channel: channel,
		config:  config,
	}
	
	// Declare exchanges
	err = publisher.declareExchanges()
	if err != nil {
		publisher.Close()
		return nil, err
	}
	
	return publisher, nil
}

// declareExchanges declares all needed exchanges
func (p *RabbitMQPublisher) declareExchanges() error {
	// Declare primary events exchange
	err := p.channel.ExchangeDeclare(
		"events",   // name
		"topic",    // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	
	// Additional exchanges could be declared here as needed
	
	return nil
}

// PublishToRabbitMQ publishes an event to RabbitMQ
func (p *RabbitMQPublisher) PublishToRabbitMQ(ctx context.Context, exchange string, routingKey string, event *models.Event) error {
	log.Printf("Publishing event %s to RabbitMQ exchange %s with routing key %s", event.ID, exchange, routingKey)
	
	// Serialize event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event %s: %v", event.ID, err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create message
	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		MessageId:    event.ID,
		Type:         event.Type,
		Body:         eventBytes,
		Headers: amqp.Table{
			"event_type": event.Type,
			"event_id":   event.ID,
			"timestamp":  event.Timestamp.Format(time.RFC3339),
		},
	}
	
	// Set expiration if configured
	if p.config.MessageTTLMs > 0 {
		msg.Expiration = fmt.Sprintf("%d", p.config.MessageTTLMs)
	}
	
	// Publish message
	err = p.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		msg,
	)
	
	if err != nil {
		log.Printf("Failed to publish event %s to RabbitMQ: %v", event.ID, err)
		return fmt.Errorf("failed to publish to RabbitMQ: %w", err)
	}
	
	log.Printf("Successfully published event %s to RabbitMQ", event.ID)
	return nil
}

// PublishToKafka is a no-op for RabbitMQPublisher
func (p *RabbitMQPublisher) PublishToKafka(ctx context.Context, topic string, key string, event *models.Event) error {
	return fmt.Errorf("kafka publishing not supported by rabbitmq publisher")
}

// Close closes the RabbitMQ connection and channel
func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	
	return nil
}