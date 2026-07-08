package model

import "time"

type OutboxEvent struct {
	ID        string    `json:"id"`
	EventType string    `json:"event_type"`
	Payload   string    `json:"payload"` // JSON string
	Status    string    `json:"status"`  // pending, processed, failed
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
