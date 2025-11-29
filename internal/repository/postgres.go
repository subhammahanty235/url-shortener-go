package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/subhammahanty235/url-shortener/internal/domain"
)

type PostgresURLRepository struct {
	db *sqlx.DB
}

func NewPostgresURLRepository(db *sqlx.DB) *PostgresURLRepository {
	return &PostgresURLRepository{db: db}
}

func (r *PostgresURLRepository) Create(ctx context.Context, url *domain.URL) error {
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
		return err
	}

	return nil
}

func (r *PostgresURLRepository) GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	query := `
	SELECT id, short_code, original_url, user_id, created_at, updated_at, 
		   expires_at, click_count, is_active
	FROM urls
	WHERE short_code = $1 AND is_active = true`

	var url domain.URL
	err := r.db.GetContext(ctx, &url, query, shortCode)
	if err != nil {
		return nil, err
	}

	if url.IsExpired() {
		return nil, errors.New("URL expired")
	}

	return &url, nil
}

// TODO: get short url by longurl for dedupliation
