package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/notification-service/internal/email"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/websocket"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/rabbitresilience"
	sharedWebSocket "github.com/bashocode/gowallet/microservices/shared/websocket"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	amqp "github.com/rabbitmq/amqp091-go"
)

type TransferInitiatedEvent struct {
	EventID        string `json:"event_id"`
	EventType      string `json:"event_type"`
	TransferID     string `json:"transfer_id"`
	SenderUserID   string `json:"sender_user_id"`
	ReceiverEmail  string `json:"receiver_email"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
	OccurredAt     string `json:"occurred_at"`
}

type TransferSettledEvent struct {
	EventID         string `json:"event_id"`
	EventType       string `json:"event_type"`
	TransferID      string `json:"transfer_id"`
	SenderUserID    string `json:"sender_user_id"`
	ReceiverUserID  string `json:"receiver_user_id"`
	ReceiverEmail   string `json:"receiver_email"`
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
	ExternalEwallet string `json:"external_ewallet"`
	OccurredAt      string `json:"occurred_at"`
}

type TransferNotificationConsumer struct {
	rabbitmqURL      string
	notificationRepo *repository.NotificationRepository
	userGRPCClient   pb.UserServiceClient
	emailSender      email.EmailSender
	wsPublisher      *websocket.Publisher
	amqpConn         *amqp.Connection
	channel          *amqp.Channel
	confirms         chan amqp.Confirmation
}

func NewTransferNotificationConsumer(
	rabbitmqURL string,
	repo *repository.NotificationRepository,
	userClient pb.UserServiceClient,
	emailSender email.EmailSender,
	wsPublisher *websocket.Publisher,
) *TransferNotificationConsumer {
	c := &TransferNotificationConsumer{
		rabbitmqURL:      rabbitmqURL,
		notificationRepo: repo,
		userGRPCClient:   userClient,
		emailSender:      emailSender,
		wsPublisher:      wsPublisher,
	}
	if err := c.ensureConnection(); err != nil {
		logger.Fatal(context.Background(), "failed to initialize RabbitMQ connection for transfer consumer", "error", err)
	}
	return c
}

func (c *TransferNotificationConsumer) ensureConnection() error {
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
		logger.Log.Info("connecting to RabbitMQ (transfer consumer)...", "attempt", attempt)
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

		if err := ch.ExchangeDeclare("transfer.events", "topic", true, false, false, false, nil); err != nil {
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

		if err := rabbitresilience.Declare(ch, rabbitresilience.QueueConfig{
			MainQueue:    "notification.transfer_events",
			RetryQueue:   "notification.transfer_events.retry",
			DLQ:          "notification.transfer_events.dlq",
			DLX:          "notification.dlx",
			MainExchange: "transfer.events",
			RoutingKey:   "transfer.#",
			RetryTTL:     10000,
		}); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}

		queue, err := ch.QueueDeclarePassive("notification.transfer_events", true, false, false, false, nil)
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

		if err := ch.QueueBind(queue.Name, "transfer.initiated", "transfer.events", false, nil); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}

		if err := ch.QueueBind(queue.Name, "transfer.success", "transfer.events", false, nil); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}

		if err := ch.QueueBind(queue.Name, "transfer.completed", "transfer.events", false, nil); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}

		if err := ch.QueueBind(queue.Name, "transfer.settled", "transfer.events", false, nil); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
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
		logger.Log.Info("successfully connected to RabbitMQ and declared notification.transfer_events queue!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (c *TransferNotificationConsumer) Start(ctx context.Context) {
	logger.Log.Info("transfer notification consumer started...")

	for {
		if err := c.ensureConnection(); err != nil {
			logger.Error(ctx, "failed to connect to RabbitMQ for transfer consumer", "error", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := c.channel.Consume(
			"notification.transfer_events",
			"transfer-notification-consumer",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error(ctx, "failed to consume from notification.transfer_events", "error", err.Error())
			c.cleanupConnection()
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Log.Info("consuming transfer events from notification queue...")

		for {
			select {
			case <-ctx.Done():
				c.cleanupConnection()
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Log.Warn("transfer consumer channel closed, reconnecting...")
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

func (c *TransferNotificationConsumer) processMessage(ctx context.Context, msg amqp.Delivery) {
	var eventType string
	var eventMap map[string]interface{}
	if err := json.Unmarshal(msg.Body, &eventMap); err != nil {
		logger.Error(ctx, "invalid transfer event payload", "error", err)
		c.deadLetter(ctx, msg, err)
		return
	}

	if et, ok := eventMap["event_type"].(string); ok && et != "" {
		eventType = et
	} else if msg.RoutingKey != "" {
		eventType = msg.RoutingKey
	} else if msg.Type != "" {
		eventType = msg.Type
	}

	switch eventType {
	case "transfer.initiated":
		c.handleTransferInitiated(ctx, msg)
	case "transfer.settled", "transfer.completed", "transfer.success":
		c.handleTransferSettled(ctx, msg)
	default:
		logger.Warn(ctx, "unknown transfer event type", "event_type", eventType)
		_ = msg.Ack(false)
	}
}

func (c *TransferNotificationConsumer) handleTransferInitiated(ctx context.Context, msg amqp.Delivery) {
	var event TransferInitiatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error(ctx, "invalid transfer.initiated payload", "error", err)
		c.deadLetter(ctx, msg, err)
		return
	}

	if event.EventID == "" || event.SenderUserID == "" || event.TransferID == "" {
		c.deadLetter(ctx, msg, fmt.Errorf("invalid transfer.initiated event: missing required fields"))
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

	senderResp, err := c.userGRPCClient.GetUserByID(ctx, &pb.GetUserRequest{Id: event.SenderUserID})
	if err != nil {
		logger.Error(ctx, "failed to fetch sender details", "error", err, "user_id", event.SenderUserID)
		c.retry(ctx, msg, err)
		return
	}

	subject := "Transfer Initiated - GoWallet"
	body := fmt.Sprintf(
		"Dear %s,\n\nYour transfer has been initiated.\n\nTransfer ID: %s\nRecipient: %s\nAmount: %s %s\n\nThank you for using GoWallet!",
		senderResp.GetFullName(),
		event.TransferID,
		event.ReceiverEmail,
		event.Currency,
		event.Amount,
	)

	if err := c.emailSender.SendEmail(ctx, senderResp.GetEmail(), subject, body); err != nil {
		logger.Error(ctx, "failed to send email", "error", err)
		c.retry(ctx, msg, err)
		return
	}

	if c.wsPublisher != nil {
		wsErr := c.wsPublisher.PublishNotification(
			ctx,
			event.SenderUserID,
			sharedWebSocket.MessageTypeTransferSent,
			"Transfer Initiated",
			fmt.Sprintf("Sending %s %s to %s", event.Currency, event.Amount, event.ReceiverEmail),
			map[string]interface{}{
				"transfer_id":     event.TransferID,
				"amount":          event.Amount,
				"currency":        event.Currency,
				"receiver_email":  event.ReceiverEmail,
				"idempotency_key": event.IdempotencyKey,
			},
		)
		if wsErr != nil {
			logger.Warn(ctx, "failed to send WebSocket notification", "error", wsErr, "user_id", event.SenderUserID)
		}
	}

	if err := c.notificationRepo.MarkProcessed(ctx, event.EventID); err != nil {
		logger.Error(ctx, "failed to mark event as processed", "error", err, "event_id", event.EventID)
		c.retry(ctx, msg, err)
		return
	}

	_ = msg.Ack(false)
	logger.Info(ctx, "transfer.initiated notification sent", "event_id", event.EventID)
}

func (c *TransferNotificationConsumer) handleTransferSettled(ctx context.Context, msg amqp.Delivery) {
	var event TransferSettledEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error(ctx, "invalid transfer.settled payload", "error", err)
		c.deadLetter(ctx, msg, err)
		return
	}

	if event.EventID == "" || event.TransferID == "" {
		c.deadLetter(ctx, msg, fmt.Errorf("invalid transfer.settled event: missing required fields"))
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

	switch event.Status {
	case "completed", "success", "settled":
		if event.ReceiverUserID != "" {
			receiverResp, err := c.userGRPCClient.GetUserByID(ctx, &pb.GetUserRequest{Id: event.ReceiverUserID})
			if err != nil {
				logger.Error(ctx, "failed to fetch receiver details", "error", err, "user_id", event.ReceiverUserID)
				c.retry(ctx, msg, err)
				return
			}

			subject := "Money Received - GoWallet"
			body := fmt.Sprintf(
				"Dear %s,\n\nYou received a transfer!\n\nTransfer ID: %s\nAmount: %s %s\nFrom: %s\n\nThank you for using GoWallet!",
				receiverResp.GetFullName(),
				event.TransferID,
				event.Currency,
				event.Amount,
				event.SenderUserID,
			)

			if err := c.emailSender.SendEmail(ctx, receiverResp.GetEmail(), subject, body); err != nil {
				logger.Error(ctx, "failed to send email to receiver", "error", err)
				c.retry(ctx, msg, err)
				return
			}

			if c.wsPublisher != nil {
				wsErr := c.wsPublisher.PublishNotification(
					ctx,
					event.ReceiverUserID,
					sharedWebSocket.MessageTypeTransferReceived,
					"Money Received!",
					fmt.Sprintf("You received %s %s", event.Currency, event.Amount),
					map[string]interface{}{
						"transfer_id": event.TransferID,
						"amount":      event.Amount,
						"currency":    event.Currency,
						"sender_id":   event.SenderUserID,
					},
				)
				if wsErr != nil {
					logger.Warn(ctx, "failed to send WebSocket notification to receiver", "error", wsErr, "user_id", event.ReceiverUserID)
				}
			}
		}

		if event.SenderUserID != "" {
			senderResp, err := c.userGRPCClient.GetUserByID(ctx, &pb.GetUserRequest{Id: event.SenderUserID})
			if err != nil {
				logger.Error(ctx, "failed to fetch sender details", "error", err, "user_id", event.SenderUserID)
				c.retry(ctx, msg, err)
				return
			}

			subject := "Transfer Successful - GoWallet"
			body := fmt.Sprintf(
				"Dear %s,\n\nYour transfer was successful.\n\nTransfer ID: %s\nRecipient: %s\nAmount: %s %s\n\nThank you for using GoWallet!",
				senderResp.GetFullName(),
				event.TransferID,
				event.ReceiverEmail,
				event.Currency,
				event.Amount,
			)

			if err := c.emailSender.SendEmail(ctx, senderResp.GetEmail(), subject, body); err != nil {
				logger.Error(ctx, "failed to send email to sender", "error", err)
				c.retry(ctx, msg, err)
				return
			}

			if c.wsPublisher != nil {
				wsErr := c.wsPublisher.PublishNotification(
					ctx,
					event.SenderUserID,
					sharedWebSocket.MessageTypeTransferSent,
					"Transfer Successful",
					fmt.Sprintf("Your transfer of %s %s was successful", event.Currency, event.Amount),
					map[string]interface{}{
						"transfer_id":    event.TransferID,
						"amount":         event.Amount,
						"currency":       event.Currency,
						"receiver_email": event.ReceiverEmail,
					},
				)
				if wsErr != nil {
					logger.Warn(ctx, "failed to send WebSocket notification to sender", "error", wsErr, "user_id", event.SenderUserID)
				}
			}
		}
	case "failed":
		if event.SenderUserID != "" {
			senderResp, err := c.userGRPCClient.GetUserByID(ctx, &pb.GetUserRequest{Id: event.SenderUserID})
			if err != nil {
				logger.Error(ctx, "failed to fetch sender details", "error", err, "user_id", event.SenderUserID)
				c.retry(ctx, msg, err)
				return
			}

			subject := "Transfer Failed - GoWallet"
			body := fmt.Sprintf(
				"Dear %s,\n\nYour transfer has failed.\n\nTransfer ID: %s\nRecipient: %s\nAmount: %s %s\n\nPlease contact support if you need assistance.\n\nThank you for using GoWallet!",
				senderResp.GetFullName(),
				event.TransferID,
				event.ReceiverEmail,
				event.Currency,
				event.Amount,
			)

			if err := c.emailSender.SendEmail(ctx, senderResp.GetEmail(), subject, body); err != nil {
				logger.Error(ctx, "failed to send email to sender", "error", err)
				c.retry(ctx, msg, err)
				return
			}

			if c.wsPublisher != nil {
				wsErr := c.wsPublisher.PublishNotification(
					ctx,
					event.SenderUserID,
					sharedWebSocket.MessageTypeTransferFailed,
					"Transfer Failed",
					fmt.Sprintf("Transfer to %s failed", event.ReceiverEmail),
					map[string]interface{}{
						"transfer_id":    event.TransferID,
						"amount":         event.Amount,
						"currency":       event.Currency,
						"receiver_email": event.ReceiverEmail,
					},
				)
				if wsErr != nil {
					logger.Warn(ctx, "failed to send WebSocket notification to sender", "error", wsErr, "user_id", event.SenderUserID)
				}
			}
		}
	}

	if err := c.notificationRepo.MarkProcessed(ctx, event.EventID); err != nil {
		logger.Error(ctx, "failed to mark event as processed", "error", err, "event_id", event.EventID)
		c.retry(ctx, msg, err)
		return
	}

	_ = msg.Ack(false)
	logger.Info(ctx, "transfer.settled notification sent", "event_id", event.EventID, "status", event.Status)
}

func (c *TransferNotificationConsumer) retry(ctx context.Context, msg amqp.Delivery, cause error) {
	if rabbitresilience.RetryCount(msg.Headers, "notification.transfer_events.retry") >= rabbitresilience.MaxRetries {
		c.deadLetter(ctx, msg, cause)
		return
	}
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "notification.dlx.retry", msg.RoutingKey, msg, msg.Headers); err != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

func (c *TransferNotificationConsumer) deadLetter(ctx context.Context, msg amqp.Delivery, cause error) {
	if err := rabbitresilience.PublishConfirmed(ctx, c.channel, c.confirms, "notification.dlx", msg.RoutingKey, msg, rabbitresilience.Headers(msg, cause.Error(), "notification.transfer_events.retry")); err == nil {
		_ = msg.Ack(false)
		return
	}
	_ = msg.Nack(false, true)
}

func (c *TransferNotificationConsumer) cleanupConnection() {
	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.amqpConn != nil {
		_ = c.amqpConn.Close()
		c.amqpConn = nil
	}
}
