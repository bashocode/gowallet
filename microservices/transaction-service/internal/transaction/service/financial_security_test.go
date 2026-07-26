package service_test

import (
	"context"
	"testing"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestFinancialSecurity_InvalidAmount(t *testing.T) {
	svc := service.NewTransactionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", "")

	// Negative amount test
	_, err := svc.Transfer(context.Background(), "user-sender-1", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromFloat(-100.0),
		IdempotencyKey: "key-invalid-amount",
	})
	assert.Error(t, err, "negative amount must be rejected")
	assert.Contains(t, err.Error(), "Amount must be greater than zero.")

	// Zero amount test
	_, err = svc.Transfer(context.Background(), "user-sender-1", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.Zero,
		IdempotencyKey: "key-zero-amount",
	})
	assert.Error(t, err, "zero amount must be rejected")
	assert.Contains(t, err.Error(), "Amount must be greater than zero.")
}
