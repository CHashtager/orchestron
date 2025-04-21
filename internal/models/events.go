package models

import (
	"time"
)

// Event represents a generic event in the system
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Metadata  map[string]string      `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// ValidationResult represents the result of event validation
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// EventSchema represents a schema for validating events
type EventSchema struct {
	ID           string    `json:"id" db:"id"`
	EventType    string    `json:"eventType" db:"event_type"`
	Version      string    `json:"version" db:"version"`
	SchemaFormat string    `json:"schemaFormat" db:"schema_format"`
	SchemaData   string    `json:"schemaData" db:"schema_data"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

// OutboxStatus represents the status of an outbox entry
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "PENDING"
	OutboxStatusPublished OutboxStatus = "PUBLISHED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

// OutboxEntry represents an entry in the outbox table
type OutboxEntry struct {
	ID          int64        `db:"id"`
	EventID     string       `db:"event_id"`
	EventType   string       `db:"event_type"`
	Payload     string       `db:"payload"`
	Status      OutboxStatus `db:"status"`
	RetryCount  int          `db:"retry_count"`
	CreatedAt   time.Time    `db:"created_at"`
	PublishedAt *time.Time   `db:"published_at"`
}

// DeadLetterEntry represents an entry in the dead letter queue
type DeadLetterEntry struct {
	ID               int64      `db:"id"`
	EventID          string     `db:"event_id"`
	EventType        string     `db:"event_type"`
	Payload          string     `db:"payload"`
	FailureReason    string     `db:"failure_reason"`
	CreatedAt        time.Time  `db:"created_at"`
	ProcessedAt      *time.Time `db:"processed_at"`
	ProcessingResult string     `db:"processing_result"`
}
