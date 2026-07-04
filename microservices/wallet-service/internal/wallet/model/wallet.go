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
	Status    string          `json:"status"`
	Version   int32           `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
