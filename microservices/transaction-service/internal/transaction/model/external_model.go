package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Add these to the end of tx.go
type WalletInquiry struct {
	Valid     bool   `json:"valid"`
	AccountID string `json:"account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type ExternalInquiryRequest struct {
	Email string `json:"email" binding:"required,email" example:"recipient@example.com"`
}

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

// EmailInquiryRequest is sent to monolith inquiry endpoint
type EmailInquiryRequest struct {
	Email string `json:"email"`
}

// EmailInquiryResponse is returned from monolith inquiry endpoint
type EmailInquiryResponse struct {
	Valid     bool   `json:"valid"`
	AccountID string `json:"account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
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

// TransferOutboxEvent mirrors transfer events stored in the shared outbox_events table.
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
