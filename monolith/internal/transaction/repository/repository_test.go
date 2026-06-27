package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_CreateTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLTransactionRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").
		WithArgs("tx1", "w1", "w2", decimal.NewFromInt(100), "transfer", "idem1", "SUCCESS").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	sender := "w1"
	txn := &model.Transaction{
		ID:               "tx1",
		SenderWalletID:   &sender,
		ReceiverWalletID: "w2",
		Amount:           decimal.NewFromInt(100),
		Description:      "transfer",
		IdempotencyKey:   "idem1",
		Status:           "SUCCESS",
	}

	err = repo.CreateTx(context.Background(), tx, txn)
	assert.NoError(t, err)

	tx.Commit()
}

func TestTransaction_GetByIdempotencyKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLTransactionRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "sender_wallet_id", "receiver_wallet_id", "amount", "description", "idempotency_key", "status", "created_at"}).
		AddRow("tx1", "w1", "w2", "100.00", "transfer", "idem1", "SUCCESS", now)

	mock.ExpectQuery("SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at FROM transactions WHERE idempotency_key = ?").
		WithArgs("idem1").
		WillReturnRows(rows)

	res, err := repo.GetByIdempotencyKey(context.Background(), "idem1")
	assert.NoError(t, err)
	assert.Equal(t, "tx1", res.ID)
	assert.Equal(t, "w1", *res.SenderWalletID)
}

func TestTransaction_GetHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLTransactionRepository(db)

	now := time.Now()

	// Case 1: without status filter
	mock.ExpectQuery("SELECT COUNT").WithArgs("w1", "w1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rows := sqlmock.NewRows([]string{"id", "sender_wallet_id", "receiver_wallet_id", "amount", "description", "idempotency_key", "status", "created_at"}).
		AddRow("tx1", "w1", "w2", "100.00", "transfer", "idem1", "SUCCESS", now)
	mock.ExpectQuery("SELECT id, sender_wallet_id").WithArgs("w1", "w1", 10, 0).WillReturnRows(rows)

	res, total, err := repo.GetHistory(context.Background(), "w1", model.PaginationParams{Page: 1, Limit: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, res, 1)

	// Case 2: with status filter
	mock.ExpectQuery("SELECT COUNT").WithArgs("w1", "w1", "SUCCESS").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rows2 := sqlmock.NewRows([]string{"id", "sender_wallet_id", "receiver_wallet_id", "amount", "description", "idempotency_key", "status", "created_at"}).
		AddRow("tx1", "w1", "w2", "100.00", "transfer", "idem1", "SUCCESS", now)
	mock.ExpectQuery("SELECT id, sender_wallet_id").WithArgs("w1", "w1", "SUCCESS", 10, 0).WillReturnRows(rows2)

	res, total, err = repo.GetHistory(context.Background(), "w1", model.PaginationParams{Page: 1, Limit: 10, Status: "SUCCESS"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, res, 1)
}
