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

// Start begins listening to Redis Pub/Sub messages with an automatic reconnect loop and backoff
// This should be called in a goroutine: go subscriber.Start()
func (s *RedisSubscriber) Start() error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		// Attempt to subscribe to the Redis channel
		pubsub := s.rdb.Subscribe(s.ctx, s.channel)

		// Wait for subscription confirmation
		_, err := pubsub.Receive(s.ctx)
		if err != nil {
			pubsub.Close()

			if s.ctx.Err() != nil {
				return s.ctx.Err()
			}

			// Back off and retry
			if err := s.sleepBackoff(&backoff, maxBackoff); err != nil {
				return err
			}
			continue
		}

		// Reset exponential backoff on successful connection
		backoff = time.Second

		// Listen for messages
		ch := pubsub.Channel()
		ctxDone := s.listenChannel(ch)

		pubsub.Close() // Ensure subscription is closed when loop breaks

		if ctxDone {
			return s.ctx.Err()
		}

		// Channel closed unexpectedly, back off before reconnecting
		if err := s.sleepBackoff(&backoff, maxBackoff); err != nil {
			return err
		}
	}
}

// listenChannel processes messages from the pubsub channel until disconnected or context canceled
func (s *RedisSubscriber) listenChannel(ch <-chan *redis.Message) (ctxDone bool) {
	for {
		select {
		case <-s.ctx.Done():
			return true
		case msg, ok := <-ch:
			if !ok {
				// Channel closed
				return false
			}

			// Process the message
			s.handleMessage(msg.Payload)
		}
	}
}

// sleepBackoff waits for backoff duration or context cancellation
func (s *RedisSubscriber) sleepBackoff(backoff *time.Duration, maxBackoff time.Duration) error {
	timer := time.NewTimer(*backoff)
	defer timer.Stop()

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-timer.C:
		*backoff *= 2
		if *backoff > maxBackoff {
			*backoff = maxBackoff
		}
		return nil
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

// Stop gracefully shuts down the subscriber
func (s *RedisSubscriber) Stop() {
	s.cancel()
}
