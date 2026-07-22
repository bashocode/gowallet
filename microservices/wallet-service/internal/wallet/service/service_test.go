package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type mockWalletRepo struct {
	getByUserIDFunc                   func(ctx context.Context, userID string) (*model.Wallet, error)
	updateBalanceWithOwnerCheckFunc   func(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	createFunc                        func(ctx context.Context, w *model.Wallet) error
	reconcileAllFunc                  func(ctx context.Context) (int, int, error)
	getByUserIDCallCount              int
}

func (m *mockWalletRepo) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	m.getByUserIDCallCount++
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockWalletRepo) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	if m.updateBalanceWithOwnerCheckFunc != nil {
		return m.updateBalanceWithOwnerCheckFunc(ctx, userID, amount, expectedVersion)
	}
	return nil, errors.New("not implemented")
}

func (m *mockWalletRepo) Create(ctx context.Context, w *model.Wallet) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, w)
	}
	return errors.New("not implemented")
}

func (m *mockWalletRepo) ReconcileAll(ctx context.Context) (int, int, error) {
	if m.reconcileAllFunc != nil {
		return m.reconcileAllFunc(ctx)
	}
	return 0, 0, errors.New("not implemented")
}

type mockCacheRepo struct {
	getBalanceFunc    func(ctx context.Context, walletID string) (string, error)
	setBalanceFunc    func(ctx context.Context, walletID string, balance string, ttl time.Duration) error
	deleteBalanceFunc func(ctx context.Context, walletID string) error
}

func (m *mockCacheRepo) GetBalance(ctx context.Context, walletID string) (string, error) {
	if m.getBalanceFunc != nil {
		return m.getBalanceFunc(ctx, walletID)
	}
	return "", redis.Nil
}

func (m *mockCacheRepo) SetBalance(ctx context.Context, walletID string, balance string, ttl time.Duration) error {
	if m.setBalanceFunc != nil {
		return m.setBalanceFunc(ctx, walletID, balance, ttl)
	}
	return nil
}

func (m *mockCacheRepo) DeleteBalance(ctx context.Context, walletID string) error {
	if m.deleteBalanceFunc != nil {
		return m.deleteBalanceFunc(ctx, walletID)
	}
	return nil
}

func TestGetByUserID_CacheHit(t *testing.T) {
	mockDB := &mockWalletRepo{
		getByUserIDFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			return &model.Wallet{
				ID:      "wallet-123",
				UserID:  userID,
				Balance: decimal.NewFromInt(50000),
			}, nil
		},
	}

	mockCache := &mockCacheRepo{
		getBalanceFunc: func(ctx context.Context, walletID string) (string, error) {
			if walletID == "wallet-123" {
				return "100000", nil
			}
			return "", redis.Nil
		},
	}

	svc := NewWalletService(mockDB, mockCache)
	wallet, err := svc.GetByUserID(context.Background(), "user-1")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if wallet.Balance.String() != "100000" {
		t.Errorf("expected balance from cache (100000), got %s", wallet.Balance.String())
	}

	if mockDB.getByUserIDCallCount != 1 {
		t.Errorf("expected DB to be called once to get wallet ID, got %d calls", mockDB.getByUserIDCallCount)
	}
}

func TestGetByUserID_CacheMiss(t *testing.T) {
	callCount := 0
	mockDB := &mockWalletRepo{
		getByUserIDFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			callCount++
			return &model.Wallet{
				ID:      "wallet-456",
				UserID:  userID,
				Balance: decimal.NewFromInt(75000),
			}, nil
		},
	}

	mockCache := &mockCacheRepo{
		getBalanceFunc: func(ctx context.Context, walletID string) (string, error) {
			return "", redis.Nil
		},
		setBalanceFunc: func(ctx context.Context, walletID string, balance string, ttl time.Duration) error {
			if balance != "75000" {
				t.Errorf("expected to cache balance 75000, got %s", balance)
			}
			if ttl != 5*time.Minute {
				t.Errorf("expected TTL of 5 minutes, got %v", ttl)
			}
			return nil
		},
	}

	svc := NewWalletService(mockDB, mockCache)
	wallet, err := svc.GetByUserID(context.Background(), "user-2")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if wallet.Balance.String() != "75000" {
		t.Errorf("expected balance 75000, got %s", wallet.Balance.String())
	}

	if callCount != 2 {
		t.Errorf("expected DB to be called twice (once for wallet ID, once for cache miss), got %d calls", callCount)
	}
}

func TestUpdateBalanceWithOwnerCheck_InvalidatesCache(t *testing.T) {
	deleteCalled := false

	mockDB := &mockWalletRepo{
		updateBalanceWithOwnerCheckFunc: func(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
			return &model.Wallet{
				ID:      "wallet-789",
				UserID:  userID,
				Balance: decimal.NewFromInt(150000),
				Version: 2,
			}, nil
		},
	}

	mockCache := &mockCacheRepo{
		deleteBalanceFunc: func(ctx context.Context, walletID string) error {
			if walletID != "wallet-789" {
				t.Errorf("expected to delete cache for wallet-789, got %s", walletID)
			}
			deleteCalled = true
			return nil
		},
	}

	svc := NewWalletService(mockDB, mockCache)
	_, err := svc.UpdateBalanceWithOwnerCheck(context.Background(), "user-3", decimal.NewFromInt(50000), 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !deleteCalled {
		t.Error("expected cache to be invalidated after balance update")
	}
}
