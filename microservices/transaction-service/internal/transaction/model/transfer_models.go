package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// OutboundTransfer tracks a transfer from a GoWallet user to a user in an
// external ewallet (the monolith app). The transfer starts as "pending" and is
// settled when the external ewallet calls back GoWallet's webhook.
type OutboundTransfer struct {
	ID              string          `json:"id"`
	SenderUserID    string          `json:"sender_user_id"`
	SenderWalletID  string          `json:"sender_wallet_id"`
	ReceiverEmail   string          `json:"receiver_email"`
	Amount          decimal.Decimal `json:"amount"`
	Currency        string          `json:"currency"`
	ExternalEwallet string          `json:"external_ewallet"`
	Status          string          `json:"status"` // pending, settled, failed
	IdempotencyKey  string          `json:"idempotency_key"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ExternalTransferRequest struct {
	ReceiverEmail  string          `json:"receiver_email" binding:"required,email" example:"receiver@monolith.test"`
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"50000"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"unique-key-123"`
}

type TransferCallback struct {
	TransferID     string `json:"transfer_id"`
	Status         string `json:"status"`
	ReceiverEmail  string `json:"receiver_email"`
	Amount         string `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

// TransferSettledEvent is the normalized event published to RabbitMQ when an
// outbound transfer is settled or failed.
type TransferSettledEvent struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	TransferID      string    `json:"transfer_id"`
	SenderUserID    string    `json:"sender_user_id"`
	ReceiverEmail   string    `json:"receiver_email"`
	Amount          string    `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	ExternalEwallet string    `json:"external_ewallet"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// TransferInitiatedEvent is published to RabbitMQ when a transfer is created.
// The consumer picks it up to asynchronously validate the receiver and notify
// the monolith, keeping the API response fast.
type TransferInitiatedEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	TransferID     string    `json:"transfer_id"`
	SenderUserID   string    `json:"sender_user_id"`
	ReceiverEmail  string    `json:"receiver_email"`
	Amount         string    `json:"amount"`
	Currency       string    `json:"currency"`
	IdempotencyKey string    `json:"idempotency_key"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// TransferOutboxEvent mirrors a row in transfer_outbox_events.
type TransferOutboxEvent struct {
	ID          string    `json:"id"`
	EventType   string    `json:"event_type"`
	AggregateID string    `json:"aggregate_id"`
	Payload     string    `json:"payload"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
