package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subhammahanty235/url-shortener/internal/domain"
)

const (
	urlCachePrefix = "url:"
	rateLimitCache = "rl:"
)

type RedisCacheRepository struct{
	client *redis.Client
	defaultTTL time.Duration
}
func NewRedisCacheRepository(client *redis.Client, defaultTTL time.Duration) *RedisCacheRepository{
	return &RedisCacheRepository{
		client: client,
		defaultTTL: defaultTTL,
	}
}

func (r *RedisCacheRepository) Get(ctx context.Context, shortCode string) (*domain.URL, error){
	key := urlCachePrefix+shortCode

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil){
			return nil, nil
		}

		return nil, err
	}

	var url domain.URL
	if err := json.Unmarshal(data, &url); err != nil {
		return nil, err
	}

	return &url, nil
}

func (r *RedisCacheRepository) Set(ctx context.Context, url *domain.URL, ttl time.Duration) error {
	if ttl == 0 {
		ttl = r.defaultTTL
	}

	key := urlCachePrefix + url.ShortURL
	data, err := json.Marshal(url)
	if err != nil {
		return nil
	}
	return r.client.Set(ctx, key, data, ttl).Err()

}


