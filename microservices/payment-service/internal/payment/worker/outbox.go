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
	outboxRepo  repository.OutboxRepository
	rabbitmqURL string
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
	stopCh      chan struct{}
}

func NewOutboxWorker(outboxRepo repository.OutboxRepository, rabbitmqURL string) (*OutboxWorker, error) {
	w := &OutboxWorker{
		outboxRepo:  outboxRepo,
		rabbitmqURL: rabbitmqURL,
		stopCh:      make(chan struct{}),
	}

	if err := w.ensureConnection(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *OutboxWorker) ensureConnection() error {
	if w.amqpConn == nil || w.amqpConn.IsClosed() {
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ for payment outbox...")
		conn, err := amqp.Dial(w.rabbitmqURL)
		if err != nil {
			return err
		}
		w.amqpConn = conn

		ch, err := conn.Channel()
		if err != nil {
			w.amqpConn.Close()
			w.amqpConn = nil
			return err
		}
		w.channel = ch

		if err := ch.ExchangeDeclare(
			"payment.events",
			"topic",
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			w.channel.Close()
			w.channel = nil
			w.amqpConn.Close()
			w.amqpConn = nil
			return err
		}
		logger.Log.Info("Successfully connected to RabbitMQ and declared payment exchange!")
	}
	return nil
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	defer func() {
		if w.channel != nil {
			w.channel.Close()
		}
		if w.amqpConn != nil {
			w.amqpConn.Close()
		}
	}()

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
	if err := w.ensureConnection(); err != nil {
		logger.Log.Error("Failed to connect to RabbitMQ for pending events", slog.Any("error", err))
		return
	}

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
			
			// Close connection and channel to trigger reconnect next time
			if w.channel != nil {
				_ = w.channel.Close()
				w.channel = nil
			}
			if w.amqpConn != nil {
				_ = w.amqpConn.Close()
				w.amqpConn = nil
			}

			if updateErr := w.outboxRepo.IncrementAttempts(ctx, event.ID, err.Error()); updateErr != nil {
				logger.Log.Error("Failed to increment attempts", "event_id", event.ID, slog.Any("error", updateErr))
			}
			return // Return early to reconnect on next tick
		}

		if err := w.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			logger.Log.Error("Failed to mark event as processed", "event_id", event.ID, slog.Any("error", err))
			continue
		}

		logger.Log.Info("Published outbox event", "event_id", event.ID, "event_type", event.EventType)
	}
}
