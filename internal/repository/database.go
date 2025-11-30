package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/subhammahanty235/url-shortener/internal/config"
	"go.uber.org/zap"
)

// NewPostgresConnection creates a new PostgreSQL connection
func NewPostgresConnection(cfg config.DatabaseConfig, logger *zap.Logger) (*sqlx.DB, error) {
	dsn := cfg.DSN()

	logger.Info("connecting to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("connected to PostgreSQL successfully")
	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sqlx.DB, logger *zap.Logger) error {
	logger.Info("running database migrations")

	migrations := []string{
		// URLs table
		`CREATE TABLE IF NOT EXISTS urls (
			id BIGSERIAL PRIMARY KEY,
			short_code VARCHAR(20) NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			user_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE,
			click_count BIGINT NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT true
		)`,

		// Index on short_code for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code) WHERE is_active = true`,

		// Index on original_url for deduplication
		`CREATE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url) WHERE is_active = true`,

		// Index on user_id for user queries
		`CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id) WHERE user_id IS NOT NULL AND is_active = true`,

		// Index on expires_at for cleanup jobs
		`CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls(expires_at) WHERE expires_at IS NOT NULL`,

		// Index on created_at for sorting
		`CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls(created_at DESC)`,

		// Click events table for analytics
		`CREATE TABLE IF NOT EXISTS click_events (
			id BIGSERIAL PRIMARY KEY,
			short_code VARCHAR(20) NOT NULL,
			ip_address VARCHAR(45),
			user_agent TEXT,
			referrer TEXT,
			country VARCHAR(2),
			city VARCHAR(100),
			device VARCHAR(20),
			browser VARCHAR(50),
			os VARCHAR(50),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)`,

		// Index on short_code for analytics queries
		`CREATE INDEX IF NOT EXISTS idx_click_events_short_code ON click_events(short_code)`,

		// Index on created_at for time-based queries
		`CREATE INDEX IF NOT EXISTS idx_click_events_created_at ON click_events(created_at DESC)`,

		// Composite index for common analytics queries
		`CREATE INDEX IF NOT EXISTS idx_click_events_short_code_created ON click_events(short_code, created_at DESC)`,

		// Partitioning setup for click_events (for large scale)
		// Note: In production, you'd use pg_partman or similar for automatic partition management
		// This is a simplified example
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", i+1, err)
		}
	}

	logger.Info("database migrations completed successfully")
	return nil
}

// Close closes the database connection
func Close(db *sqlx.DB, logger *zap.Logger) {
	if db != nil {
		logger.Info("closing database connection")
		db.Close()
	}
}
