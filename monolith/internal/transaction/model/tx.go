package model

import "time"

type Transaction struct {
	ID               string    `json:"id"`
	SenderWalletID   *string   `json:"sender_wallet_id"` // nullable if top up
	ReceiverWalletID string    `json:"receiver_wallet_id"`
	Amount           float64   `json:"amount"`
	Description      string    `json:"description"`
	IdempotencyKey   string    `json:"idempotency_key"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type TransferRequest struct {
	ReceiverEmail  string  `json:"receiver_email" binding:"required,email"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	Description    string  `json:"description"`
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
}
