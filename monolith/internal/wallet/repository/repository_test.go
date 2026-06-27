package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestWallet_CreateTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLWalletRepository(db)

	w := &model.Wallet{
		ID:       "w1",
		UserID:   "u1",
		Balance:  decimal.NewFromInt(100),
		Currency: "USD",
		Status:   "active",
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO wallets").
		WithArgs("w1", "u1", decimal.NewFromInt(100), "USD", "active").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	err = repo.CreateTx(context.Background(), tx, w)
	assert.NoError(t, err)

	tx.Commit()
}

func TestWallet_GetByUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLWalletRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "balance", "currency", "status", "version", "created_at", "updated_at"}).
		AddRow("w1", "u1", "100.00", "USD", "active", 1, now, now)

	mock.ExpectQuery("SELECT id, user_id, balance, currency, status, version, created_at, updated_at FROM wallets").
		WithArgs("u1").
		WillReturnRows(rows)

	res, err := repo.GetByUserID(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Equal(t, "w1", res.ID)
}

func TestWallet_UpdateBalanceTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLWalletRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WithArgs(decimal.NewFromInt(50), "w1", 1, decimal.NewFromInt(50)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	err = repo.UpdateBalanceTx(context.Background(), tx, "w1", decimal.NewFromInt(50), 1)
	assert.NoError(t, err)

	tx.Commit()
}
