package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/stretchr/testify/assert"
)

func TestGetWalletByUserID_Success(t *testing.T) {
	mockRepo := new(repository.MockWalletRepository)
	svc := NewWalletService(mockRepo)

	ctx := context.TODO()
	userID := "user-123"
	expectedWallet := &model.Wallet{
		ID:       "wallet-123",
		UserID:   userID,
		Balance:  1000.0,
		Currency: "IDR",
		Status:   "active",
	}

	mockRepo.On("GetByUserID", ctx, userID).Return(expectedWallet, nil)

	wallet, err := svc.GetWalletByUserID(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedWallet, wallet)
	mockRepo.AssertExpectations(t)
}

func TestGetWalletByUserID_NotFound(t *testing.T) {
	mockRepo := new(repository.MockWalletRepository)
	svc := NewWalletService(mockRepo)

	ctx := context.TODO()
	userID := "user-123"

	mockRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("not found"))

	wallet, err := svc.GetWalletByUserID(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, wallet)
	mockRepo.AssertExpectations(t)
}
