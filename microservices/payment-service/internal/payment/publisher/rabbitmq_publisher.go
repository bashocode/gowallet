package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPaymentPublisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func NewRabbitMQPaymentPublisher(rabbitmqURL string) (*RabbitMQPaymentPublisher, error) {
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create channel: %w", err)
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
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	logger.Log.Info("RabbitMQ payment publisher initialized successfully")
	return &RabbitMQPaymentPublisher{
		connection: conn,
		channel:    ch,
	}, nil
}

func (p *RabbitMQPaymentPublisher) PublishPaymentSettled(ctx context.Context, evt model.PaymentSettledEvent) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		"payment.events",
		"payment.settled",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    evt.EventID,
			Type:         evt.EventType,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	logger.Log.Info("Published payment.settled event", "event_id", evt.EventID, "payment_id", evt.PaymentID)
	return nil
}

func (p *RabbitMQPaymentPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			logger.Log.Error("Failed to close channel", "error", err)
		}
	}
	if p.connection != nil {
		if err := p.connection.Close(); err != nil {
			logger.Log.Error("Failed to close connection", "error", err)
		}
	}
	return nil
}
