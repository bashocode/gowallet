package rabbitresilience

import (
	"context"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const MaxRetries = 3

type QueueConfig struct {
	MainQueue    string
	RetryQueue   string
	DLQ          string
	DLX          string
	MainExchange string
	RoutingKey   string
	RetryTTL     int32
}

func Declare(ch *amqp.Channel, cfg QueueConfig) error {
	if err := ch.ExchangeDeclare(cfg.MainExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(cfg.DLX, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(cfg.DLX+".retry", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.DLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.DLQ, "#", cfg.DLX, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.RetryQueue, true, false, false, false, amqp.Table{
		"x-message-ttl": cfg.RetryTTL, "x-dead-letter-exchange": cfg.MainExchange,
	}); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.RetryQueue, cfg.RoutingKey, cfg.DLX+".retry", false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.MainQueue, true, false, false, false, amqp.Table{"x-dead-letter-exchange": cfg.DLX}); err != nil {
		return err
	}
	return ch.QueueBind(cfg.MainQueue, cfg.RoutingKey, cfg.MainExchange, false, nil)
}

func RetryCount(headers amqp.Table, retryQueue string) int {
	var count int64
	deaths, ok := headers["x-death"].([]interface{})
	if !ok {
		return 0
	}
	for _, raw := range deaths {
		entry, ok := raw.(amqp.Table)
		if !ok || fmt.Sprint(entry["queue"]) != retryQueue {
			continue
		}
		switch n := entry["count"].(type) {
		case int64:
			count += n
		case int32:
			count += int64(n)
		case int:
			count += int64(n)
		}
	}
	return int(count)
}

func PublishConfirmed(ctx context.Context, ch *amqp.Channel, confirms <-chan amqp.Confirmation, exchange, key string, msg amqp.Delivery, headers amqp.Table) error {
	if headers == nil {
		headers = amqp.Table{}
	}
	publishing := amqp.Publishing{ContentType: msg.ContentType, DeliveryMode: msg.DeliveryMode, MessageId: msg.MessageId, Headers: headers, Body: msg.Body}
	if err := ch.PublishWithContext(ctx, exchange, key, false, false, publishing); err != nil {
		return err
	}
	select {
	case confirmation := <-confirms:
		if !confirmation.Ack {
			return errors.New("broker rejected message")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func Headers(msg amqp.Delivery, reason, retryQueue string) amqp.Table {
	h := amqp.Table{}
	for k, v := range msg.Headers {
		h[k] = v
	}
	h["x-failure-reason"] = reason
	h["x-original-routing-key"] = msg.RoutingKey
	h["x-final-attempt"] = RetryCount(msg.Headers, retryQueue)
	return h
}
