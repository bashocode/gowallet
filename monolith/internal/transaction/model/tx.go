package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID               string          `json:"id"`
	SenderWalletID   *string         `json:"sender_wallet_id"` // nullable if top up
	ReceiverWalletID string          `json:"receiver_wallet_id"`
	Amount           decimal.Decimal `json:"amount"`
	Description      string          `json:"description"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
}

type TransferRequest struct {
	ReceiverEmail  string          `json:"receiver_email" binding:"required,email" example:"receiver@example.com"`
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"50000"`
	Description    string          `json:"description" example:"Dinner split"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"unique-uuid-key-123"`
}

type TopUpRequest struct {
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"100000"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"unique-uuid-key-abc"`
}

type ExternalTransferRequest struct {
	TransferID     string          `json:"transfer_id" binding:"required" example:"external-transfer-id"`
	ReceiverEmail  string          `json:"receiver_email" binding:"required,email" example:"receiver@example.com"`
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"50000"`
	Currency       string          `json:"currency" example:"IDR"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"unique-uuid-key-123"`
	SenderUserID   string          `json:"sender_user_id" binding:"required" example:"gowallet-user-id"`
	CallbackURL    string          `json:"callback_url,omitempty"`
}

type ExternalTransferStatus struct {
	TransferID     string `json:"transfer_id"`
	Status         string `json:"status"`
	IdempotencyKey string `json:"idempotency_key"`
}
