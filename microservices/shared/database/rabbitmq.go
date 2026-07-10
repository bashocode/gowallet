package database

import (
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

func ConnectRabbitMQ(url string) (*amqp.Connection, error) {
	maxRetries := 5
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	var conn *amqp.Connection
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		conn, lastErr = amqp.Dial(url)
		if lastErr == nil {
			logger.Log.Info("Successfully connected to RabbitMQ!", "attempt", attempt)
			return conn, nil
		}

		if attempt < maxRetries {
			logger.Log.Warn("Failed to connect to RabbitMQ, retrying...",
				"attempt", attempt,
				"max_retries", maxRetries,
				"backoff", backoff.String(),
				"error", lastErr.Error(),
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
