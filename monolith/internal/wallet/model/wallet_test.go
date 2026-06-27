package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestWalletJSON(t *testing.T) {
	now := time.Now()
	w := Wallet{
		ID:        "w1",
		UserID:    "u1",
		Balance:   decimal.NewFromInt(500),
		Currency:  "USD",
		Status:    "active",
		Version:   5,
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(w)
	assert.NoError(t, err)

	// Verify Version is ignored by JSON
	assert.False(t, strings.Contains(string(data), `"version"`))

	var decoded Wallet
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, w.ID, decoded.ID)
	assert.Equal(t, w.Status, decoded.Status)
}
