package service

import (
	"context"
	"database/sql"
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	ledgerModel "github.com/bashocode/gowallet/monolith/internal/ledger/model"
	ledgerRepo "github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/bashocode/gowallet/monolith/internal/transaction/repository"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type TransactionService interface {
	Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error)
	GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error)
}

type transactionService struct {
	db         *sql.DB
	rdb        *redis.Client
	txRepo     repository.TransactionRepository
	userRepo   userRepo.UserRepository
	walletRepo walletRepo.WalletRepository
	ledgerRepo ledgerRepo.LedgerRepository
}

func NewTransactionService(
	db *sql.DB,
	rdb *redis.Client,
	txRepo repository.TransactionRepository,
	uRepo userRepo.UserRepository,
	wRepo walletRepo.WalletRepository,
	lRepo ledgerRepo.LedgerRepository,
) TransactionService {
	return &transactionService{
		db:         db,
		rdb:        rdb,
		txRepo:     txRepo,
		userRepo:   uRepo,
		walletRepo: wRepo,
		ledgerRepo: lRepo,
	}
}

func (s *transactionService) Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error) {
	// check idempotency key (this is for checking to not reprocess the same request)
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// look receiver by email
	receiverUser, err := s.userRepo.GetByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
	}

	// start db transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback()

	// look for sender and receiver wallet
	senderWallet, err := s.walletRepo.GetByUserID(ctx, senderUserID)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}

	receiverWallet, err := s.walletRepo.GetByUserID(ctx, receiverUser.ID)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found")
	}

	// sender & receiver cannot be the same
	if senderWallet.ID == receiverWallet.ID {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_TRANSFER", "Cannot transfer to self account")
	}

	// validate sender wallet balance is enough or not
	if senderWallet.Balance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance")
	}

	// Debit: amount POSITIVE (reduce balance)
	err = s.walletRepo.UpdateBalanceTx(ctx, tx, senderWallet.ID, req.Amount, senderWallet.Version)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusConflict, "CONCURRENCY_CONFLICT", "Transaksi sedang sibuk, silakan coba lagi nanti.")
	}

	// Credit: amount NEGATIVE (adding balance)
	err = s.walletRepo.UpdateBalanceTx(ctx, tx, receiverWallet.ID, req.Amount.Neg(), receiverWallet.Version)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusConflict, "CONCURRENCY_CONFLICT", "Transaksi sedang sibuk, silakan coba lagi nanti.")
	}

	// create data transaction record
	transactionID := uuid.New().String()
	transaction := &model.Transaction{
		ID:               transactionID,
		SenderWalletID:   &senderWallet.ID,
		ReceiverWalletID: receiverWallet.ID,
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
	}
	if err = s.txRepo.CreateTx(ctx, tx, transaction); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// create two ledger rows (debit for sender, and credit for receiver)
	debitEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      senderWallet.ID,
		TransactionID: transactionID,
		EntryType:     "debit",
		Amount:        req.Amount,
	}
	if err := s.ledgerRepo.CreateTx(ctx, tx, debitEntry); err != nil {
		return nil, customErr.ErrInternalServer
	}

	creditEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWallet.ID,
		TransactionID: transactionID,
		EntryType:     "credit",
		Amount:        req.Amount,
	}
	if err := s.ledgerRepo.CreateTx(ctx, tx, creditEntry); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// commit the db transaction
	if err := tx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// invalidate cache
	senderCacheKey := "wallet:user:" + senderUserID
	receiverCacheKey := "wallet:user:" + receiverUser.ID

	// delete the cache keys asynchronously (don't block HTTP response)
	go func() {
		s.rdb.Del(context.Background(), senderCacheKey, receiverCacheKey)
	}()

	return transaction, nil
}

func (s *transactionService) GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found")
	}

	// max limit
	if params.Limit > 100 {
		params.Limit = 100
	}

	txs, total, err := s.txRepo.GetHistory(ctx, wallet.ID, params)
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
