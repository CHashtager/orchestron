package service

import (
	"github.com/chashtager/orchestron/internal/config"
)

// RoutingServiceImpl implements the RoutingService interface
type RoutingServiceImpl struct {
	config *config.RoutingConfig
}

// NewRoutingService creates a new routing service
func NewRoutingService(config *config.RoutingConfig) RoutingService {
	return &RoutingServiceImpl{
		config: config,
	}
}

// GetTopicForEventType returns the Kafka topic for an event type
func (s *RoutingServiceImpl) GetTopicForEventType(eventType string) string {
	// Look for specific configuration
	if route, ok := s.config.Routes[eventType]; ok && route.Kafka.Topic != "" {
		return route.Kafka.Topic
	}
	
	// Default to event type name or configured default
	if s.config.Defaults.Kafka.Topic != "" {
		return s.config.Defaults.Kafka.Topic
	}
	
	return eventType
}

// GetExchangeForEventType returns the RabbitMQ exchange for an event type
func (s *RoutingServiceImpl) GetExchangeForEventType(eventType string) string {
	// Look for specific configuration
	if route, ok := s.config.Routes[eventType]; ok && route.RabbitMQ.Exchange != "" {
		return route.RabbitMQ.Exchange
	}
	
	// Default to configured default
	if s.config.Defaults.RabbitMQ.Exchange != "" {
		return s.config.Defaults.RabbitMQ.Exchange
	}
	
	return "events"
}

// GetRoutingKeyForEventType returns the RabbitMQ routing key for an event type
func (s *RoutingServiceImpl) GetRoutingKeyForEventType(eventType string) string {
	// Look for specific configuration
	if route, ok := s.config.Routes[eventType]; ok && route.RabbitMQ.RoutingKey != "" {
		return route.RabbitMQ.RoutingKey
	}
	
	// Default to event type
	return eventType
}

// GetBrokerForEventType returns the broker type for an event type
func (s *RoutingServiceImpl) GetBrokerForEventType(eventType string) string {
	// Look for specific configuration
	if route, ok := s.config.Routes[eventType]; ok && route.Broker != "" {
		return route.Broker
	}
	
	// Default to configured default
	return s.config.Defaults.Broker
}

// GetMaxRetries returns the maximum number of retries for an event type
func (s *RoutingServiceImpl) GetMaxRetries(eventType string) int {
	// Look for specific configuration
	if route, ok := s.config.Routes[eventType]; ok && route.MaxRetries > 0 {
		return route.MaxRetries
	}
	
	// Default to configured default
	if s.config.Defaults.MaxRetries > 0 {
		return s.config.Defaults.MaxRetries
	}
	
	return 3
}
