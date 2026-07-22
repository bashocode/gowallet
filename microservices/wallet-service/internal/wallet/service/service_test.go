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
	getByUserIDFunc                 func(ctx context.Context, userID string) (*model.Wallet, error)
	updateBalanceWithOwnerCheckFunc func(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	createFunc                      func(ctx context.Context, w *model.Wallet) error
	reconcileAllFunc                func(ctx context.Context) (int, int, error)
	getByUserIDCallCount            int
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
	getWalletByUserIDFunc    func(ctx context.Context, userID string) (*model.Wallet, error)
	setWalletByUserIDFunc    func(ctx context.Context, userID string, wallet *model.Wallet, ttl time.Duration) error
	deleteWalletByUserIDFunc func(ctx context.Context, userID string) error
	deleteWalletByIDFunc     func(ctx context.Context, walletID string) error
}

func (m *mockCacheRepo) GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	if m.getWalletByUserIDFunc != nil {
		return m.getWalletByUserIDFunc(ctx, userID)
	}
	return nil, redis.Nil
}

func (m *mockCacheRepo) SetWalletByUserID(ctx context.Context, userID string, wallet *model.Wallet, ttl time.Duration) error {
	if m.setWalletByUserIDFunc != nil {
		return m.setWalletByUserIDFunc(ctx, userID, wallet, ttl)
	}
	return nil
}

func (m *mockCacheRepo) DeleteWalletByUserID(ctx context.Context, userID string) error {
	if m.deleteWalletByUserIDFunc != nil {
		return m.deleteWalletByUserIDFunc(ctx, userID)
	}
	return nil
}

func (m *mockCacheRepo) DeleteWalletByID(ctx context.Context, walletID string) error {
	if m.deleteWalletByIDFunc != nil {
		return m.deleteWalletByIDFunc(ctx, walletID)
	}
	return nil
}

func TestGetByUserID_CacheHit(t *testing.T) {
	mockDB := &mockWalletRepo{
		getByUserIDFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			t.Errorf("expected DB not to be called on cache hit")
			return nil, errors.New("db should not be called")
		},
	}

	mockCache := &mockCacheRepo{
		getWalletByUserIDFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			if userID == "user-1" {
				return &model.Wallet{
					ID:      "wallet-123",
					UserID:  userID,
					Balance: decimal.NewFromInt(100000),
				}, nil
			}
			return nil, redis.Nil
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

	if mockDB.getByUserIDCallCount != 0 {
		t.Errorf("expected DB not to be called on cache hit, got %d calls", mockDB.getByUserIDCallCount)
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
		getWalletByUserIDFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			return nil, redis.Nil
		},
		setWalletByUserIDFunc: func(ctx context.Context, userID string, wallet *model.Wallet, ttl time.Duration) error {
			if wallet.Balance.String() != "75000" {
				t.Errorf("expected to cache balance 75000, got %s", wallet.Balance.String())
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

	if callCount != 1 {
		t.Errorf("expected DB to be called exactly once on cache miss, got %d calls", callCount)
	}
}

func TestUpdateBalanceWithOwnerCheck(t *testing.T) {
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

	mockCache := &mockCacheRepo{}

	svc := NewWalletService(mockDB, mockCache)
	updatedWallet, err := svc.UpdateBalanceWithOwnerCheck(context.Background(), "user-3", decimal.NewFromInt(50000), 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if updatedWallet.Balance.String() != "150000" {
		t.Errorf("expected updated balance 150000, got %s", updatedWallet.Balance.String())
	}
}
