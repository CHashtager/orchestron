// config/config.go
package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration struct
type Config struct {
	API       APIConfig      `mapstructure:"api"`
	Database  DatabaseConfig `mapstructure:"database"`
	Broker    BrokerConfig   `mapstructure:"broker"`
	Routing   RoutingConfig  `mapstructure:"routing"`
	Outbox    OutboxConfig   `mapstructure:"outbox"`
	Metrics   MetricsConfig  `mapstructure:"metrics"`
}

// APIConfig contains API configuration
type APIConfig struct {
	Address        string        `mapstructure:"address"`
	Timeout        time.Duration `mapstructure:"timeout"`
	RateLimit      float64       `mapstructure:"rate_limit"`
	RateLimitBurst int           `mapstructure:"rate_limit_burst"`
	APIKeys        []string      `mapstructure:"api_keys"`
	MaxPayloadSize int64         `mapstructure:"max_payload_size"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Host                string        `mapstructure:"host"`
	Port                int           `mapstructure:"port"`
	User                string        `mapstructure:"user"`
	Password            string        `mapstructure:"password"`
	DBName              string        `mapstructure:"dbname"`
	SSLMode             string        `mapstructure:"sslmode"`
	MaxOpenConns        int           `mapstructure:"max_open_conns"`
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSecs int          `mapstructure:"conn_max_lifetime_secs"`
}

// BrokerConfig contains message broker configuration
type BrokerConfig struct {
	Type     string         `mapstructure:"type"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
}

// KafkaConfig contains Kafka configuration
type KafkaConfig struct {
	BootstrapServers      string `mapstructure:"bootstrap_servers"`
	ClientID             string `mapstructure:"client_id"`
	MaxRetries           int    `mapstructure:"max_retries"`
	RetryBackoffMs       int    `mapstructure:"retry_backoff_ms"`
	DeliveryTimeoutMs    int    `mapstructure:"delivery_timeout_ms"`
	QueueBufferingMaxMs  int    `mapstructure:"queue_buffering_max_ms"`
	QueueBufferingMaxKbytes int `mapstructure:"queue_buffering_max_kbytes"`
	FlushTimeoutMs       int    `mapstructure:"flush_timeout_ms"`
}

// RabbitMQConfig contains RabbitMQ configuration
type RabbitMQConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	Vhost          string `mapstructure:"vhost"`
	PrefetchCount  int    `mapstructure:"prefetch_count"`
	MessageTTLMs   int    `mapstructure:"message_ttl_ms"`
}

// RoutingConfig contains event routing configuration
type RoutingConfig struct {
	Defaults RouteConfig            `mapstructure:"defaults"`
	Routes   map[string]RouteConfig `mapstructure:"routes"`
}

// RouteConfig contains configuration for routing a specific event type
type RouteConfig struct {
	Broker     string         `mapstructure:"broker"`
	MaxRetries int            `mapstructure:"max_retries"`
	Kafka      KafkaRoute     `mapstructure:"kafka"`
	RabbitMQ   RabbitMQRoute  `mapstructure:"rabbitmq"`
}

// KafkaRoute represents a Kafka route configuration
type KafkaRoute struct {
	Topic string `mapstructure:"topic"`
}

// RabbitMQRoute represents a RabbitMQ route configuration
type RabbitMQRoute struct {
	Exchange   string `mapstructure:"exchange"`
	RoutingKey string `mapstructure:"routing_key"`
}

// OutboxConfig contains outbox configuration
type OutboxConfig struct {
	ProcessInterval  time.Duration `mapstructure:"process_interval"`
	BatchSize       int           `mapstructure:"batch_size"`
	MaxRetries      int           `mapstructure:"max_retries"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	RetentionPeriod time.Duration `mapstructure:"retention_period"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Port      int    `mapstructure:"port"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
	Subsystem string `mapstructure:"subsystem"`
}

// Load loads the configuration
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/orchestron")
	
	// Environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ORCHESTRON")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	// Set defaults
	setDefaults()
	
	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		// If the config file is not found, it's okay
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}
	
	// Override with environment variables
	overrideWithEnv()
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	
	// Post-process configuration
	postProcessConfig(&config)
	
	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// API defaults
	viper.SetDefault("api.address", ":8080")
	viper.SetDefault("api.timeout", "30s")
	viper.SetDefault("api.rate_limit", 100.0)
	viper.SetDefault("api.rate_limit_burst", 10)
	viper.SetDefault("api.max_payload_size", 1024*1024) // 1MB default
	
	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.dbname", "event_orchestrator")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 20)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime_secs", 300)
	
	// Broker defaults
	viper.SetDefault("broker.type", "kafka")
	
	// Kafka defaults
	viper.SetDefault("broker.kafka.bootstrap_servers", "localhost:9092")
	viper.SetDefault("broker.kafka.client_id", "event-orchestrator")
	viper.SetDefault("broker.kafka.max_retries", 3)
	viper.SetDefault("broker.kafka.retry_backoff_ms", 100)
	viper.SetDefault("broker.kafka.delivery_timeout_ms", 30000)
	viper.SetDefault("broker.kafka.queue_buffering_max_ms", 5)
	viper.SetDefault("broker.kafka.queue_buffering_max_kbytes", 1024)
	viper.SetDefault("broker.kafka.flush_timeout_ms", 1000)
	
	// RabbitMQ defaults
	viper.SetDefault("broker.rabbitmq.host", "localhost")
	viper.SetDefault("broker.rabbitmq.port", 5672)
	viper.SetDefault("broker.rabbitmq.user", "guest")
	viper.SetDefault("broker.rabbitmq.vhost", "/")
	viper.SetDefault("broker.rabbitmq.prefetch_count", 10)
	viper.SetDefault("broker.rabbitmq.message_ttl_ms", 86400000) // 24 hours
	
	// Routing defaults
	viper.SetDefault("routing.defaults.broker", "kafka")
	viper.SetDefault("routing.defaults.max_retries", 3)
	viper.SetDefault("routing.defaults.kafka.topic", "events")
	viper.SetDefault("routing.defaults.rabbitmq.exchange", "events")
	
	// Outbox defaults
	viper.SetDefault("outbox.process_interval", 5*time.Second)
	viper.SetDefault("outbox.batch_size", 100)
}

// overrideWithEnv overrides configuration with environment variables
func overrideWithEnv() {
	// Database password from environment
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		viper.Set("database.password", dbPass)
	}
	
	// RabbitMQ password from environment
	if rabbitPass := os.Getenv("RABBITMQ_PASSWORD"); rabbitPass != "" {
		viper.Set("broker.rabbitmq.password", rabbitPass)
	}
	
	// API key from environment
	if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		viper.Set("api.api_keys", []string{apiKey})
	}
}

// postProcessConfig performs post-processing on the configuration
func postProcessConfig(config *Config) {
	// Parse outbox process interval
	intervalStr := viper.GetString("outbox.process_interval")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		// Default to 5 seconds
		interval = 5 * time.Second
	}
	config.Outbox.ProcessInterval = interval
}
