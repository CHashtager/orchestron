CREATE TABLE event_schemas (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    schema_format VARCHAR(50) NOT NULL,
    schema_data TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(event_type, version)
);

CREATE TABLE event_outbox (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    published_at TIMESTAMP
);

CREATE INDEX idx_status ON event_outbox (status);
CREATE INDEX idx_created_at ON event_outbox (created_at);

CREATE TABLE dead_letter_queue (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload TEXT NOT NULL,
    failure_reason TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    processed_at TIMESTAMP,
    processing_result TEXT
);

CREATE INDEX idx_created_at ON dead_letter_queue (created_at);