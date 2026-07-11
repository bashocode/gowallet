package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/stretchr/testify/mock"
)

type MockTransactionRepository struct {
	mock.Mock
}

func (m *MockTransactionRepository) CreateTx(ctx context.Context, tx *sql.Tx, t *model.Transaction) error {
	args := m.Called(ctx, tx, t)
	return args.Error(0)
}

func (m *MockTransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) GetHistory(ctx context.Context, walletID string, params model.PaginationParams) ([]model.Transaction, int64, error) {
	args := m.Called(ctx, walletID, params)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]model.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockTransactionRepository) GetByID(ctx context.Context, id string) (*model.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transaction), args.Error(1)
}
