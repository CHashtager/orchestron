package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chashtager/orchestron/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sethvargo/go-retry"
)

// Database represents a database connection
type Database struct {
	db *sqlx.DB
}

// NewDatabase creates a new database connection with retry logic
func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	var db *sqlx.DB
	var err error

	// Retry logic with exponential backoff
	backoff := retry.NewExponential(1 * time.Second)
	ctx := context.Background()

	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		db, err = sqlx.Connect("postgres", dsn)
		if err != nil {
			log.Printf("Failed to connect to database, retrying: %v", err)
			return retry.RetryableError(err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSecs) * time.Second)

	// Start health check goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := db.Ping(); err != nil {
				log.Printf("Database health check failed: %v", err)
			}
		}
	}()

	return &Database{db: db}, nil
}

// GetDB returns the database connection
func (d *Database) GetDB() *sqlx.DB {
	return d.db
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// HealthCheck performs a database health check
func (d *Database) HealthCheck() error {
	return d.db.Ping()
}

// WithTransaction executes a function within a transaction
func (d *Database) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction rollback failed: %v, original error: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}
