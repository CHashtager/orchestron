package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/streadway/amqp"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Vhost    string
	Exchange string
	Queue    string
}

func main() {
	// Load configuration
	cfg := Config{
		Host:     "localhost",
		Port:     5672,
		User:     "guest",
		Password: "guest",
		Vhost:    "/",
		Exchange: "events",
		Queue:    "test-queue",
	}

	// Create connection string
	connStr := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Vhost,
	)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(connStr)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare exchange
	err = ch.ExchangeDeclare(
		cfg.Exchange, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare exchange: %v", err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		cfg.Queue, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// Bind queue to exchange with routing key pattern
	err = ch.QueueBind(
		q.Name,       // queue name
		"#",          // routing key (match all)
		cfg.Exchange, // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to bind queue: %v", err)
	}

	// Set QoS
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	// Consume messages
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// Create context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	log.Printf("Starting to consume messages from exchange '%s' with routing key pattern '#'", cfg.Exchange)
	
	for {
		select {
		case msg := <-msgs:
			// Process message
			log.Printf("Received message:")
			log.Printf("  Exchange: %s", msg.Exchange)
			log.Printf("  Routing Key: %s", msg.RoutingKey)
			log.Printf("  Message ID: %s", msg.MessageId)
			log.Printf("  Timestamp: %s", msg.Timestamp)
			log.Printf("  Headers: %v", msg.Headers)
			log.Printf("  Body: %s", string(msg.Body))

			// Acknowledge message
			if err := msg.Ack(false); err != nil {
				log.Printf("Error acknowledging message: %v", err)
			}

		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			cancel()
			return
		}
	}
} 