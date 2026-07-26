package websocket

import (
	"context"
	"encoding/json"
	"time"

	sharedWebSocket "github.com/bashocode/gowallet/microservices/shared/websocket"
	"github.com/redis/go-redis/v9"
)

// RedisSubscriber listens to Redis Pub/Sub and forwards messages to the Hub
type RedisSubscriber struct {
	rdb     *redis.Client
	hub     *Hub
	channel string
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRedisSubscriber creates a new Redis Pub/Sub subscriber
func NewRedisSubscriber(rdb *redis.Client, hub *Hub, channel string) *RedisSubscriber {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisSubscriber{
		rdb:     rdb,
		hub:     hub,
		channel: channel,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins listening to Redis Pub/Sub messages
// This should be called in a goroutine: go subscriber.Start()
func (s *RedisSubscriber) Start() error {
	// Subscribe to the Redis channel
	pubsub := s.rdb.Subscribe(s.ctx, s.channel)
	defer pubsub.Close()

	// Wait for subscription confirmation
	_, err := pubsub.Receive(s.ctx)
	if err != nil {
		return err
	}

	// Listen for messages
	ch := pubsub.Channel()

	for {
		select {
		case <-s.ctx.Done():
			// Shutdown requested
			return s.ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				// Channel closed, attempt to reconnect
				return s.reconnect()
			}

			// Process the message
			s.handleMessage(msg.Payload)
		}
	}
}

// handleMessage processes a single Redis Pub/Sub message
func (s *RedisSubscriber) handleMessage(payload string) {
	// Parse the Redis notification payload
	var notification sharedWebSocket.RedisNotificationPayload
	if err := json.Unmarshal([]byte(payload), &notification); err != nil {
		// Invalid message format - log and skip
		return
	}

	// Validate the payload
	if notification.UserID == "" || notification.Message == nil {
		// Invalid payload - skip
		return
	}

	// Serialize the WebSocket message
	messageBytes, err := notification.Message.ToJSON()
	if err != nil {
		// Serialization failed - skip
		return
	}

	// Forward to the user's WebSocket connection
	s.hub.SendToUser(notification.UserID, messageBytes)
}

// reconnect attempts to reconnect to Redis Pub/Sub with exponential backoff
func (s *RedisSubscriber) reconnect() error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(backoff):
			// Attempt to restart subscription
			if err := s.Start(); err != nil {
				// Increase backoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			return nil
		}
	}
}

// Stop gracefully shuts down the subscriber
func (s *RedisSubscriber) Stop() {
	s.cancel()
}
