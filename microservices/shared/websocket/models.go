package websocket

import (
	"encoding/json"
	"time"
)

// WebSocketMessage represents a message sent to the client over WebSocket
type WebSocketMessage struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ToJSON serializes the message to JSON bytes
func (m *WebSocketMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// RedisNotificationPayload represents the payload published to Redis Pub/Sub
// This is used to broadcast notifications across multiple API Gateway instances
type RedisNotificationPayload struct {
	UserID  string            `json:"user_id"`
	Message *WebSocketMessage `json:"message"`
}

// ToJSON serializes the payload to JSON bytes
func (p *RedisNotificationPayload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON deserializes JSON bytes to RedisNotificationPayload
func (p *RedisNotificationPayload) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewWebSocketMessage creates a new WebSocket message with the current timestamp
func NewWebSocketMessage(msgType, title, message string, data any) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      msgType,
		Title:     title,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewRedisNotificationPayload creates a new Redis notification payload
func NewRedisNotificationPayload(userID string, message *WebSocketMessage) *RedisNotificationPayload {
	return &RedisNotificationPayload{
		UserID:  userID,
		Message: message,
	}
}
