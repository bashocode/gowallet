package worker

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

// TransferOutboxWorker polls transfer_outbox_events for pending rows and
// publishes them to the transfer.events RabbitMQ exchange (Episode 35).
type TransferOutboxWorker struct {
	db          *sql.DB
	rabbitmqURL string
	outboxRepo  repository.TransferOutboxRepository
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
}

func NewTransferOutboxWorker(db *sql.DB, rabbitmqURL string, outboxRepo repository.TransferOutboxRepository) *TransferOutboxWorker {
	w := &TransferOutboxWorker{
		db:          db,
		rabbitmqURL: rabbitmqURL,
		outboxRepo:  outboxRepo,
	}
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(nil, "Failed to initialize RabbitMQ connection for transfer outbox", "error", err)
	}
	return w
}

func (w *TransferOutboxWorker) ensureConnection() error {
	if w.amqpConn != nil && !w.amqpConn.IsClosed() {
		return nil
	}

	const (
		maxRetries     = 5
		maxBackoff     = 30 * time.Second
		initialBackoff = 1 * time.Second
	)

	backoff := initialBackoff
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ (transfer outbox)...", "attempt", attempt)
		conn, err := amqp.Dial(w.rabbitmqURL)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				logger.Log.Warn("Failed to dial RabbitMQ, retrying...",
					"attempt", attempt, "backoff", backoff.String(), "error", err.Error())
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		if err := ch.ExchangeDeclare(
			"transfer.events",
			"topic",
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		w.amqpConn = conn
		w.channel = ch
		logger.Log.Info("Successfully connected to RabbitMQ and declared transfer.events exchange!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (w *TransferOutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	logger.Log.Info("Transfer Outbox Publisher Worker started...")

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

func (w *TransferOutboxWorker) processPendingEvents(ctx context.Context) {
	if err := w.ensureConnection(); err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ for transfer outbox", "error", err.Error())
		return
	}

	events, err := w.outboxRepo.FetchPending(ctx, 50)
	if err != nil {
		logger.Error(ctx, "Failed to fetch pending transfer outbox events", "error", err.Error())
		return
	}
	if len(events) == 0 {
		return
	}

	logger.Info(ctx, "Publishing pending transfer outbox events", "count", len(events))

	for _, event := range events {
		err := w.channel.PublishWithContext(
			ctx,
			"transfer.events",
			event.EventType,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				MessageId:    event.ID,
				Type:         event.EventType,
				Body:         []byte(event.Payload),
			},
		)
		if err != nil {
			logger.Error(ctx, "Failed to publish transfer event to RabbitMQ", "event_id", event.ID, "error", err.Error())
			_ = w.outboxRepo.IncrementAttempts(ctx, event.ID, err.Error())
			if w.channel != nil {
				_ = w.channel.Close()
				w.channel = nil
			}
			if w.amqpConn != nil {
				_ = w.amqpConn.Close()
				w.amqpConn = nil
			}
			return
		}

		if err := w.outboxRepo.MarkProcessed(ctx, event.ID); err != nil {
			logger.Error(ctx, "Failed to mark transfer outbox event processed", "event_id", event.ID, "error", err.Error())
		}
	}
}
