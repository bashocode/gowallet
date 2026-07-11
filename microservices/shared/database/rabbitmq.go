package database

import (
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ConnectRabbitMQ dials RabbitMQ with exponential backoff retries so the caller
// does not crash on a broker that is still starting up during `docker compose up`.
func ConnectRabbitMQ(url string) (*amqp.Connection, error) {
	const (
		maxRetries     = 5
		maxBackoff     = 30 * time.Second
		initialBackoff = 1 * time.Second
	)

	backoff := initialBackoff
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			logger.Log.Info("Successfully connected to RabbitMQ!", "attempt", attempt)
			return conn, nil
		}
		lastErr = err

		if attempt < maxRetries {
			logger.Log.Warn("Failed to connect to RabbitMQ, retrying...",
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

	return nil, fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}
