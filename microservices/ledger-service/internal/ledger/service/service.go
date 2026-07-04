package service

import (
	"context"

	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/model"
	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/repository"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/shopspring/decimal"
)

type LedgerService interface {
	GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error)
	ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error)
}

type ledgerService struct {
	ledgerRepo   repository.LedgerRepository
	walletClient pbWallet.WalletServiceClient
}

func NewLedgerService(lRepo repository.LedgerRepository, wClient pbWallet.WalletServiceClient) LedgerService {
	return &ledgerService{
		ledgerRepo:   lRepo,
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

	walletBalance := decimal.NewFromFloat(wallet.GetBalance())

	calculatedBalance, err := s.ledgerRepo.GetBalanceByWalletID(ctx, wallet.GetId())
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	isConsistent := walletBalance.Equal(calculatedBalance)
	return isConsistent, walletBalance, calculatedBalance, nil
}
