package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/cache"
	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/model"
	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type LedgerService interface {
	GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error)
	ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error)
	Create(ctx context.Context, entry *model.LedgerEntry) error
	CreateBatch(ctx context.Context, entries []*model.LedgerEntry) error
}

type ledgerService struct {
	ledgerRepo   repository.LedgerRepository
	cacheRepo    cache.LedgerCacheRepository
	walletClient pbWallet.WalletServiceClient
}

func NewLedgerService(lRepo repository.LedgerRepository, cacheRepo cache.LedgerCacheRepository, wClient pbWallet.WalletServiceClient) LedgerService {
	return &ledgerService{
		ledgerRepo:   lRepo,
		cacheRepo:    cacheRepo,
		walletClient: wClient,
	}
}

func (s *ledgerService) GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error) {
	wallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return s.ledgerRepo.GetEntriesByWalletID(ctx, wallet.GetId())
}

func (s *ledgerService) ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error) {
	wallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	walletBalance, err := decimal.NewFromString(wallet.GetBalance())
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	balance, err := s.cacheRepo.GetBalance(ctx, wallet.GetId())
	if err == nil {
		logger.Log.InfoContext(ctx, "[Cache Hit] Retrieved ledger balance from Redis",
			slog.String("wallet_id", wallet.GetId()),
			slog.String("cached_balance", balance))

		calculatedBalance, parseErr := decimal.NewFromString(balance)
		if parseErr == nil {
			isConsistent := walletBalance.Equal(calculatedBalance)
			return isConsistent, walletBalance, calculatedBalance, nil
		}
	}

	if !errors.Is(err, redis.Nil) && err != nil {
		logger.Log.WarnContext(ctx, "[Cache] Redis error, falling back to DB",
			slog.String("wallet_id", wallet.GetId()),
			slog.String("error", err.Error()))
	} else {
		logger.Log.InfoContext(ctx, "[Cache Miss] Ledger balance not in Redis, calculating from DB",
			slog.String("wallet_id", wallet.GetId()))
	}

	calculatedBalance, err := s.ledgerRepo.GetBalanceByWalletID(ctx, wallet.GetId())
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	_ = s.cacheRepo.SetBalance(ctx, wallet.GetId(), calculatedBalance.String(), 5*time.Minute)
	logger.Log.InfoContext(ctx, "[Cache Set] Stored ledger balance in Redis with TTL 5m",
		slog.String("wallet_id", wallet.GetId()),
		slog.String("balance", calculatedBalance.String()))

	isConsistent := walletBalance.Equal(calculatedBalance)
	return isConsistent, walletBalance, calculatedBalance, nil
}

func (s *ledgerService) Create(ctx context.Context, entry *model.LedgerEntry) error {
	return s.ledgerRepo.Create(ctx, entry)
}

func (s *ledgerService) CreateBatch(ctx context.Context, entries []*model.LedgerEntry) error {
	return s.ledgerRepo.CreateBatch(ctx, entries)
}
