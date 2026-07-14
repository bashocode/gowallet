package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongoDB(url string) (*mongo.Client, error) {
	const (
		maxRetries     = 5
		maxBackoff     = 30 * time.Second
		initialBackoff = 1 * time.Second
	)

	backoff := initialBackoff
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
		if err == nil {
			err = client.Ping(ctx, nil)
			if err == nil {
				cancel()
				logger.Log.Info("Successfully connected to MongoDB!", "attempt", attempt)
				return client, nil
			}
			client.Disconnect(ctx)
		}
		cancel()
		lastErr = err

		if attempt < maxRetries {
			logger.Log.Warn("Failed to connect to MongoDB, retrying...",
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

	return nil, fmt.Errorf("failed to connect to MongoDB after %d attempts: %w", maxRetries, lastErr)
}
