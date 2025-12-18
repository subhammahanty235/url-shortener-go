package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subhammahanty235/url-shortener/internal/domain"
	"github.com/subhammahanty235/url-shortener/internal/pkg/metrics"
)

const (
	urlCachePrefix = "url:"
	rateLimitCache = "rl:"
)

type RedisCacheRepository struct {
	client     *redis.Client
	defaultTTL time.Duration
	metrics    *metrics.Metrics
}

func NewRedisCacheRepository(client *redis.Client, defaultTTL time.Duration, m *metrics.Metrics) *RedisCacheRepository {
	return &RedisCacheRepository{
		client:     client,
		defaultTTL: defaultTTL,
		metrics:    m,
	}
}

func (r *RedisCacheRepository) Get(ctx context.Context, shortCode string) (*domain.URL, error) {
	key := urlCachePrefix + shortCode
	operation := "get"

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Cache miss - key doesn't exist
			// This is NOT an error, it's expected behavior
			// Learning: Cache miss means we need to query the database
			r.metrics.CacheMissesTotal.WithLabelValues(operation).Inc()
			return nil, nil
		}

		// Actual error (connection failure, timeout, etc.)
		// Learning: Track errors separately from misses
		r.metrics.CacheErrors.WithLabelValues(operation).Inc()
		return nil, err
	}

	var url domain.URL
	if err := json.Unmarshal(data, &url); err != nil {
		// Deserialization error - data is corrupted
		r.metrics.CacheErrors.WithLabelValues(operation).Inc()
		return nil, err
	}

	// Cache hit - found the data in Redis!
	// Learning: High hit ratio = cache is working well
	// Low hit ratio = maybe TTL is too short or cache is too small
	r.metrics.CacheHitsTotal.WithLabelValues(operation).Inc()
	return &url, nil
}

func (r *RedisCacheRepository) Set(ctx context.Context, url *domain.URL, ttl time.Duration) error {
	if ttl == 0 {
		ttl = r.defaultTTL
	}

	key := urlCachePrefix + url.ShortURL
	data, err := json.Marshal(url)
	if err != nil {
		// Serialization error
		r.metrics.CacheErrors.WithLabelValues("set").Inc()
		return err // Fixed: was returning nil, should return err
	}

	err = r.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		// Redis write error
		r.metrics.CacheErrors.WithLabelValues("set").Inc()
		return err
	}

	// Successfully cached
	return nil
}

func (r *RedisCacheRepository) Delete(ctx context.Context, shortCode string) error {
	key := urlCachePrefix + shortCode
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCacheRepository) Exists(ctx context.Context, shortCode string) (bool, error) {
	key := urlCachePrefix + shortCode
	result, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
