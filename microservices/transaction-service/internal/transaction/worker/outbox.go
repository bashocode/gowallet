package worker

import (
	"context"
	"database/sql"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxWorker struct {
	db       *sql.DB
	amqpConn *amqp.Connection
	channel  *amqp.Channel
}

func NewOutboxWorker(db *sql.DB, conn *amqp.Connection) *OutboxWorker {
	ch, err := conn.Channel()
	if err != nil {
		logger.Fatal(nil, "Failed to open channel", "error", err)
	}

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
		logger.Fatal(nil, "Failed to declare exchange", "error", err)
	}

	return &OutboxWorker{
		db:       db,
		amqpConn: conn,
		channel:  ch,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Log.Info("Outbox Publisher Worker started...")

	for {
		select {
		case <-ctx.Done():
			w.channel.Close()
			return
		case <-ticker.C:
			w.processPendingEvents(ctx)
		}
	}
}

func (w *OutboxWorker) processPendingEvents(ctx context.Context) {
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

		if err != nil {
			logger.Error(ctx, "Failed to publish event to RabbitMQ", "event_id", event.ID, "error", err.Error())
			// Increment attempts count in database
			_, _ = w.db.ExecContext(ctx, "UPDATE outbox_events SET attempts = attempts + 1 WHERE id = ?", event.ID)
			continue
		}

		// 3. Update status to processed if successfully sent to RabbitMQ
		_, err = w.db.ExecContext(ctx, "UPDATE outbox_events SET status = 'processed' WHERE id = ?", event.ID)
		if err != nil {
			logger.Error(ctx, "Failed to update outbox event status", "event_id", event.ID, "error", err.Error())
		}
	}
}
