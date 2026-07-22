package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/cache"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type WalletService interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	Create(ctx context.Context, w *model.Wallet) error
	ReconcileAll(ctx context.Context) (mismatches int, total int, err error)
}

type walletService struct {
	dbRepo    repository.WalletRepository
	cacheRepo cache.WalletCacheRepository
}

func NewWalletService(dbRepo repository.WalletRepository, cacheRepo cache.WalletCacheRepository) WalletService {
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

	balance, err := s.cacheRepo.GetBalance(ctx, wallet.ID)
	if err == nil {
		logger.Log.InfoContext(ctx, "[Cache Hit] Retrieved balance from Redis",
			slog.String("wallet_id", wallet.ID),
			slog.String("cached_balance", balance))

		balanceDecimal, parseErr := decimal.NewFromString(balance)
		if parseErr == nil {
			wallet.Balance = balanceDecimal
			return wallet, nil
		}
		logger.Log.WarnContext(ctx, "[Cache] Failed to parse cached balance, falling back to DB",
			slog.String("wallet_id", wallet.ID),
			slog.String("error", parseErr.Error()))
	}

	if !errors.Is(err, redis.Nil) && err != nil {
		logger.Log.WarnContext(ctx, "[Cache Miss] Redis error, falling back to DB",
			slog.String("wallet_id", wallet.ID),
			slog.String("error", err.Error()))
	} else {
		logger.Log.InfoContext(ctx, "[Cache Miss] Key not found in Redis, reading from DB",
			slog.String("wallet_id", wallet.ID))
	}

	dbWallet, err := s.dbRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	_ = s.cacheRepo.SetBalance(ctx, dbWallet.ID, dbWallet.Balance.String(), 5*time.Minute)
	logger.Log.InfoContext(ctx, "[Cache Set] Stored balance in Redis with TTL 5m",
		slog.String("wallet_id", dbWallet.ID),
		slog.String("balance", dbWallet.Balance.String()))

	return dbWallet, nil
}

func (s *walletService) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	wallet, err := s.dbRepo.UpdateBalanceWithOwnerCheck(ctx, userID, amount, expectedVersion)
	if err != nil {
		return nil, err
	}

	_ = s.cacheRepo.DeleteBalance(ctx, wallet.ID)
	logger.Log.InfoContext(ctx, "[Cache Invalidation] Deleted balance cache after update",
		slog.String("wallet_id", wallet.ID))

	return wallet, nil
}

func (s *walletService) Create(ctx context.Context, w *model.Wallet) error {
	return s.dbRepo.Create(ctx, w)
}

func (s *walletService) ReconcileAll(ctx context.Context) (int, int, error) {
	return s.dbRepo.ReconcileAll(ctx)
}
