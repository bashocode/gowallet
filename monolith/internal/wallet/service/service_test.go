package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/go-redis/redismock/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitLogger()
}

func TestGetWalletByUserID_Success(t *testing.T) {
	mockRepo := new(repository.MockWalletRepository)
	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()
	svc := NewWalletService(mockRepo, rdb)

	ctx := context.TODO()
	userID := "user-123"
	expectedWallet := &model.Wallet{
		ID:       "wallet-123",
		UserID:   userID,
		Balance:  decimal.NewFromInt(1000),
		Currency: "IDR",
		Status:   "active",
	}
	expectedWalletBytes, _ := json.Marshal(expectedWallet)

	// Mock Redis cache miss
	mockRedis.ExpectGet("wallet:user:user-123").RedisNil()

	// Mock DB success
	mockRepo.On("GetByUserID", ctx, userID).Return(expectedWallet, nil)

	// Mock Redis cache save
	mockRedis.ExpectSet("wallet:user:user-123", expectedWalletBytes, 5*time.Minute).SetVal("OK")

	wallet, err := svc.GetWalletByUserID(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedWallet, wallet)
	mockRepo.AssertExpectations(t)
	assert.NoError(t, mockRedis.ExpectationsWereMet())
}

func TestGetWalletByUserID_NotFound(t *testing.T) {
	mockRepo := new(repository.MockWalletRepository)
	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()
	svc := NewWalletService(mockRepo, rdb)

	ctx := context.TODO()
	userID := "user-123"

	// Mock Redis cache miss
	mockRedis.ExpectGet("wallet:user:user-123").RedisNil()

	// Mock DB failure
	mockRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("not found"))

	wallet, err := svc.GetWalletByUserID(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, wallet)
	mockRepo.AssertExpectations(t)
	assert.NoError(t, mockRedis.ExpectationsWereMet())
}

func TestGetWalletByUserID_CacheHit(t *testing.T) {
	mockRepo := new(repository.MockWalletRepository)
	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()
	svc := NewWalletService(mockRepo, rdb)

	ctx := context.TODO()
	userID := "user-123"
	expectedWallet := &model.Wallet{
		ID:       "wallet-123",
		UserID:   userID,
		Balance:  decimal.NewFromInt(1000),
		Currency: "IDR",
		Status:   "active",
	}
	expectedWalletBytes, _ := json.Marshal(expectedWallet)

	// Mock Redis cache hit
	mockRedis.ExpectGet("wallet:user:user-123").SetVal(string(expectedWalletBytes))

	// DB should NOT be called (since it's a cache hit)

	wallet, err := svc.GetWalletByUserID(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedWallet, wallet)
	mockRepo.AssertExpectations(t)
	assert.NoError(t, mockRedis.ExpectationsWereMet())
}
