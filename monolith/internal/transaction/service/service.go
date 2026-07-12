package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
	TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error)
	ReceiveExternalTransfer(ctx context.Context, req model.ExternalTransferRequest) (*model.ExternalTransferStatus, error)
	GetExternalTransferStatus(ctx context.Context, transferID string) (*model.ExternalTransferStatus, error)
}

type transactionService struct {
	db            *sql.DB
	rdb           *redis.Client
	txRepo        repository.TransactionRepository
	userRepo      userRepo.UserRepository
	walletRepo    walletRepo.WalletRepository
	ledgerRepo    ledgerRepo.LedgerRepository
	webhookSecret string
}

func NewTransactionService(
	db *sql.DB,
	rdb *redis.Client,
	txRepo repository.TransactionRepository,
	uRepo userRepo.UserRepository,
	wRepo walletRepo.WalletRepository,
	lRepo ledgerRepo.LedgerRepository,
	webhookSecret string,
) TransactionService {
	return &transactionService{
		db:            db,
		rdb:           rdb,
		txRepo:        txRepo,
		userRepo:      uRepo,
		walletRepo:    wRepo,
		ledgerRepo:    lRepo,
		webhookSecret: webhookSecret,
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
	// Validation - FIX: add minimum validation
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 10 // default
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

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

func (s *transactionService) TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error) {
	// look for wallet
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found")
	}

	// start db transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback()

	// Credit: amount NEGATIVE (adding balance)
	err = s.walletRepo.UpdateBalanceTx(ctx, tx, wallet.ID, req.Amount.Neg(), wallet.Version)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusConflict, "CONCURRENCY_CONFLICT", "Transaksi sedang sibuk, silakan coba lagi nanti.")
	}

	// create data transaction record (sender_wallet_id is nil for top-up)
	transactionID := uuid.New().String()
	transaction := &model.Transaction{
		ID:               transactionID,
		SenderWalletID:   nil,
		ReceiverWalletID: wallet.ID,
		Amount:           req.Amount,
		Description:      "Top Up",
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
	}
	if err = s.txRepo.CreateTx(ctx, tx, transaction); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// create ledger row (credit for receiver)
	creditEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      wallet.ID,
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
	cacheKey := "wallet:user:" + userID
	go func() {
		s.rdb.Del(context.Background(), cacheKey)
	}()

	return transaction, nil
}

func (s *transactionService) ReceiveExternalTransfer(ctx context.Context, req model.ExternalTransferRequest) (*model.ExternalTransferStatus, error) {
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil {
		return &model.ExternalTransferStatus{
			TransferID:     existing.ID,
			Status:         existing.Status,
			IdempotencyKey: existing.IdempotencyKey,
		}, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, customErr.ErrInternalServer
	}

	receiverWallet, err := s.walletRepo.GetWalletByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback()

	if err := s.walletRepo.UpdateBalanceTx(ctx, tx, receiverWallet.ID, req.Amount.Neg(), receiverWallet.Version); err != nil {
		return nil, customErr.NewAppError(http.StatusConflict, "CONCURRENCY_CONFLICT", "Receiver wallet is busy, please retry.")
	}

	transaction := &model.Transaction{
		ID:               req.TransferID,
		SenderWalletID:   nil,
		ReceiverWalletID: receiverWallet.ID,
		Amount:           req.Amount,
		Description:      "External transfer from GoWallet",
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
	}
	if err := s.txRepo.CreateTx(ctx, tx, transaction); err != nil {
		return nil, customErr.ErrInternalServer
	}

	creditEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWallet.ID,
		TransactionID: req.TransferID,
		EntryType:     "credit",
		Amount:        req.Amount,
	}
	if err := s.ledgerRepo.CreateTx(ctx, tx, creditEntry); err != nil {
		return nil, customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	cacheKey := "wallet:user:" + receiverWallet.UserID
	if s.rdb != nil {
		go func() {
			s.rdb.Del(context.Background(), cacheKey)
		}()
	}

	if req.CallbackURL != "" {
		go s.sendTransferCallback(context.Background(), req)
	}

	return &model.ExternalTransferStatus{
		TransferID:     req.TransferID,
		Status:         "success",
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func (s *transactionService) sendTransferCallback(ctx context.Context, req model.ExternalTransferRequest) {
	callbackPayload := map[string]any{
		"transfer_id":     req.TransferID,
		"status":          "success",
		"receiver_email":  req.ReceiverEmail,
		"amount":          req.Amount.String(),
		"idempotency_key": req.IdempotencyKey,
	}

	body, err := json.Marshal(callbackPayload)
	if err != nil {
		slog.Error("failed to marshal callback payload", "transfer_id", req.TransferID, "error", err)
		return
	}

	signature := generateHMAC(body, s.webhookSecret)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.CallbackURL, bytes.NewBuffer(body))
	if err != nil {
		slog.Error("failed to create callback request", "transfer_id", req.TransferID, "error", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", s.webhookSecret)
	httpReq.Header.Set("X-Webhook-Signature", signature)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		slog.Error("failed to send callback", "transfer_id", req.TransferID, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("callback returned error", "transfer_id", req.TransferID, "status", resp.StatusCode, "body", string(bodyBytes))
		return
	}

	slog.Info("callback sent successfully", "transfer_id", req.TransferID)
}

func generateHMAC(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *transactionService) GetExternalTransferStatus(ctx context.Context, transferID string) (*model.ExternalTransferStatus, error) {
	transaction, err := s.txRepo.GetByID(ctx, transferID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, customErr.NewAppError(http.StatusNotFound, "TRANSFER_NOT_FOUND", "Transfer not found")
		}
		return nil, customErr.ErrInternalServer
	}

	return &model.ExternalTransferStatus{
		TransferID:     transaction.ID,
		Status:         transaction.Status,
		IdempotencyKey: transaction.IdempotencyKey,
	}, nil
}
