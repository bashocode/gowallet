package models

import "time"

type AuditLog struct {
	ID          string         `bson:"_id"`
	EventType   string         `bson:"event_type"`
	MessageID   string         `bson:"message_id"`
	Source      string         `bson:"source"`
	Payload     map[string]any `bson:"payload"`
	ReceivedAt  time.Time      `bson:"received_at"`
	ProcessedAt time.Time      `bson:"processed_at"`
}
