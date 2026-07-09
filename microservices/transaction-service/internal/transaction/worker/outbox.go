package worker

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxWorker struct {
	db          *sql.DB
	rabbitmqURL string
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
	mu          sync.Mutex
}

func NewOutboxWorker(db *sql.DB, rabbitmqURL string) *OutboxWorker {
	w := &OutboxWorker{
		db:          db,
		rabbitmqURL: rabbitmqURL,
	}

	// Connect on initialization to fail fast if config is wrong
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(nil, "Failed to initialize RabbitMQ connection", "error", err)
	}

	return w
}

func (w *OutboxWorker) ensureConnection() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.amqpConn == nil || w.amqpConn.IsClosed() {
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ...")
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

		// Declare main Exchange for wallet transactions
		err = ch.ExchangeDeclare(
			"wallet.events", // exchange name
			"topic",         // exchange type
			true,            // durable
			false,           // auto-deleted
			false,           // internal
			false,           // no-wait
			nil,             // arguments
		)
		if err != nil {
			w.channel.Close()
			w.channel = nil
			w.amqpConn.Close()
			w.amqpConn = nil
			return err
		}
		logger.Log.Info("Successfully connected to RabbitMQ and declared exchange!")
	}
	return nil
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Log.Info("Outbox Publisher Worker started...")

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			if w.channel != nil {
				w.channel.Close()
			}
			if w.amqpConn != nil {
				w.amqpConn.Close()
			}
			w.mu.Unlock()
			return
		case <-ticker.C:
			w.processPendingEvents(ctx)
		}
	}
}

func (w *OutboxWorker) processPendingEvents(ctx context.Context) {
	// Ensure connection before processing
	if err := w.ensureConnection(); err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ for pending events", "error", err.Error())
		return
	}

	// 1. Get oldest pending events
	query := `SELECT id, event_type, payload FROM outbox_events WHERE status = 'pending' ORDER BY created_at ASC LIMIT 20`
	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		logger.Error(ctx, "Failed to query pending outbox events", "error", err.Error())
		return
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload); err != nil {
			continue
		}
		events = append(events, e)
	}

	if len(events) == 0 {
		return
	}

	logger.Info(ctx, "Publishing pending outbox events", "count", len(events))

	// 2. Publish one by one to RabbitMQ
	for _, event := range events {
		w.mu.Lock()
		err = w.channel.PublishWithContext(
			ctx,
			"wallet.events", // exchange
			event.EventType, // routing key (e.g., "transfer.completed")
			false,           // mandatory
			false,           // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        []byte(event.Payload),
				MessageId:   event.ID,
			},
		)
		w.mu.Unlock()

		if err != nil {
			logger.Error(ctx, "Failed to publish event to RabbitMQ", "event_id", event.ID, "error", err.Error())
			
			// Close connection and channel to trigger reconnect next time
			w.mu.Lock()
			if w.channel != nil {
				_ = w.channel.Close()
				w.channel = nil
			}
			if w.amqpConn != nil {
				_ = w.amqpConn.Close()
				w.amqpConn = nil
			}
			w.mu.Unlock()

			// Increment attempts count in database
			_, _ = w.db.ExecContext(ctx, "UPDATE outbox_events SET attempts = attempts + 1 WHERE id = ?", event.ID)
			return // Return early to reconnect on next tick
		}

		// 3. Update status to processed if successfully sent to RabbitMQ
		_, err = w.db.ExecContext(ctx, "UPDATE outbox_events SET status = 'processed' WHERE id = ?", event.ID)
		if err != nil {
			logger.Error(ctx, "Failed to update outbox event status", "event_id", event.ID, "error", err.Error())
		}
	}
}
