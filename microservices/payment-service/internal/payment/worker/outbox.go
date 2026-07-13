package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxWorker struct {
	outboxRepo repository.OutboxRepository
	channel    *amqp.Channel
	stopCh     chan struct{}
}

func NewOutboxWorker(outboxRepo repository.OutboxRepository, rabbitmqURL string) (*OutboxWorker, error) {
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	if err := ch.ExchangeDeclare(
		"payment.events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	return &OutboxWorker{
		outboxRepo: outboxRepo,
		channel:    ch,
		stopCh:     make(chan struct{}),
	}, nil
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	logger.Log.Info("Outbox worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("Outbox worker stopping")
			return
		case <-w.stopCh:
			logger.Log.Info("Outbox worker stopped")
			return
		case <-ticker.C:
			w.processPendingEvents(ctx)
		}
	}
}

func (w *OutboxWorker) Stop() {
	close(w.stopCh)
}

func (w *OutboxWorker) processPendingEvents(ctx context.Context) {
	events, err := w.outboxRepo.GetPendingEvents(ctx, 50)
	if err != nil {
		logger.Log.Error("Failed to read payment outbox", slog.Any("error", err))
		return
	}

	for _, event := range events {
		err := w.channel.PublishWithContext(ctx,
			"payment.events",
			event.EventType,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				MessageId:    event.ID,
				Type:         event.EventType,
				Body:         event.Payload,
			},
		)

		if err != nil {
			logger.Log.Error("Failed to publish outbox event", "event_id", event.ID, slog.Any("error", err))
			if updateErr := w.outboxRepo.IncrementAttempts(ctx, event.ID, err.Error()); updateErr != nil {
				logger.Log.Error("Failed to increment attempts", "event_id", event.ID, slog.Any("error", updateErr))
			}
			continue
		}

		if err := w.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			logger.Log.Error("Failed to mark event as processed", "event_id", event.ID, slog.Any("error", err))
			continue
		}

		logger.Log.Info("Published outbox event", "event_id", event.ID, "event_type", event.EventType)
	}
}
