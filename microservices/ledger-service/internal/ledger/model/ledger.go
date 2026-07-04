package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type LedgerEntry struct {
	ID            string          `json:"id"`
	WalletID      string          `json:"wallet_id"`
	TransactionID string          `json:"transaction_id"`
	EntryType     string          `json:"entry_type"` // credit or debit
	Amount        decimal.Decimal `json:"amount"`
	CreatedAt     time.Time       `json:"created_at"`
}
