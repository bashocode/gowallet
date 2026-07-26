package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sharedWebSocket "github.com/bashocode/gowallet/microservices/shared/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Test connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	return rdb
}

func TestNewRedisSubscriber(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	channel := "test:channel"

	subscriber := NewRedisSubscriber(rdb, hub, channel)

	assert.NotNil(t, subscriber)
	assert.Equal(t, rdb, subscriber.rdb)
	assert.Equal(t, hub, subscriber.hub)
	assert.Equal(t, channel, subscriber.channel)
	assert.NotNil(t, subscriber.ctx)
	assert.NotNil(t, subscriber.cancel)
}

func TestRedisSubscriber_HandleMessage_ValidPayload(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	subscriber := NewRedisSubscriber(rdb, hub, "test:channel")

	// Register a client
	client := &Client{
		UserID: "user123",
		Conn:   nil,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Create a valid notification payload
	wsMessage := sharedWebSocket.NewWebSocketMessage(
		"test_type",
		"Test Title",
		"Test Message",
		map[string]interface{}{"key": "value"},
	)

	payload := sharedWebSocket.NewRedisNotificationPayload("user123", wsMessage)
	payloadJSON, err := payload.ToJSON()
	require.NoError(t, err)

	// Handle the message
	subscriber.handleMessage(string(payloadJSON))

	// Verify client received the message
	select {
	case msg := <-client.Send:
		var decoded sharedWebSocket.WebSocketMessage
		err := json.Unmarshal(msg, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "test_type", decoded.Type)
		assert.Equal(t, "Test Title", decoded.Title)
		assert.Equal(t, "Test Message", decoded.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client did not receive message")
	}
}

func TestRedisSubscriber_HandleMessage_InvalidJSON(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	subscriber := NewRedisSubscriber(rdb, hub, "test:channel")

	// Handle invalid JSON (should not panic)
	assert.NotPanics(t, func() {
		subscriber.handleMessage("invalid json")
	})
}

func TestRedisSubscriber_HandleMessage_MissingUserID(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	subscriber := NewRedisSubscriber(rdb, hub, "test:channel")

	// Create payload with missing UserID
	payload := map[string]interface{}{
		"user_id": "",
		"message": map[string]interface{}{
			"type":    "test",
			"title":   "Test",
			"message": "Test",
		},
	}

	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	// Handle the message (should be skipped)
	assert.NotPanics(t, func() {
		subscriber.handleMessage(string(payloadJSON))
	})
}

func TestRedisSubscriber_HandleMessage_NilMessage(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	subscriber := NewRedisSubscriber(rdb, hub, "test:channel")

	// Create payload with nil message
	payload := map[string]interface{}{
		"user_id": "user123",
		"message": nil,
	}

	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	// Handle the message (should be skipped)
	assert.NotPanics(t, func() {
		subscriber.handleMessage(string(payloadJSON))
	})
}

func TestRedisSubscriber_HandleMessage_UserNotConnected(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	subscriber := NewRedisSubscriber(rdb, hub, "test:channel")

	// Create a valid payload for a user who is not connected
	wsMessage := sharedWebSocket.NewWebSocketMessage(
		"test_type",
		"Test Title",
		"Test Message",
		nil,
	)

	payload := sharedWebSocket.NewRedisNotificationPayload("nonexistent_user", wsMessage)
	payloadJSON, err := payload.ToJSON()
	require.NoError(t, err)

	// Handle the message (should not panic, message dropped)
	assert.NotPanics(t, func() {
		subscriber.handleMessage(string(payloadJSON))
	})
}

func TestRedisSubscriber_StartAndStop(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	channel := "test:subscriber:channel"
	subscriber := NewRedisSubscriber(rdb, hub, channel)

	// Start subscriber in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- subscriber.Start()
	}()

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Stop subscriber
	subscriber.Stop()

	// Verify it stopped
	select {
	case err := <-errChan:
		assert.Error(t, err) // Should return context.Canceled
		assert.Equal(t, context.Canceled, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Subscriber did not stop within timeout")
	}
}

func TestRedisSubscriber_IntegrationTest(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	channel := "test:integration:channel"
	subscriber := NewRedisSubscriber(rdb, hub, channel)

	// Start subscriber
	go subscriber.Start()
	time.Sleep(100 * time.Millisecond)

	// Register a client
	client := &Client{
		UserID: "user456",
		Conn:   nil,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Publish a message via Redis
	wsMessage := sharedWebSocket.NewWebSocketMessage(
		"transfer_received",
		"Money Received",
		"You received $100",
		map[string]interface{}{
			"amount":   100.0,
			"currency": "USD",
		},
	)

	payload := sharedWebSocket.NewRedisNotificationPayload("user456", wsMessage)
	payloadJSON, err := payload.ToJSON()
	require.NoError(t, err)

	ctx := context.Background()
	err = rdb.Publish(ctx, channel, string(payloadJSON)).Err()
	require.NoError(t, err)

	// Verify client received the message
	select {
	case msg := <-client.Send:
		var decoded sharedWebSocket.WebSocketMessage
		err := json.Unmarshal(msg, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "transfer_received", decoded.Type)
		assert.Equal(t, "Money Received", decoded.Title)
		assert.Equal(t, "You received $100", decoded.Message)
		assert.NotNil(t, decoded.Data)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Client did not receive published message")
	}

	// Cleanup
	subscriber.Stop()
}

func TestRedisSubscriber_MultipleMessages(t *testing.T) {
	rdb := setupTestRedis(t)
	defer rdb.Close()

	hub := NewHub()
	go hub.Run()

	channel := "test:multiple:channel"
	subscriber := NewRedisSubscriber(rdb, hub, channel)

	go subscriber.Start()
	time.Sleep(100 * time.Millisecond)

	// Register a client
	client := &Client{
		UserID: "user789",
		Conn:   nil,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Publish multiple messages
	messageTypes := []string{"transfer_sent", "transfer_received", "topup_success"}
	ctx := context.Background()

	for _, msgType := range messageTypes {
		wsMessage := sharedWebSocket.NewWebSocketMessage(
			msgType,
			"Test Title",
			"Test Message",
			nil,
		)

		payload := sharedWebSocket.NewRedisNotificationPayload("user789", wsMessage)
		payloadJSON, err := payload.ToJSON()
		require.NoError(t, err)

		err = rdb.Publish(ctx, channel, string(payloadJSON)).Err()
		require.NoError(t, err)
	}

	// Verify all messages were received
	receivedTypes := make([]string, 0)
	timeout := time.After(1 * time.Second)

	for i := 0; i < len(messageTypes); i++ {
		select {
		case msg := <-client.Send:
			var decoded sharedWebSocket.WebSocketMessage
			err := json.Unmarshal(msg, &decoded)
			require.NoError(t, err)
			receivedTypes = append(receivedTypes, decoded.Type)
		case <-timeout:
			t.Fatalf("Only received %d/%d messages", i, len(messageTypes))
		}
	}

	assert.ElementsMatch(t, messageTypes, receivedTypes)

	// Cleanup
	subscriber.Stop()
}
