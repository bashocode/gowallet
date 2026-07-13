package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Payment struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	Amount          decimal.Decimal `json:"amount"`
	Currency        string          `json:"currency"`
	StripeSessionID string          `json:"stripe_session_id"`
	Status          string          `json:"status"` // 'pending', 'completed', 'failed', 'expired'
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type StripeCheckoutRequest struct {
	Amount   decimal.Decimal `json:"amount" binding:"required"`
	Currency string          `json:"currency" binding:"required"` // e.g. 'usd'
}

type StripeCheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}

type PaymentSettledEvent struct {
	EventID           string    `json:"event_id"`
	EventType         string    `json:"event_type"`
	Provider          string    `json:"provider"`
	ProviderPaymentID string    `json:"provider_payment_id"`
	PaymentID         string    `json:"payment_id"`
	UserID            string    `json:"user_id"`
	Amount            string    `json:"amount"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	OccurredAt        time.Time `json:"occurred_at"`
}

type OutboxEvent struct {
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
