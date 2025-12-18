package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/subhammahanty235/url-shortener/internal/domain"
	"github.com/subhammahanty235/url-shortener/internal/pkg/metrics"
)

type PostgresURLRepository struct {
	db      *sqlx.DB
	metrics *metrics.Metrics // Added for observability
}

func NewPostgresURLRepository(db *sqlx.DB, m *metrics.Metrics) *PostgresURLRepository {
	return &PostgresURLRepository{
		db:      db,
		metrics: m,
	}
}

func (r *PostgresURLRepository) Create(ctx context.Context, url *domain.URL) error {
	// Start timing the database operation
	// Learning: Always measure DB queries - they're often the bottleneck
	start := time.Now()
	operation := "create_url"

	// Defer metrics recording so it happens even if we return early
	defer func() {
		duration := time.Since(start).Seconds()
		r.metrics.DBQueryDuration.WithLabelValues(operation).Observe(duration)
	}()

	query := `
		INSERT INTO urls (short_code, original_url, user_id, expires_at, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	now := time.Now()
	url.CreatedAt = now
	url.UpdatedAt = now
	url.IsActive = true

	err := r.db.QueryRowContext(
		ctx,
		query,
		url.ShortURL,
		url.OriginalURL,
		url.UserID,
		url.ExpiresAt,
		url.IsActive,
		url.CreatedAt,
		url.UpdatedAt,
	).Scan(&url.ID)

	if err != nil {
		// Track database errors
		// Learning: Separate metric from duration - errors need alerting
		r.metrics.DBErrors.WithLabelValues(operation).Inc()
		return err
	}

	return nil
}

func (r *PostgresURLRepository) GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	// Start timing the database operation
	start := time.Now()
	operation := "get_by_short_code"

	// Defer metrics recording
	defer func() {
		duration := time.Since(start).Seconds()
		r.metrics.DBQueryDuration.WithLabelValues(operation).Observe(duration)
	}()

	query := `
	SELECT id, short_code, original_url, user_id, created_at, updated_at,
		   expires_at, click_count, is_active
	FROM urls
	WHERE short_code = $1 AND is_active = true`

	var url domain.URL
	err := r.db.GetContext(ctx, &url, query, shortCode)
	if err != nil {
		// Track database errors (including "not found")
		r.metrics.DBErrors.WithLabelValues(operation).Inc()
		return nil, err
	}

	if url.IsExpired() {
		// Track expired URLs separately
		// Learning: This is a business metric - helps understand user experience
		r.metrics.ExpiredURLsTotal.Inc()
		return nil, domain.ErrURLExpired // Fixed: was returning generic error
	}

	return &url, nil
}

// TODO: get short url by longurl for dedupliation
