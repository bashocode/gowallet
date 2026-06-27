package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationParamsOffset(t *testing.T) {
	p := PaginationParams{Page: 1, Limit: 10}
	assert.Equal(t, 0, p.Offset())

	p2 := PaginationParams{Page: 3, Limit: 15}
	assert.Equal(t, 30, p2.Offset())
}

func TestTransactionJSON(t *testing.T) {
	sender := "sender_id"
	tx := Transaction{
		ID:               "tx_1",
		SenderWalletID:   &sender,
		ReceiverWalletID: "receiver_id",
		Status:           "SUCCESS",
	}

	data, err := json.Marshal(tx)
	assert.NoError(t, err)

	var decoded Transaction
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, decoded.ID)
	assert.Equal(t, *tx.SenderWalletID, *decoded.SenderWalletID)
	assert.Equal(t, tx.ReceiverWalletID, decoded.ReceiverWalletID)
}
