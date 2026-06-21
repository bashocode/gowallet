package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
	ledgerRepo "github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	walletModel "github.com/bashocode/gowallet/monolith/internal/wallet/model"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/stretchr/testify/assert"
)

func TestReconcileWalletBalance_Consistent(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"
	wallet := &walletModel.Wallet{
		ID:      "wallet-123",
		Balance: 500.0,
	}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)
	mockLedgerRepo.On("GetBalanceByWalletID", ctx, wallet.ID).Return(500.0, nil)

	isConsistent, walletBalance, calculatedBalance, err := svc.ReconcileWalletBalance(ctx, userID)

	assert.NoError(t, err)
	assert.True(t, isConsistent)
	assert.Equal(t, 500.0, walletBalance)
	assert.Equal(t, 500.0, calculatedBalance)
	mockWalletRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
}

func TestReconcileWalletBalance_Discrepancy(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"
	wallet := &walletModel.Wallet{
		ID:      "wallet-123",
		Balance: 500.0,
	}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)
	mockLedgerRepo.On("GetBalanceByWalletID", ctx, wallet.ID).Return(450.0, nil)

	isConsistent, walletBalance, calculatedBalance, err := svc.ReconcileWalletBalance(ctx, userID)

	assert.NoError(t, err)
	assert.False(t, isConsistent)
	assert.Equal(t, 500.0, walletBalance)
	assert.Equal(t, 450.0, calculatedBalance)
}

func TestReconcileWalletBalance_WalletError(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("db error"))

	isConsistent, _, _, err := svc.ReconcileWalletBalance(ctx, userID)

	assert.Error(t, err)
	assert.False(t, isConsistent)
}

func TestReconcileWalletBalance_LedgerError(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"
	wallet := &walletModel.Wallet{
		ID:      "wallet-123",
		Balance: 500.0,
	}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)
	mockLedgerRepo.On("GetBalanceByWalletID", ctx, wallet.ID).Return(0.0, errors.New("ledger error"))

	isConsistent, _, _, err := svc.ReconcileWalletBalance(ctx, userID)

	assert.Error(t, err)
	assert.False(t, isConsistent)
}

func TestGetMutationHistory_Success(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"
	wallet := &walletModel.Wallet{
		ID: "wallet-123",
	}
	expectedEntries := []model.LedgerEntry{
		{ID: "entry-1", WalletID: "wallet-123", EntryType: "credit", Amount: 100},
	}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)
	mockLedgerRepo.On("GetEntriesByWalletID", ctx, wallet.ID).Return(expectedEntries, nil)

	entries, err := svc.GetMutationHistory(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedEntries, entries)
}

func TestGetMutationHistory_WalletError(t *testing.T) {
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewLedgerService(mockLedgerRepo, mockWalletRepo)

	ctx := context.TODO()
	userID := "user-123"

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("db error"))

	entries, err := svc.GetMutationHistory(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, entries)
}
