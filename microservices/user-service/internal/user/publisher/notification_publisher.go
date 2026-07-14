package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type NotificationEvent struct {
	EventID    string `json:"event_id"`
	To         string `json:"to"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	OccurredAt string `json:"occurred_at"`
}

type NotificationPublisher interface {
	PublishSendEmail(ctx context.Context, to string, subject string, body string) error
	Close() error
}

type rabbitMQNotificationPublisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func NewRabbitMQNotificationPublisher(rabbitmqURL string) (NotificationPublisher, error) {
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
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	logger.Log.Info("RabbitMQ notification publisher initialized successfully")
	return &rabbitMQNotificationPublisher{
		connection: conn,
		channel:    ch,
	}, nil
}

func (p *rabbitMQNotificationPublisher) PublishSendEmail(ctx context.Context, to string, subject string, body string) error {
	evt := NotificationEvent{
		EventID:    uuid.NewString(),
		To:         to,
		Subject:    subject,
		Body:       body,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		"notification.events",
		"notification.send_email",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    evt.EventID,
			Body:         payload,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish notification event: %w", err)
	}

	logger.Log.Info("Published notification.send_email event", "event_id", evt.EventID, "to", to)
	return nil
}

func (p *rabbitMQNotificationPublisher) Close() error {
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
