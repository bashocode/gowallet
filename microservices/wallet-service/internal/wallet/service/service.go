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
	// 1. Cek Cache Redis DULU berdasarkan userID
	cachedWallet, err := s.cacheRepo.GetWalletByUserID(ctx, userID)
	if err == nil && cachedWallet != nil {
		logger.Log.InfoContext(ctx, "[Cache Hit] Retrieved wallet from Redis",
			slog.String("user_id", userID),
			slog.String("wallet_id", cachedWallet.ID))
		return cachedWallet, nil
	}

	if !errors.Is(err, redis.Nil) && err != nil {
		logger.Log.WarnContext(ctx, "[Cache Miss] Redis error, falling back to DB",
			slog.String("user_id", userID),
			slog.String("error", err.Error()))
	} else {
		logger.Log.InfoContext(ctx, "[Cache Miss] Key not found in Redis, reading from DB",
			slog.String("user_id", userID))
	}

	// 2. Query DB HANYA 1 KALI saat cache miss
	dbWallet, err := s.dbRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. Simpan wallet ke Redis
	_ = s.cacheRepo.SetWalletByUserID(ctx, userID, dbWallet, 5*time.Minute)
	logger.Log.InfoContext(ctx, "[Cache Set] Stored wallet in Redis with TTL 5m",
		slog.String("user_id", userID),
		slog.String("wallet_id", dbWallet.ID))

	return dbWallet, nil
}

func (s *walletService) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	return s.dbRepo.UpdateBalanceWithOwnerCheck(ctx, userID, amount, expectedVersion)
}

func (s *walletService) Create(ctx context.Context, w *model.Wallet) error {
	return s.dbRepo.Create(ctx, w)
}

func (s *walletService) ReconcileAll(ctx context.Context) (int, int, error) {
	return s.dbRepo.ReconcileAll(ctx)
}
