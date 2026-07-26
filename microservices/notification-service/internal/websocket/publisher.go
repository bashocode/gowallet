package websocket

import (
	"context"
	"fmt"

	sharedWebSocket "github.com/bashocode/gowallet/microservices/shared/websocket"
	"github.com/redis/go-redis/v9"
)

// Publisher publishes WebSocket notifications to Redis Pub/Sub
// The API Gateway subscribes to these messages and forwards them to connected clients
type Publisher struct {
	rdb     *redis.Client
	channel string
}

// NewPublisher creates a new WebSocket notification publisher
func NewPublisher(rdb *redis.Client, channel string) *Publisher {
	return &Publisher{
		rdb:     rdb,
		channel: channel,
	}
}

// PublishNotification publishes a WebSocket notification to Redis Pub/Sub
// The notification will be forwarded to the user's WebSocket connection if they are connected
func (p *Publisher) PublishNotification(
	ctx context.Context,
	userID string,
	messageType string,
	title string,
	message string,
	data interface{},
) error {
	// Create WebSocket message
	wsMessage := sharedWebSocket.NewWebSocketMessage(messageType, title, message, data)

	// Create Redis payload
	payload := sharedWebSocket.NewRedisNotificationPayload(userID, wsMessage)

	// Serialize to JSON
	jsonPayload, err := payload.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize notification payload: %w", err)
	}

	// Publish to Redis
	if err := p.rdb.Publish(ctx, p.channel, jsonPayload).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis channel %s: %w", p.channel, err)
	}

	return nil
}
