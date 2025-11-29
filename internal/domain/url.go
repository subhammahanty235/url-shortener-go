package domain

import (
	"errors"
	"time"
)

// common errors
var (
	ErrURLNotFound       = errors.New("url not found")
	ErrURLExpired        = errors.New("url has expired")
	ErrInvalidURL        = errors.New("invalid url format")
	ErrShortCodeExists   = errors.New("short code already exists")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrInvalidShortCode  = errors.New("invalid short code")
)

type URL struct {
	ID          int64      `json:"id"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	UserID      *string    `json:"user_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	ClickCount  int64      `json:"click_count" db:"click_count"`
	IsActive    bool       `json:"is_active" db:"is_active"`
}

func (u *URL) isExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

type CreateURLRequest struct {
	OriginalURL string  `json:"original_url" binding:"required,url"`
	CustomAlias *string `json:"custom_alias,omitempty"`
	ExpiresIn   *int64  `json:"expires_in,omitempty"`
	UserID      *string `json:"user_id,omitempty"`
}

type CreateURLResponse struct {
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
type URLStats struct {
	ShortCode   string    `json:"short_code"`
	ClickCount  int64     `json:"click_count"`
	LastClicked *time.Time `json:"last_clicked,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type ClickEvent struct {
	ID        int64     `json:"id" db:"id"`
	ShortCode string    `json:"short_code" db:"short_code"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	Referrer  string    `json:"referrer" db:"referrer"`
	Country   string    `json:"country" db:"country"`
	City      string    `json:"city" db:"city"`
	Device    string    `json:"device" db:"device"`
	Browser   string    `json:"browser" db:"browser"`
	OS        string    `json:"os" db:"os"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type UrlRepository interface {

}

type CacheRepository interface {
	
}
