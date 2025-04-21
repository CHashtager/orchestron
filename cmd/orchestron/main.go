package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chashtager/orchestron/internal/api"
	"github.com/chashtager/orchestron/internal/broker"
	"github.com/chashtager/orchestron/internal/config"
	"github.com/chashtager/orchestron/internal/processor"
	"github.com/chashtager/orchestron/internal/repository"
	"github.com/chashtager/orchestron/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize repositories
	db, err := repository.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	outboxRepo := repository.NewOutboxRepository(db)
	dlqRepo := repository.NewDeadLetterRepository(db)
	schemaRepo := repository.NewSchemaRepository(db)

	// Initialize message broker client
	var msgBroker broker.EventPublisher
	switch cfg.Broker.Type {
	case "kafka":
		msgBroker, err = broker.NewKafkaPublisher(&cfg.Broker.Kafka)
	case "rabbitmq":
		msgBroker, err = broker.NewRabbitMQPublisher(&cfg.Broker.RabbitMQ)
	default:
		log.Fatalf("Unsupported broker type: %s", cfg.Broker.Type)
	}
	if err != nil {
		log.Fatalf("Failed to initialize message broker: %v", err)
	}
	defer msgBroker.Close()

	// Initialize services
	eventValidator := service.NewEventValidator(schemaRepo)
	routingService := service.NewRoutingService(&cfg.Routing)
	dlqService := service.NewDeadLetterService(dlqRepo)
	
	// Initialize event publisher with outbox pattern
	eventPublisher := service.NewEventPublisherWithOutbox(
		msgBroker,
		outboxRepo,
		routingService,
	)
	// Initialize schema provider
	schemaProvider := service.NewSchemaProvider(schemaRepo)

	// Initialize event processor
	eventProcessor := processor.NewEventProcessor(
		eventValidator,
		eventPublisher,
		dlqService,
		schemaProvider,
		cfg.API.MaxPayloadSize,
		cfg.Outbox.RetryBackoff,
	)

	// Initialize outbox processor
	outboxProcessor := processor.NewOutboxProcessor(
		outboxRepo,
		msgBroker,
		routingService,
		cfg.Outbox.ProcessInterval,
		cfg.Outbox.BatchSize,
	)

	// Start outbox processor in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go outboxProcessor.Start(ctx)

	// Initialize HTTP server
	httpServer := api.NewServer(&cfg.API, eventProcessor, schemaRepo)
	server := &http.Server{
		Addr:    cfg.API.Address,
		Handler: httpServer.Router(),
	}

	// Start HTTP server in goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.API.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	// Cancel outbox processor
	cancel()

	log.Println("Server gracefully stopped")
}