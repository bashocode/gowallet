package service

import (
	"context"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

const balanceCacheTTL = 5 * time.Minute

type WalletService interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	Create(ctx context.Context, w *model.Wallet) error
	ReconcileAll(ctx context.Context) (mismatches int, total int, err error)
}

type walletService struct {
	dbRepo    repository.WalletRepository
	cacheRepo repository.WalletCacheRepository
}

func NewWalletService(dbRepo repository.WalletRepository, cacheRepo repository.WalletCacheRepository) WalletService {
	return &walletService{
		dbRepo:    dbRepo,
		cacheRepo: cacheRepo,
	}
}

func (s *walletService) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	wallet, err := s.dbRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	balanceInt := wallet.Balance.IntPart()
	_, cacheErr := s.cacheRepo.GetBalance(ctx, wallet.ID)
	if cacheErr != nil && cacheErr != redis.Nil {
		logger.Log.InfoContext(ctx, "[Cache Miss] Reading balance from MySQL", "wallet_id", wallet.ID, "user_id", userID)
		_ = s.cacheRepo.SetBalance(ctx, wallet.ID, balanceInt, balanceCacheTTL)
	} else if cacheErr == redis.Nil {
		logger.Log.InfoContext(ctx, "[Cache Miss] Reading balance from MySQL", "wallet_id", wallet.ID, "user_id", userID)
		_ = s.cacheRepo.SetBalance(ctx, wallet.ID, balanceInt, balanceCacheTTL)
	} else {
		logger.Log.InfoContext(ctx, "[Cache Hit] Reading balance from Redis", "wallet_id", wallet.ID, "user_id", userID)
	}

	return wallet, nil
}

func (s *walletService) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	wallet, err := s.dbRepo.UpdateBalanceWithOwnerCheck(ctx, userID, amount, expectedVersion)
	if err != nil {
		return nil, err
	}

	_ = s.cacheRepo.DeleteBalance(ctx, wallet.ID)
	logger.Log.InfoContext(ctx, "[Cache Invalidate] Balance updated, cache cleared", "wallet_id", wallet.ID, "user_id", userID)

	return wallet, nil
}

func (s *walletService) Create(ctx context.Context, w *model.Wallet) error {
	return s.dbRepo.Create(ctx, w)
}

func (s *walletService) ReconcileAll(ctx context.Context) (int, int, error) {
	return s.dbRepo.ReconcileAll(ctx)
}
