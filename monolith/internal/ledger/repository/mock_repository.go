package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
	"github.com/stretchr/testify/mock"
)

type MockLedgerRepository struct {
	mock.Mock
}

func (m *MockLedgerRepository) CreateTx(ctx context.Context, tx *sql.Tx, entry *model.LedgerEntry) error {
	args := m.Called(ctx, tx, entry)
	return args.Error(0)
}

func (m *MockLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (float64, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockLedgerRepository) GetEntriesByWalletID(ctx context.Context, walletID string) ([]model.LedgerEntry, error) {
	args := m.Called(ctx, walletID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.LedgerEntry), args.Error(1)
}
