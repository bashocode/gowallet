package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Wallet struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Balance   decimal.Decimal `json:"balance"`
	Currency  string          `json:"currency"`
	Status    string          `json:"status"` // active, frozen
	Version   int             `json:"-"`      // used for concurrency control
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type WalletInquiry struct {
	Valid     bool   `json:"valid"`
	AccountID string `json:"account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type EmailInquiryRequest struct {
	Email string `json:"email" binding:"required,email" example:"recipient@example.com"`
}
