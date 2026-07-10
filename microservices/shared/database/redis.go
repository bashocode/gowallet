package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(addr string) (*redis.Client, error) {
	maxRetries := 5
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

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

	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, lastErr)
}
