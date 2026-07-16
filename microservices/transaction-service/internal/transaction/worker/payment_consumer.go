package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/rabbitresilience"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentConsumerWorker struct {
	rabbitmqURL string
	svc         service.TransactionService
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
	confirms    chan amqp.Confirmation
}

func NewPaymentConsumerWorker(rabbitmqURL string, svc service.TransactionService) *PaymentConsumerWorker {
	w := &PaymentConsumerWorker{
		rabbitmqURL: rabbitmqURL,
		svc:         svc,
	}
	if err := w.ensureConnection(); err != nil {
		logger.Fatal(context.Background(), "Failed to initialize RabbitMQ connection for payment consumer", "error", err)
	}
	return w
}

func (w *PaymentConsumerWorker) ensureConnection() error {
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
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ (payment consumer)...", "attempt", attempt)
		conn, err := amqp.Dial(w.rabbitmqURL)
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

		if err := rabbitresilience.Declare(ch, rabbitresilience.QueueConfig{MainQueue: "transaction.payment_settled", RetryQueue: "transaction.payment_settled.retry", DLQ: "transaction.payment_settled.dlq", DLX: "transaction.dlx", MainExchange: "payment.events", RoutingKey: "payment.settled", RetryTTL: 10000}); err != nil {
			ch.Close()
			conn.Close()
			lastErr = err
			continue
		}
		queue, err := ch.QueueDeclarePassive("transaction.payment_settled", true, false, false, false, nil)
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

		w.amqpConn = conn
		w.channel = ch
		w.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
		logger.Log.Info("Successfully connected to RabbitMQ and declared payment.settled queue!", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, lastErr)
}

func (w *PaymentConsumerWorker) Start(ctx context.Context) {
	logger.Log.Info("Payment Consumer Worker started...")

	for {
		if err := w.ensureConnection(); err != nil {
			logger.Error(ctx, "Failed to connect to RabbitMQ for payment consumer", "error", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := w.channel.Consume(
			"transaction.payment_settled",
			"payment-consumer",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error(ctx, "Failed to consume from transaction.payment_settled", "error", err.Error())
			w.cleanupConnection()
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Log.Info("Consuming payment.settled events from queue...")

		for {
			select {
			case <-ctx.Done():
				w.cleanupConnection()
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Log.Warn("Payment consumer channel closed, reconnecting...")
					w.cleanupConnection()
					time.Sleep(2 * time.Second)
					goto reconnect
				}
				w.processMessage(ctx, msg)
			}
		}

	reconnect:
	}
}

func (w *PaymentConsumerWorker) processMessage(ctx context.Context, msg amqp.Delivery) {
	var event model.PaymentSettledEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error(ctx, "Failed to unmarshal payment.settled event", "error", err.Error())
		w.deadLetter(ctx, msg, err)
		return
	}
	if event.EventID == "" || event.EventType != "payment.settled" || event.Provider == "" || event.ProviderPaymentID == "" || event.PaymentID == "" || event.UserID == "" || event.Amount == "" {
		w.deadLetter(ctx, msg, fmt.Errorf("invalid payment event: required fields missing"))
		return
	}

	logger.Log.Info("Received payment.settled event",
		"payment_id", event.PaymentID,
		"event_id", event.EventID,
	)

	if err := w.svc.ProcessPaymentSettled(ctx, event); err != nil {
		logger.Error(ctx, "Failed to process payment.settled event",
			"payment_id", event.PaymentID,
			"error", err.Error(),
		)
		w.retry(ctx, msg, err)
		return
	}

	_ = msg.Ack(false)
}

func (w *PaymentConsumerWorker) retry(ctx context.Context, msg amqp.Delivery, cause error) {
	if rabbitresilience.RetryCount(msg.Headers, "transaction.payment_settled.retry") >= rabbitresilience.MaxRetries {
		w.deadLetter(ctx, msg, cause)
		return
	}
	if err := rabbitresilience.PublishConfirmed(ctx, w.channel, w.confirms, "transaction.dlx.retry", msg.RoutingKey, msg, msg.Headers); err != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

func (w *PaymentConsumerWorker) deadLetter(ctx context.Context, msg amqp.Delivery, cause error) {
	if err := rabbitresilience.PublishConfirmed(ctx, w.channel, w.confirms, "transaction.dlx", msg.RoutingKey, msg, rabbitresilience.Headers(msg, cause.Error(), "transaction.payment_settled.retry")); err == nil {
		_ = msg.Ack(false)
		return
	}
	_ = msg.Nack(false, true)
}

func (w *PaymentConsumerWorker) cleanupConnection() {
	if w.channel != nil {
		_ = w.channel.Close()
		w.channel = nil
	}
	if w.amqpConn != nil {
		_ = w.amqpConn.Close()
		w.amqpConn = nil
	}
}
