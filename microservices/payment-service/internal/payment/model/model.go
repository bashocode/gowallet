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

type PaymentGatewayCallback struct {
	OrderID        string  `json:"order_id"`
	UserID         string  `json:"user_id"`
	Amount         float64 `json:"amount"`
	PaymentStatus  string  `json:"payment_status"`  // settled, pending, deny
	PaymentGateway string  `json:"payment_gateway"` // midtrans, stripe
}
