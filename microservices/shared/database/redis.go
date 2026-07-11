package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/redis/go-redis/v9"
)

// ConnectRedis creates a Redis client and pings with exponential backoff
// retries so the caller survives a Redis instance that is still starting.
func ConnectRedis(addr string) (*redis.Client, error) {
	const (
		maxRetries     = 5
		maxBackoff     = 30 * time.Second
		initialBackoff = 1 * time.Second
	)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	backoff := initialBackoff
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := rdb.Ping(ctx).Result()
		cancel()
		if err == nil {
			logger.Log.Info("Successfully connected to Redis!", "attempt", attempt)
			return rdb, nil
		}
		lastErr = err

		if attempt < maxRetries {
			logger.Log.Warn("Failed to connect to Redis, retrying...",
				"attempt", attempt,
				"max_retries", maxRetries,
				"backoff", backoff.String(),
				"error", err.Error(),
			)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	_ = rdb.Close()
	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, lastErr)
}
