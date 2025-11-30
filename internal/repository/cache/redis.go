package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subhammahanty235/url-shortener/internal/config"
	"go.uber.org/zap"
)

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg config.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	logger.Info("connecting to Redis",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.Int("db", cfg.DB),
	)

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	logger.Info("connected to Redis successfully")
	return client, nil
}

// NewRedisClusterClient creates a new Redis Cluster client (for production)
func NewRedisClusterClient(addrs []string, password string, logger *zap.Logger) (*redis.ClusterClient, error) {
	logger.Info("connecting to Redis Cluster",
		zap.Strings("addresses", addrs),
	)

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        addrs,
		Password:     password,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis Cluster: %w", err)
	}

	logger.Info("connected to Redis Cluster successfully")
	return client, nil
}

// Close closes the Redis client
func Close(client *redis.Client, logger *zap.Logger) {
	if client != nil {
		logger.Info("closing Redis connection")
		client.Close()
	}
}

// CloseCluster closes the Redis Cluster client
func CloseCluster(client *redis.ClusterClient, logger *zap.Logger) {
	if client != nil {
		logger.Info("closing Redis Cluster connection")
		client.Close()
	}
}