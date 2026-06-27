package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestLedgerEntryJSON(t *testing.T) {
	now := time.Now()
	entry := LedgerEntry{
		ID:            "1",
		WalletID:      "100",
		TransactionID: "500",
		EntryType:     "credit",
		Amount:        decimal.NewFromFloat(150.50),
		CreatedAt:     now,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal LedgerEntry: %v", err)
	}

	var decoded LedgerEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal LedgerEntry: %v", err)
	}

	if decoded.ID != entry.ID || decoded.WalletID != entry.WalletID || decoded.TransactionID != entry.TransactionID || decoded.EntryType != entry.EntryType || !decoded.Amount.Equal(entry.Amount) {
		t.Errorf("decoded entry does not match original: %+v vs %+v", decoded, entry)
	}
}
