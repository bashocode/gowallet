package worker

import (
	"context"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type NotificationOutboxWorker struct {
	outboxRepo  repository.NotificationOutboxRepository
	rabbitmqURL string
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
}

func NewNotificationOutboxWorker(outboxRepo repository.NotificationOutboxRepository, rabbitmqURL string) *NotificationOutboxWorker {
	w := &NotificationOutboxWorker{
		outboxRepo:  outboxRepo,
		rabbitmqURL: rabbitmqURL,
	}

	// Connect on initialization to fail fast if config is wrong
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(nil, "Failed to initialize RabbitMQ connection for notification outbox", "error", err)
	}

	return w
}

func (w *NotificationOutboxWorker) ensureConnection() error {
	if w.amqpConn == nil || w.amqpConn.IsClosed() {
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ for notification outbox...")
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

		// Declare exchange for notification events
		err = ch.ExchangeDeclare(
			"notification.events", // exchange name
			"topic",               // exchange type
			true,                  // durable
			false,                 // auto-deleted
			false,                 // internal
			false,                 // no-wait
			nil,                   // arguments
		)
		if err != nil {
			w.channel.Close()
			w.channel = nil
			w.amqpConn.Close()
			w.amqpConn = nil
			return err
		}
		logger.Log.Info("Successfully connected to RabbitMQ and declared notification.events exchange!")
	}
	return nil
}

func (w *NotificationOutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Log.Info("Notification Outbox Worker started...")

	for {
		select {
		case <-ctx.Done():
			if w.channel != nil {
				w.channel.Close()
			}
			if w.amqpConn != nil {
				w.amqpConn.Close()
			}
			return
		case <-ticker.C:
			w.processPendingEvents(ctx)
		}
	}
}

func (w *NotificationOutboxWorker) processPendingEvents(ctx context.Context) {
	// Ensure connection before processing
	if err := w.ensureConnection(); err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ for notification pending events", "error", err.Error())
		return
	}

	events, err := w.outboxRepo.GetPendingEvents(ctx, 50)
	if err != nil {
		logger.Error(ctx, "Failed to read notification outbox", "error", err.Error())
		return
	}

	if len(events) == 0 {
		return
	}

	logger.Info(ctx, "Publishing pending notification outbox events", "count", len(events))

	for _, event := range events {
		err := w.channel.PublishWithContext(
			ctx,
			"notification.events", // exchange
			event.EventType,       // routing key (e.g., "notification.send_email")
			false,                 // mandatory
			false,                 // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				MessageId:    event.ID,
				Type:         event.EventType,
				Body:         event.Payload,
			},
		)

		if err != nil {
			logger.Error(ctx, "Failed to publish notification outbox event", "event_id", event.ID, "error", err.Error())

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
				logger.Error(ctx, "Failed to increment notification outbox attempts", "event_id", event.ID, "error", updateErr.Error())
			}
			return // Return early to reconnect on next tick
		}

		if err := w.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			logger.Error(ctx, "Failed to mark notification outbox event as processed", "event_id", event.ID, "error", err.Error())
			continue
		}

		logger.Info(ctx, "Published notification outbox event", "event_id", event.ID, "event_type", event.EventType)
	}
}
