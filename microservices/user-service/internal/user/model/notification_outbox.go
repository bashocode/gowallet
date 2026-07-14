package model

import "time"

// NotificationOutboxEvent represents a pending notification event to be published via RabbitMQ.
type NotificationOutboxEvent struct {
	ID          string    `json:"id"`
	EventType   string    `json:"event_type"`
	AggregateID string    `json:"aggregate_id"`
	Payload     []byte    `json:"payload"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	LastError   *string   `json:"last_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
