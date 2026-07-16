package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/audit-service/internal/model"
	"github.com/bashocode/gowallet/microservices/audit-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	rabbitresilience "github.com/bashocode/gowallet/microservices/shared/rabbitresilience"
	amqp "github.com/rabbitmq/amqp091-go"
)

type AuditConsumer struct {
	rabbitmqURL string
	auditRepo   *repository.AuditRepository
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
	confirms    chan amqp.Confirmation
}

func NewAuditConsumer(rabbitmqURL string, repo *repository.AuditRepository) *AuditConsumer {
	c := &AuditConsumer{
		rabbitmqURL: rabbitmqURL,
		auditRepo:   repo,
	}
	if err := c.ensureConnection(); err != nil {
		logger.Fatal(context.Background(), "failed to initialize RabbitMQ connection for audit consumer", "error", err)
	}
	return c
}

func (c *AuditConsumer) ensureConnection() error {
	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
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
		logger.Log.Info("connecting/reconnecting to RabbitMQ (audit consumer)...", "attempt", attempt)
		conn, err := amqp.Dial(c.rabbitmqURL)
		if err != nil {
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
			"payment.events",
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

		if err := ch.ExchangeDeclare(
			"wallet.events",
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

		if err := rabbitresilience.Declare(ch, rabbitresilience.QueueConfig{MainQueue: "audit.payment_events", RetryQueue: "audit.payment_events.retry", DLQ: "audit.payment_events.dlq", DLX: "audit.dlx", MainExchange: "payment.events", RoutingKey: "payment.#", RetryTTL: 10000}); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}
		if err := ch.QueueBind("audit.payment_events.retry", "#", "audit.dlx.retry", false, nil); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}
		queue, err := ch.QueueDeclarePassive("audit.payment_events", true, false, false, false, nil)
		if err != nil {
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

		// Bind to payment events
		if err := ch.QueueBind(
			queue.Name,
			"payment.#",
			"payment.events",
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

		// Bind to wallet events
		if err := ch.QueueBind(
			queue.Name,
			"#",
			"wallet.events",
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

		// Bind to transfer events
		if err := ch.QueueBind(
			queue.Name,
			"transfer.#",
			"transfer.events",
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

		if err := ch.Confirm(false); err != nil {
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

		c.amqpConn = conn
		c.channel = ch
		c.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
		logger.Log.Info("successfully connected to RabbitMQ and bound exchanges to audit queue!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (c *AuditConsumer) Start(ctx context.Context) {
	logger.Log.Info("audit consumer started...")

	for {
		if err := c.ensureConnection(); err != nil {
			logger.Error(ctx, "failed to connect to RabbitMQ for audit consumer", "error", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := c.channel.Consume(
			"audit.payment_events",
			"audit-consumer",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error(ctx, "failed to consume from audit.payment_events", "error", err.Error())
			c.cleanupConnection()
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Log.Info("consuming payment events from audit queue...")

		for {
			select {
			case <-ctx.Done():
				c.cleanupConnection()
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Log.Warn("audit consumer channel closed, reconnecting...")
					c.cleanupConnection()
					time.Sleep(2 * time.Second)
					goto reconnect
				}
				c.processMessage(ctx, msg)
			}
		}

	reconnect:
	}
}

func (c *AuditConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	var payload map[string]any
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		logger.Error(ctx, "invalid payment event payload", "error", err)
		c.deadLetter(ctx, msg, err)
		return
	}

	eventID, _ := payload["event_id"].(string)
	if eventID == "" {
		c.deadLetter(ctx, msg, fmt.Errorf("invalid event: event_id is required"))
		return
	}

	source := "payment-service"
	switch msg.Exchange {
	case "wallet.events":
		source = "transaction-service"
	case "transfer.events":
		source = "transaction-service"
	case "payment.events":
		source = "payment-service"
	}

	auditLog := model.AuditLog{
		ID:          eventID,
		EventType:   msg.RoutingKey,
		MessageID:   msg.MessageId,
		Source:      source,
		Payload:     payload,
		ReceivedAt:  time.Now().UTC(),
		ProcessedAt: time.Now().UTC(),
	}

	if err := c.auditRepo.SaveAuditLog(ctx, auditLog); err != nil {
		logger.Error(ctx, "failed to save audit log", "error", err, "event_id", eventID)
		c.retry(ctx, msg, err)
		return
	}

	_ = msg.Ack(false)
	logger.Info(ctx, "audit log saved successfully", "event_id", eventID, "event_type", msg.RoutingKey)
}

func (c *AuditConsumer) retry(ctx context.Context, msg amqp.Delivery, cause error) {
	if rabbitresilience.RetryCount(msg.Headers, "audit.payment_events.retry") >= rabbitresilience.MaxRetries {
		c.deadLetter(ctx, msg, cause)
		return
	}
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "audit.dlx.retry", msg.RoutingKey, msg, msg.Headers); err != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

func (c *AuditConsumer) deadLetter(ctx context.Context, msg amqp.Delivery, cause error) {
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "audit.dlx", msg.RoutingKey, msg, rabbitresilience.Headers(msg, cause.Error(), "audit.payment_events.retry")); err == nil {
		_ = msg.Ack(false)
		return
	}
	_ = msg.Nack(false, true)
}

func (c *AuditConsumer) cleanupConnection() {
	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.amqpConn != nil {
		_ = c.amqpConn.Close()
		c.amqpConn = nil
	}
}
