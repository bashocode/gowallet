package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCreateTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMysqlLedgerRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ledger_entries").
		WithArgs("1", "w1", "t1", "credit", decimal.NewFromInt(100)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	entry := &model.LedgerEntry{
		ID:            "1",
		WalletID:      "w1",
		TransactionID: "t1",
		EntryType:     "credit",
		Amount:        decimal.NewFromInt(100),
	}

	err = repo.CreateTx(context.Background(), tx, entry)
	assert.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)
}

func TestGetBalanceByWalletID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMysqlLedgerRepository(db)

	rows := sqlmock.NewRows([]string{"balance"}).AddRow("75.50")
	mock.ExpectQuery("SELECT").WithArgs("w1").WillReturnRows(rows)

	balance, err := repo.GetBalanceByWalletID(context.Background(), "w1")
	assert.NoError(t, err)
	assert.True(t, balance.Equal(decimal.NewFromFloat(75.50)))
}

func TestGetEntriesByWalletID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMysqlLedgerRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "wallet_id", "transaction_id", "entry_type", "amount", "created_at"}).
		AddRow("1", "w1", "t1", "credit", "100.00", now)
	mock.ExpectQuery("SELECT").WithArgs("w1").WillReturnRows(rows)

	entries, err := repo.GetEntriesByWalletID(context.Background(), "w1")
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "1", entries[0].ID)
}
