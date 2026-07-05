package service

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	pbUser "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransactionService interface {
	Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error)
	GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error)
	TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error)
}

type transactionService struct {
	db           *sql.DB
	txRepo       repository.TransactionRepository
	userClient   pbUser.UserServiceClient
	walletClient pbWallet.WalletServiceClient
	ledgerClient pbLedger.LedgerServiceClient
}

func NewTransactionService(
	db *sql.DB,
	txRepo repository.TransactionRepository,
	userClient pbUser.UserServiceClient,
	walletClient pbWallet.WalletServiceClient,
	ledgerClient pbLedger.LedgerServiceClient,
) TransactionService {
	return &transactionService{
		db:           db,
		txRepo:       txRepo,
		userClient:   userClient,
		walletClient: walletClient,
		ledgerClient: ledgerClient,
	}
}

func (s *transactionService) Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error) {
	// 1. Idempotency check — FIRST, before any gRPC call
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// 2. Validate receiver exists
	receiverUser, err := s.userClient.GetUserByEmail(ctx, &pbUser.GetUserByEmailRequest{Email: req.ReceiverEmail})
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
	}

	// 3. Get sender and receiver wallets
	senderWallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}

	receiverWallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: receiverUser.Id})
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found")
	}

	// Sender and receiver cannot be the same
	if senderWallet.Id == receiverWallet.Id {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_TRANSFER", "Cannot transfer to self account")
	}

	// Validate sender balance (compare local copy before mutation)
	senderBalance, err := decimal.NewFromString(senderWallet.GetBalance())
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	if senderBalance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance")
	}

	// 4. Create transaction record with status "pending"
	transactionID := uuid.New().String()
	transaction := &model.Transaction{
		ID:               transactionID,
		SenderWalletID:   &senderWallet.Id,
		ReceiverWalletID: receiverWallet.Id,
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "pending",
	}
	if err := s.txRepo.Create(ctx, transaction); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 5. Saga: Debit sender (negative amount = reduce balance)
	debitAmount := req.Amount.Neg()
	_, err = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
		UserId:          senderUserID,
		Amount:          debitAmount.String(),
		ExpectedVersion: senderWallet.Version,
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		return nil, customErr.NewAppError(http.StatusConflict, "TRANSFER_FAILED", "Debit failed, please try again")
	}

	// 6. Saga: Credit receiver (positive amount = add balance)
	_, err = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
		UserId:          receiverUser.Id,
		Amount:          req.Amount.String(),
		ExpectedVersion: receiverWallet.Version,
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		logger.Log.Error("Partial saga failure: credit failed after debit succeeded",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
		return nil, customErr.NewAppError(http.StatusConflict, "TRANSFER_FAILED", "Credit failed, please try again")
	}

	// 7. Saga: Record ledger entries (debit + credit)
	_, err = s.ledgerClient.RecordLedgerEntries(ctx, &pbLedger.RecordEntriesRequest{
		Entries: []*pbLedger.RecordEntryRequest{
			{
				TransactionId: transactionID,
				WalletId:      senderWallet.Id,
				Type:          "debit",
				Amount:        req.Amount.String(),
			},
			{
				TransactionId: transactionID,
				WalletId:      receiverWallet.Id,
				Type:          "credit",
				Amount:        req.Amount.String(),
			},
		},
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		logger.Log.Error("Partial saga failure: ledger record failed after wallet updates",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
		return nil, customErr.NewAppError(http.StatusConflict, "TRANSFER_FAILED", "Ledger record failed, please try again")
	}

	// 8. Mark transaction as success
	if err := s.txRepo.UpdateStatus(ctx, transactionID, "success"); err != nil {
		logger.Log.Error("Failed to update transaction status to success",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
	}
	transaction.Status = "success"

	return transaction, nil
}

func (s *transactionService) GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	// Get user's wallet via gRPC
	wallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
	if err != nil {
		return nil, nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found")
	}

	txs, total, err := s.txRepo.GetHistory(ctx, wallet.Id, params)
	if err != nil {
		return nil, nil, customErr.ErrInternalServer
	}

	totalPages := int(total / int64(params.Limit))
	if total%int64(params.Limit) != 0 {
		totalPages++
	}

	meta := &model.PaginationMeta{
		Page:      params.Page,
		Limit:     params.Limit,
		Total:     total,
		TotalPage: totalPages,
	}

	return txs, meta, nil
}

func (s *transactionService) TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error) {
	// 1. Idempotency check
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// 2. Get user's wallet
	wallet, err := s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found")
	}

	// 3. Create transaction record with status "pending"
	transactionID := uuid.New().String()
	transaction := &model.Transaction{
		ID:               transactionID,
		SenderWalletID:   nil, // nil for top-up
		ReceiverWalletID: wallet.Id,
		Amount:           req.Amount,
		Description:      "Top Up",
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "pending",
	}
	if err := s.txRepo.Create(ctx, transaction); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 4. Saga: Credit wallet (positive amount = add balance)
	_, err = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
		UserId:          userID,
		Amount:          req.Amount.String(),
		ExpectedVersion: wallet.Version,
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		return nil, customErr.NewAppError(http.StatusConflict, "TOPUP_FAILED", "Wallet credit failed, please try again")
	}

	// 5. Saga: Record ledger entry (credit only)
	_, err = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
		TransactionId: transactionID,
		WalletId:      wallet.Id,
		Type:          "credit",
		Amount:        req.Amount.String(),
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		logger.Log.Error("Partial saga failure: ledger record failed after wallet credit",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
		return nil, customErr.NewAppError(http.StatusConflict, "TOPUP_FAILED", "Ledger record failed, please try again")
	}

	// 6. Mark transaction as success
	if err := s.txRepo.UpdateStatus(ctx, transactionID, "success"); err != nil {
		logger.Log.Error("Failed to update transaction status to success",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
	}
	transaction.Status = "success"

	return transaction, nil
}

func (s *transactionService) markFailed(ctx context.Context, transactionID string) {
	if err := s.txRepo.UpdateStatus(ctx, transactionID, "failed"); err != nil {
		logger.Log.Error("Failed to mark transaction as failed",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
	}
}
