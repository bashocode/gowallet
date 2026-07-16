package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/notification-service/internal/email"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	rabbitresilience "github.com/bashocode/gowallet/microservices/shared/rabbitresilience"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentSettledEvent struct {
	EventID           string `json:"event_id"`
	EventType         string `json:"event_type"`
	Provider          string `json:"provider"`
	ProviderPaymentID string `json:"provider_payment_id"`
	PaymentID         string `json:"payment_id"`
	UserID            string `json:"user_id"`
	UserEmail         string `json:"user_email"`
	Amount            string `json:"amount"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	OccurredAt        string `json:"occurred_at"`
}

type PaymentNotificationConsumer struct {
	rabbitmqURL      string
	notificationRepo *repository.NotificationRepository
	userGRPCClient   pb.UserServiceClient
	emailSender      email.EmailSender
	amqpConn         *amqp.Connection
	channel          *amqp.Channel
	confirms         chan amqp.Confirmation
}

func NewPaymentNotificationConsumer(rabbitmqURL string, repo *repository.NotificationRepository, userClient pb.UserServiceClient, emailSender email.EmailSender) *PaymentNotificationConsumer {
	w := &PaymentNotificationConsumer{
		rabbitmqURL:      rabbitmqURL,
		notificationRepo: repo,
		userGRPCClient:   userClient,
		emailSender:      emailSender,
	}
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(context.Background(), "failed to initialize RabbitMQ connection for notification consumer", "error", err)
	}
	return w
}

func (c *PaymentNotificationConsumer) ensureConnection() error {
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
		logger.Log.Info("connecting/reconnecting to RabbitMQ (notification consumer)...", "attempt", attempt)
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

		if err := rabbitresilience.Declare(ch, rabbitresilience.QueueConfig{MainQueue: "notification.payment_settled", RetryQueue: "notification.payment_settled.retry", DLQ: "notification.payment_settled.dlq", DLX: "notification.dlx", MainExchange: "payment.events", RoutingKey: "payment.settled", RetryTTL: 10000}); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}
		queue, err := ch.QueueDeclarePassive("notification.payment_settled", true, false, false, false, nil)
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
			"payment.settled",
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
		logger.Log.Info("successfully connected to RabbitMQ and declared notification.payment_settled queue!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (c *PaymentNotificationConsumer) Start(ctx context.Context) {
	logger.Log.Info("notification consumer started...")

	for {
		if err := c.ensureConnection(); err != nil {
			logger.Error(ctx, "failed to connect to RabbitMQ for notification consumer", "error", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := c.channel.Consume(
			"notification.payment_settled",
			"notification-consumer",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error(ctx, "failed to consume from notification.payment_settled", "error", err.Error())
			c.cleanupConnection()
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Log.Info("consuming payment.settled events from notification queue...")

		for {
			select {
			case <-ctx.Done():
				c.cleanupConnection()
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Log.Warn("notification consumer channel closed, reconnecting...")
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

func (c *PaymentNotificationConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	var event PaymentSettledEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error(ctx, "invalid payment event payload", "error", err)
		c.deadLetter(ctx, msg, err)
		return
	}
	if event.EventID == "" || event.EventType != "payment.settled" || event.UserID == "" || event.PaymentID == "" {
		c.deadLetter(ctx, msg, fmt.Errorf("invalid payment event: event_id, user_id, and payment_id are required"))
		return
	}

	hasProcessed, err := c.notificationRepo.HasProcessed(ctx, event.EventID)
	if err != nil {
		logger.Error(ctx, "failed to check if event was processed", "error", err, "event_id", event.EventID)
		c.retry(ctx, msg, err)
		return
	}

	if hasProcessed {
		logger.Info(ctx, "event already processed, skipping", "event_id", event.EventID)
		_ = msg.Ack(false)
		return
	}

	// Fetch user details from user-service using UserID from the event
	userResp, err := c.userGRPCClient.GetUserByID(ctx, &pb.GetUserRequest{
		Id: event.UserID,
	})
	if err != nil {
		logger.Error(ctx, "failed to fetch user details from user-service", "error", err, "user_id", event.UserID)
		c.retry(ctx, msg, err)
		return
	}

	logger.Info(ctx, "sending payment settled notification via email",
		"user_id", event.UserID,
		"user_email", userResp.GetEmail(),
		"payment_id", event.PaymentID,
		"amount", event.Amount,
		"currency", event.Currency,
	)

	subject := "Payment Settled - GoWallet"
	body := fmt.Sprintf(
		"Dear User,\n\nYour payment has been successfully settled.\n\nPayment ID: %s\nAmount: %s %s\nStatus: %s\n\nThank you for using GoWallet!",
		event.PaymentID,
		event.Currency,
		event.Amount,
		event.Status,
	)

	err = c.emailSender.SendEmail(ctx, userResp.GetEmail(), subject, body)
	if err != nil {
		logger.Error(ctx, "failed to send email notification", "error", err, "user_email", userResp.GetEmail())
		c.retry(ctx, msg, err)
		return
	}

	if err := c.notificationRepo.MarkProcessed(ctx, event.EventID); err != nil {
		logger.Error(ctx, "failed to mark event as processed", "error", err, "event_id", event.EventID)
		c.retry(ctx, msg, err)
		return
	}

	_ = msg.Ack(false)
	logger.Info(ctx, "notification sent successfully", "event_id", event.EventID, "payment_id", event.PaymentID)
}

func (c *PaymentNotificationConsumer) retry(ctx context.Context, msg amqp.Delivery, cause error) {
	if rabbitresilience.RetryCount(msg.Headers, "notification.payment_settled.retry") >= rabbitresilience.MaxRetries {
		c.deadLetter(ctx, msg, cause)
		return
	}
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "notification.dlx.retry", msg.RoutingKey, msg, msg.Headers); err != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

func (c *PaymentNotificationConsumer) deadLetter(ctx context.Context, msg amqp.Delivery, cause error) {
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "notification.dlx", msg.RoutingKey, msg, rabbitresilience.Headers(msg, cause.Error(), "notification.payment_settled.retry")); err == nil {
		_ = msg.Ack(false)
		return
	}
	_ = msg.Nack(false, true)
}

func (c *PaymentNotificationConsumer) cleanupConnection() {
	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.amqpConn != nil {
		_ = c.amqpConn.Close()
		c.amqpConn = nil
	}
}
