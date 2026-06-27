package service

import (
	"context"

	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
	"github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/shopspring/decimal"
)

type LedgerService interface {
	ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error)
	GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error)
}

type ledgerService struct {
	ledgerRepo repository.LedgerRepository
	walletRepo walletRepo.WalletRepository
}

func NewLedgerService(lRepo repository.LedgerRepository, wRepo walletRepo.WalletRepository) LedgerService {
	return &ledgerService{
		ledgerRepo: lRepo,
		walletRepo: wRepo,
	}
}

func (s *ledgerService) ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error) {
	// get the user wallet data
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	// get the sum of all entries
	calculatedBalance, err := s.ledgerRepo.GetBalanceByWalletID(ctx, wallet.ID)
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}

	// check if there is a discrepancy
	isConsistent := wallet.Balance.Equal(calculatedBalance)
	return isConsistent, wallet.Balance, calculatedBalance, nil
}

func (s *ledgerService) GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ledgerRepo.GetEntriesByWalletID(ctx, wallet.ID)
}
