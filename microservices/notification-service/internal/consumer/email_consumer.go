package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/notification-service/internal/email"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type SendEmailEvent struct {
	EventID    string `json:"event_id"`
	To         string `json:"to"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	OccurredAt string `json:"occurred_at"`
}

type EmailNotificationConsumer struct {
	rabbitmqURL      string
	notificationRepo *repository.NotificationRepository
	emailSender      email.EmailSender
	amqpConn         *amqp.Connection
	channel          *amqp.Channel
}

func NewEmailNotificationConsumer(rabbitmqURL string, repo *repository.NotificationRepository, emailSender email.EmailSender) *EmailNotificationConsumer {
	w := &EmailNotificationConsumer{
		rabbitmqURL:      rabbitmqURL,
		notificationRepo: repo,
		emailSender:      emailSender,
	}
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(nil, "failed to initialize RabbitMQ connection for email notification consumer", "error", err)
	}
	return w
}

func (c *EmailNotificationConsumer) ensureConnection() error {
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
		logger.Log.Info("connecting/reconnecting to RabbitMQ (email consumer)...", "attempt", attempt)
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
			"notification.events",
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

		queue, err := ch.QueueDeclare(
			"notification.send_email.queue",
			true,
			false,
			false,
			false,
			nil,
		)
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

		if err := ch.QueueBind(
			queue.Name,
			"notification.send_email",
			"notification.events",
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

		c.amqpConn = conn
		c.channel = ch
		logger.Log.Info("successfully connected to RabbitMQ and declared notification.send_email queue!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (c *EmailNotificationConsumer) Start(ctx context.Context) {
	logger.Log.Info("email notification consumer started...")

	for {
		if err := c.ensureConnection(); err != nil {
			logger.Error(ctx, "failed to connect to RabbitMQ for email consumer", "error", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := c.channel.Consume(
			"notification.send_email.queue",
			"email-notification-consumer",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error(ctx, "failed to consume from notification.send_email.queue", "error", err.Error())
			c.cleanupConnection()
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Log.Info("consuming notification.send_email events from queue...")

		for {
			select {
			case <-ctx.Done():
				c.cleanupConnection()
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Log.Warn("email consumer channel closed, reconnecting...")
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

func (c *EmailNotificationConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	var event SendEmailEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error(ctx, "invalid email event payload", "error", err)
		_ = msg.Nack(false, false)
		return
	}

	hasProcessed, err := c.notificationRepo.HasProcessed(ctx, event.EventID)
	if err != nil {
		logger.Error(ctx, "failed to check if event was processed", "error", err, "event_id", event.EventID)
		_ = msg.Nack(false, true)
		return
	}

	if hasProcessed {
		logger.Info(ctx, "email event already processed, skipping", "event_id", event.EventID)
		_ = msg.Ack(false)
		return
	}

	logger.Info(ctx, "sending email from rabbitmq event",
		"to", event.To,
		"subject", event.Subject,
		"event_id", event.EventID,
	)

	err = c.emailSender.SendEmail(ctx, event.To, event.Subject, event.Body)
	if err != nil {
		logger.Error(ctx, "failed to send email via SMTP", "error", err, "to", event.To)
		_ = msg.Nack(false, true)
		return
	}

	if err := c.notificationRepo.MarkProcessed(ctx, event.EventID); err != nil {
		logger.Error(ctx, "failed to mark event as processed", "error", err, "event_id", event.EventID)
		_ = msg.Nack(false, true)
		return
	}

	_ = msg.Ack(false)
	logger.Info(ctx, "email sent and event processed successfully", "event_id", event.EventID)
}

func (c *EmailNotificationConsumer) cleanupConnection() {
	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.amqpConn != nil {
		_ = c.amqpConn.Close()
		c.amqpConn = nil
	}
}
