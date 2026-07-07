package dlq

import (
	"context"
	"log/slog"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

type NoOpPublisher struct{}

func NewNoOpPublisher() *NoOpPublisher {
	return &NoOpPublisher{}
}

func (p *NoOpPublisher) Publish(ctx context.Context, topic string, payload map[string]string) error {
	logger.Log.Error("DLQ event",
		slog.String("topic", topic),
		slog.Any("payload", payload),
	)
	return nil
}
