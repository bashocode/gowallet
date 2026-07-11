package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/hmac"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/circuitbreaker"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transferModel "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	transferRepo "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	pbUser "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransactionService interface {
	Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error)
	GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error)
	TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error)
	GenerateDailyReport(ctx context.Context) (int, error)
	CreateExternalTransfer(ctx context.Context, senderUserID string, req transferModel.ExternalTransferRequest) (*transferModel.OutboundTransfer, error)
	GetExternalTransfer(ctx context.Context, transferID string) (*transferModel.OutboundTransfer, error)
	ProcessTransferInitiated(ctx context.Context, event transferModel.TransferInitiatedEvent) error
	SettleTransferTx(ctx context.Context, cb transferModel.TransferCallback) error
	ReconcilePendingTransfers(ctx context.Context) error
}

type DLQPublisher interface {
	Publish(ctx context.Context, topic string, payload map[string]string) error
}

type transactionService struct {
	db              *sql.DB
	txRepo          repository.TransactionRepository
	transferRepo    transferRepo.OutboundTransferRepository
	outboxRepo      transferRepo.TransferOutboxRepository
	userClient      pbUser.UserServiceClient
	walletClient    pbWallet.WalletServiceClient
	ledgerClient    pbLedger.LedgerServiceClient
	userBreaker     *circuitbreaker.CircuitBreaker
	walletBreaker   *circuitbreaker.CircuitBreaker
	ledgerBreaker   *circuitbreaker.CircuitBreaker
	dlqPublisher    DLQPublisher
	monolithBaseURL string
	webhookSecret   string
	httpClient      *http.Client
}

func NewTransactionService(
	db *sql.DB,
	txRepo repository.TransactionRepository,
	transferRepo transferRepo.OutboundTransferRepository,
	outboxRepo transferRepo.TransferOutboxRepository,
	userClient pbUser.UserServiceClient,
	walletClient pbWallet.WalletServiceClient,
	ledgerClient pbLedger.LedgerServiceClient,
	dlq DLQPublisher,
	monolithBaseURL string,
	webhookSecret string,
) TransactionService {
	if monolithBaseURL == "" {
		monolithBaseURL = "http://localhost:8080"
	}
	if webhookSecret == "" {
		webhookSecret = "gowallet-webhook-secret-change-me"
	}
	return &transactionService{
		db:              db,
		txRepo:          txRepo,
		transferRepo:    transferRepo,
		outboxRepo:      outboxRepo,
		userClient:      userClient,
		walletClient:    walletClient,
		ledgerClient:    ledgerClient,
		userBreaker:     circuitbreaker.New(3, 30*time.Second),
		walletBreaker:   circuitbreaker.New(3, 30*time.Second),
		ledgerBreaker:   circuitbreaker.New(3, 30*time.Second),
		dlqPublisher:    dlq,
		monolithBaseURL: monolithBaseURL,
		webhookSecret:   webhookSecret,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *transactionService) Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error) {
	// 1. Check Idempotency Key (double transaction security)
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// 2. Find & Validate Receiver User via User Service gRPC
	var receiverUser *pbUser.UserResponse
	err := s.userBreaker.Call(func() error {
		var callErr error
		receiverUser, callErr = s.userClient.GetUserByEmail(ctx, &pbUser.GetUserByEmailRequest{Email: req.ReceiverEmail})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found.")
	}

	// 3. Get Sender Wallet Details via Wallet Service gRPC
	var senderWallet *pbWallet.WalletResponse
	err = s.walletBreaker.Call(func() error {
		var callErr error
		senderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found.")
	}

	senderBalance, err := decimal.NewFromString(senderWallet.GetBalance())
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	if senderBalance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance.")
	}

	// 4. Record PENDING transaction record to database.
	// We do this in a separate short transaction to release database lock quickly.
	txID := uuid.New().String()
	txRecord := &model.Transaction{
		ID:               txID,
		SenderWalletID:   &senderWallet.Id,
		ReceiverWalletID: receiverUser.Id, // Using User ID as destination WalletID
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "PENDING",
	}
	initTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer initTx.Rollback()

	if err := s.txRepo.CreateTx(ctx, initTx, txRecord); err != nil {
		return nil, customErr.ErrInternalServer
	}
	if err := initTx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 5. Contact Wallet Service & Ledger Service via gRPC for balance mutations (OUTSIDE LOCAL DATABASE TRANSACTION)
	// We apply Saga Orchestration with manual rollback orchestration if any step fails.
	if err := s.executeGrpcTransferChain(ctx, txID, senderUserID, receiverUser.Id, req.Amount, senderWallet); err != nil {
		// If failed, update status to FAILED
		s.txRepo.UpdateStatus(ctx, txID, "FAILED")
		return nil, err
	}

	// 6. If gRPC chain succeeds: Start new super-fast local SQL transaction
	// to update transaction status to SUCCESS and insert outbox event.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback()

	txRecord.Status = "SUCCESS"
	if err := s.txRepo.UpdateStatusTx(ctx, tx, txID, "SUCCESS"); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// Compose Event Payload for Outbox
	eventPayload := fmt.Sprintf(`{
		"transaction_id": "%s",
		"sender_user_id": "%s",
		"receiver_user_id": "%s",
		"amount": %s,
		"description": "%s"
	}`, txID, senderUserID, receiverUser.Id, req.Amount.String(), req.Description)

	outboxEvent := &model.OutboxEvent{
		ID:        uuid.New().String(),
		EventType: "transfer.completed",
		Payload:   eventPayload,
		Status:    "pending",
	}

	// Save event to outbox table in the same local transaction
	if err := s.txRepo.CreateOutboxTx(ctx, tx, outboxEvent); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// Commit local transaction (Lock released in milliseconds!)
	if err := tx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	return txRecord, nil
}

// executeGrpcTransferChain performs the full balance-mutation + ledger saga
// entirely OUTSIDE any local database transaction. On failure it orchestrates
// manual compensation (refund) and returns the originating error so the caller
// can mark the transaction record as FAILED.
func (s *transactionService) executeGrpcTransferChain(
	ctx context.Context,
	txID, senderUserID, receiverUserID string,
	amount decimal.Decimal,
	senderWallet *pbWallet.WalletResponse,
) error {
	debitAmount := amount.Neg()
	// 5. Deduct Sender Balance (Debit) via Wallet Service gRPC
	err := s.walletBreaker.Call(func() error {
		var callErr error
		_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
			UserId:          senderUserID,
			Amount:          debitAmount.String(),
			ExpectedVersion: senderWallet.Version,
		})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return customErr.NewAppError(http.StatusConflict, "CONCURRENT_ERROR", "Failed to process transaction. Try again.")
	}

	// 6. Add Receiver Balance (Credit) via Wallet Service gRPC
	var receiverWallet *pbWallet.WalletResponse
	err = s.walletBreaker.Call(func() error {
		var callErr error
		receiverWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: receiverUserID})
		return callErr
	})
	if err != nil {
		// Compensation: re-read sender wallet before compensate (BUG-2 fix)
		var compSenderWallet *pbWallet.WalletResponse
		compReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compSenderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
			return callErr
		})
		if compReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read sender failed", slog.String("transaction_id", txID), slog.Any("error", compReadErr))
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "get_sender_wallet_for_compensation", "error": compReadErr.Error()})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}
		compRefundErr := s.walletBreaker.Call(func() error {
			var callErr error
			_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
				UserId:          senderUserID,
				Amount:          amount.String(),
				ExpectedVersion: compSenderWallet.Version,
			})
			return callErr
		})
		if compRefundErr != nil {
			logger.Error(ctx, "CRITICAL: compensation refund sender failed", slog.String("transaction_id", txID), slog.Any("error", compRefundErr))
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "refund_sender_after_receiver_wallet_not_found", "error": compRefundErr.Error()})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}
		if err.Error() == "circuit breaker is open — service unavailable" {
			return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found.")
	}

	err = s.walletBreaker.Call(func() error {
		var callErr error
		_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
			UserId:          receiverUserID,
			Amount:          amount.String(),
			ExpectedVersion: receiverWallet.Version,
		})
		return callErr
	})
	if err != nil {
		// Compensation: re-read sender wallet before compensate (BUG-2 fix)
		var compSenderWallet *pbWallet.WalletResponse
		compReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compSenderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
			return callErr
		})
		if compReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read sender failed", slog.String("transaction_id", txID), slog.Any("error", compReadErr))
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "get_sender_wallet_for_compensation", "error": compReadErr.Error()})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}
		compRefundErr := s.walletBreaker.Call(func() error {
			var callErr error
			_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
				UserId:          senderUserID,
				Amount:          amount.String(),
				ExpectedVersion: compSenderWallet.Version,
			})
			return callErr
		})
		if compRefundErr != nil {
			logger.Error(ctx, "CRITICAL: compensation refund sender failed", slog.String("transaction_id", txID), slog.Any("error", compRefundErr))
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "refund_sender_after_credit_fail", "error": compRefundErr.Error()})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}
		if err.Error() == "circuit breaker is open — service unavailable" {
			return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return customErr.ErrInternalServer
	}

	// 7. Record Financial Audit Trail in Ledger Service gRPC
	err = s.ledgerBreaker.Call(func() error {
		var callErr error
		_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
			TransactionId: txID,
			WalletId:      senderWallet.Id,
			Type:          "debit",
			Amount:        amount.String(),
		})
		return callErr
	})
	if err != nil {
		// Compensation: re-read receiver & sender wallet before compensate (BUG-2 fix)
		compFailed := false

		var compReceiverWallet *pbWallet.WalletResponse
		compRecvReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compReceiverWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: receiverUserID})
			return callErr
		})
		if compRecvReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read receiver failed", slog.String("transaction_id", txID), slog.Any("error", compRecvReadErr))
			compFailed = true
		} else {
			if compDebitErr := s.walletBreaker.Call(func() error {
				var callErr error
				_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
					UserId:          receiverUserID,
					Amount:          debitAmount.String(),
					ExpectedVersion: compReceiverWallet.Version,
				})
				return callErr
			}); compDebitErr != nil {
				logger.Error(ctx, "CRITICAL: compensation debit receiver failed", slog.String("transaction_id", txID), slog.Any("error", compDebitErr))
				compFailed = true
			}
		}

		var compSenderWallet *pbWallet.WalletResponse
		compSendReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compSenderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
			return callErr
		})
		if compSendReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read sender failed", slog.String("transaction_id", txID), slog.Any("error", compSendReadErr))
			compFailed = true
		} else {
			if compCreditErr := s.walletBreaker.Call(func() error {
				var callErr error
				_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
					UserId:          senderUserID,
					Amount:          amount.String(),
					ExpectedVersion: compSenderWallet.Version,
				})
				return callErr
			}); compCreditErr != nil {
				logger.Error(ctx, "CRITICAL: compensation credit sender failed", slog.String("transaction_id", txID), slog.Any("error", compCreditErr))
				compFailed = true
			}
		}

		if compFailed {
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "compensation_after_ledger_debit_fail"})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}

		if err.Error() == "circuit breaker is open — service unavailable" {
			return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ledger service is currently unavailable.")
		}
		return customErr.NewAppError(http.StatusInternalServerError, "LEDGER_ERROR", "Failed to record audit log. Transaction cancelled.")
	}

	err = s.ledgerBreaker.Call(func() error {
		var callErr error
		_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
			TransactionId: txID,
			WalletId:      receiverWallet.Id,
			Type:          "credit",
			Amount:        amount.String(),
		})
		return callErr
	})
	if err != nil {
		// Compensation: re-read receiver & sender wallet before compensate (BUG-2 fix)
		compFailed := false

		var compReceiverWallet *pbWallet.WalletResponse
		compRecvReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compReceiverWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: receiverUserID})
			return callErr
		})
		if compRecvReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read receiver failed", slog.String("transaction_id", txID), slog.Any("error", compRecvReadErr))
			compFailed = true
		} else {
			if compDebitErr := s.walletBreaker.Call(func() error {
				var callErr error
				_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
					UserId:          receiverUserID,
					Amount:          debitAmount.String(),
					ExpectedVersion: compReceiverWallet.Version,
				})
				return callErr
			}); compDebitErr != nil {
				logger.Error(ctx, "CRITICAL: compensation debit receiver failed", slog.String("transaction_id", txID), slog.Any("error", compDebitErr))
				compFailed = true
			}
		}

		var compSenderWallet *pbWallet.WalletResponse
		compSendReadErr := s.walletBreaker.Call(func() error {
			var callErr error
			compSenderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
			return callErr
		})
		if compSendReadErr != nil {
			logger.Error(ctx, "CRITICAL: compensation re-read sender failed", slog.String("transaction_id", txID), slog.Any("error", compSendReadErr))
			compFailed = true
		} else {
			if compCreditErr := s.walletBreaker.Call(func() error {
				var callErr error
				_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
					UserId:          senderUserID,
					Amount:          amount.String(),
					ExpectedVersion: compSenderWallet.Version,
				})
				return callErr
			}); compCreditErr != nil {
				logger.Error(ctx, "CRITICAL: compensation credit sender failed", slog.String("transaction_id", txID), slog.Any("error", compCreditErr))
				compFailed = true
			}
		}

		// Compensation: since sender's DEBIT ledger above was already recorded, we must write a balancing CREDIT ledger (ledger is immutable)
		if compLedgerErr := s.ledgerBreaker.Call(func() error {
			var callErr error
			_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
				TransactionId: txID,
				WalletId:      senderWallet.Id,
				Type:          "credit",
				Amount:        amount.String(),
			})
			return callErr
		}); compLedgerErr != nil {
			logger.Error(ctx, "CRITICAL: compensation ledger reversal failed", slog.String("transaction_id", txID), slog.Any("error", compLedgerErr))
			compFailed = true
		}

		if compFailed {
			s.dlqPublisher.Publish(ctx, "compensation.failed", map[string]string{"transaction_id": txID, "step": "compensation_after_ledger_credit_fail"})
			return customErr.NewAppError(http.StatusInternalServerError, "COMPENSATION_FAILED", "Compensation failed. Manual intervention required.")
		}

		if err.Error() == "circuit breaker is open — service unavailable" {
			return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ledger service is currently unavailable.")
		}
		return customErr.NewAppError(http.StatusInternalServerError, "LEDGER_ERROR", "Failed to record receiver audit log. Transaction cancelled.")
	}

	return nil
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

	// Get user's wallet via gRPC (with circuit breaker)
	var wallet *pbWallet.WalletResponse
	err := s.walletBreaker.Call(func() error {
		var callErr error
		wallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
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

	// 2. Get user's wallet (with circuit breaker)
	var wallet *pbWallet.WalletResponse
	err := s.walletBreaker.Call(func() error {
		var callErr error
		wallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: userID})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
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
	err = s.walletBreaker.Call(func() error {
		var callErr error
		_, callErr = s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
			UserId:          userID,
			Amount:          req.Amount.String(),
			ExpectedVersion: wallet.Version,
		})
		return callErr
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusConflict, "TOPUP_FAILED", "Wallet credit failed, please try again")
	}

	// 5. Saga: Record ledger entry (credit only)
	err = s.ledgerBreaker.Call(func() error {
		var callErr error
		_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
			TransactionId: transactionID,
			WalletId:      wallet.Id,
			Type:          "credit",
			Amount:        req.Amount.String(),
		})
		return callErr
	})
	if err != nil {
		s.markFailed(ctx, transactionID)
		logger.Error(ctx, "Partial saga failure: ledger record failed after wallet credit",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ledger service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusConflict, "TOPUP_FAILED", "Ledger record failed, please try again")
	}

	// 6. Mark transaction as success
	if err := s.txRepo.UpdateStatus(ctx, transactionID, "success"); err != nil {
		logger.Error(ctx, "Failed to update transaction status to success",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
	}
	transaction.Status = "success"

	return transaction, nil
}

// GenerateDailyReport produces the daily transaction report. Currently returns
// the count of today's transactions; a CSV/object-storage export can be wired
// here later without changing the scheduler contract.
func (s *transactionService) GenerateDailyReport(ctx context.Context) (int, error) {
	count, err := s.txRepo.CountToday(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to count today's transactions", slog.Any("error", err))
		return 0, err
	}
	return int(count), nil
}

func (s *transactionService) markFailed(ctx context.Context, transactionID string) {
	if err := s.txRepo.UpdateStatus(ctx, transactionID, "failed"); err != nil {
		logger.Error(ctx, "Failed to mark transaction as failed",
			slog.String("transaction_id", transactionID),
			slog.Any("error", err),
		)
	}
}

const (
	minTransferAmount = 1000
	maxTransferAmount = 500000
)

func (s *transactionService) CreateExternalTransfer(ctx context.Context, senderUserID string, req transferModel.ExternalTransferRequest) (*transferModel.OutboundTransfer, error) {
	if req.Amount.LessThan(decimal.NewFromInt(minTransferAmount)) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_AMOUNT",
			fmt.Sprintf("amount below minimum transfer limit of %d IDR", minTransferAmount))
	}
	if req.Amount.GreaterThan(decimal.NewFromInt(maxTransferAmount)) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_AMOUNT",
			fmt.Sprintf("amount exceeds maximum transfer limit of %d IDR", maxTransferAmount))
	}

	existing, _ := s.transferRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	var senderWallet *pbWallet.WalletResponse
	err := s.walletBreaker.Call(func() error {
		var callErr error
		senderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found.")
	}

	senderBalance, err := decimal.NewFromString(senderWallet.Balance)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	if senderBalance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE",
			fmt.Sprintf("insufficient balance: have %s, need %s", senderBalance.String(), req.Amount.String()))
	}

	debitAmount := req.Amount.Neg()
	err = s.walletBreaker.Call(func() error {
		_, callErr := s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
			UserId:          senderUserID,
			Amount:          debitAmount.String(),
			ExpectedVersion: senderWallet.Version,
		})
		return callErr
	})
	if err != nil {
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Wallet service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusConflict, "CONCURRENT_ERROR", "Failed to process transfer. Try again.")
	}

	transferID := uuid.New().String()
	err = s.ledgerBreaker.Call(func() error {
		var callErr error
		_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
			TransactionId: transferID,
			WalletId:      senderWallet.Id,
			Type:          "debit",
			Amount:        req.Amount.String(),
		})
		return callErr
	})
	if err != nil {
		s.refundSender(ctx, senderUserID, req.Amount, senderWallet.Version, transferID)
		if err.Error() == "circuit breaker is open — service unavailable" {
			return nil, customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Ledger service is currently unavailable.")
		}
		return nil, customErr.NewAppError(http.StatusInternalServerError, "LEDGER_ERROR", "Failed to record debit audit log. Sender refunded.")
	}

	now := time.Now().UTC()
	transfer := &transferModel.OutboundTransfer{
		ID:              transferID,
		SenderUserID:    senderUserID,
		SenderWalletID:  senderWallet.Id,
		ReceiverEmail:   req.ReceiverEmail,
		Amount:          req.Amount,
		Currency:        "IDR",
		ExternalEwallet: "monolith",
		Status:          "pending",
		IdempotencyKey:  req.IdempotencyKey,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.transferRepo.Create(ctx, transfer); err != nil {
		s.refundSender(ctx, senderUserID, req.Amount, senderWallet.Version, transferID)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "TRANSFER_FAILED", "Failed to create transfer record. Sender refunded.")
	}

	event := transferModel.TransferInitiatedEvent{
		EventID:        uuid.NewString(),
		EventType:      "transfer.initiated",
		TransferID:     transferID,
		SenderUserID:   senderUserID,
		ReceiverEmail:  req.ReceiverEmail,
		Amount:         req.Amount.String(),
		Currency:       "IDR",
		IdempotencyKey: req.IdempotencyKey,
		OccurredAt:     now,
	}
	payload, _ := json.Marshal(event)
	outboxEvent := &transferModel.TransferOutboxEvent{
		ID:          event.EventID,
		EventType:   event.EventType,
		AggregateID: transferID,
		Payload:     string(payload),
		Status:      "pending",
	}
	if err := s.outboxRepo.Create(ctx, outboxEvent); err != nil {
		logger.Log.Error("Failed to publish transfer.initiated event to outbox",
			slog.String("transfer_id", transferID),
			slog.Any("error", err),
		)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "TRANSFER_FAILED", "Failed to queue transfer for processing.")
	}

	logger.Log.Info("External transfer initiated, event queued",
		slog.String("transfer_id", transferID),
		slog.String("event_id", event.EventID),
	)

	return transfer, nil
}

func (s *transactionService) GetExternalTransfer(ctx context.Context, transferID string) (*transferModel.OutboundTransfer, error) {
	transfer, err := s.transferRepo.GetByID(ctx, transferID)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	if transfer == nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "TRANSFER_NOT_FOUND", "Transfer not found.")
	}
	return transfer, nil
}

func (s *transactionService) ProcessTransferInitiated(ctx context.Context, event transferModel.TransferInitiatedEvent) error {
	logger.Log.Info("Processing transfer.initiated event",
		slog.String("transfer_id", event.TransferID),
		slog.String("event_id", event.EventID),
	)

	if err := s.validateReceiver(ctx, event.ReceiverEmail); err != nil {
		logger.Log.Error("Receiver validation failed, refunding sender",
			slog.String("transfer_id", event.TransferID),
			slog.Any("error", err),
		)
		s.failTransfer(ctx, event.TransferID, event.SenderUserID, event.Amount)
		return err
	}

	transfer := &transferModel.OutboundTransfer{
		ID:             event.TransferID,
		SenderUserID:   event.SenderUserID,
		ReceiverEmail:  event.ReceiverEmail,
		Amount:         decimal.RequireFromString(event.Amount),
		Currency:       event.Currency,
		IdempotencyKey: event.IdempotencyKey,
	}
	if err := s.notifyMonolith(ctx, transfer); err != nil {
		logger.Log.Error("Monolith notification failed, refunding sender",
			slog.String("transfer_id", event.TransferID),
			slog.Any("error", err),
		)
		s.failTransfer(ctx, event.TransferID, event.SenderUserID, event.Amount)
		return err
	}

	logger.Log.Info("Monolith accepted external transfer, awaiting callback",
		slog.String("transfer_id", event.TransferID),
	)
	return nil
}

func (s *transactionService) failTransfer(ctx context.Context, transferID, senderUserID, amount string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE transactions SET status = 'failed' WHERE id = ? AND type = 'external_transfer'`, transferID)
	s.refundSender(ctx, senderUserID, decimal.RequireFromString(amount), 0, transferID)
}

func (s *transactionService) validateReceiver(ctx context.Context, receiverEmail string) error {
	url := fmt.Sprintf("%s/api/v1/users/email/%s", s.monolithBaseURL, receiverEmail)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return customErr.ErrInternalServer
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return customErr.NewAppError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Monolith service is currently unavailable.")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver email not found.")
	}
	if resp.StatusCode >= 300 {
		return customErr.NewAppError(http.StatusBadGateway, "MONOLITH_ERROR", "Monolith returned an error during receiver validation.")
	}
	return nil
}

func (s *transactionService) notifyMonolith(ctx context.Context, transfer *transferModel.OutboundTransfer) error {
	payload, _ := json.Marshal(map[string]any{
		"transfer_id":     transfer.ID,
		"receiver_email":  transfer.ReceiverEmail,
		"amount":          transfer.Amount.String(),
		"currency":        transfer.Currency,
		"idempotency_key": transfer.IdempotencyKey,
		"sender_user_id":  transfer.SenderUserID,
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		s.monolithBaseURL+"/api/v1/transfers/external",
		bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 300 {
		return fmt.Errorf("monolith rejected transfer with status %d", resp.StatusCode)
	}

	logger.Log.Info("Monolith accepted external transfer, awaiting callback",
		slog.String("transfer_id", transfer.ID),
	)
	return nil
}

func (s *transactionService) refundSender(ctx context.Context, senderUserID string, amount decimal.Decimal, originalVersion int32, transferID string) {
	var senderWallet *pbWallet.WalletResponse
	err := s.walletBreaker.Call(func() error {
		var callErr error
		senderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
		return callErr
	})
	if err != nil {
		logger.Log.Error("CRITICAL: compensation failed — cannot re-read sender wallet for refund",
			slog.String("user_id", senderUserID),
			slog.Any("error", err),
		)
		return
	}

	err = s.walletBreaker.Call(func() error {
		_, callErr := s.walletClient.UpdateWalletBalance(ctx, &pbWallet.UpdateBalanceRequest{
			UserId:          senderUserID,
			Amount:          amount.String(),
			ExpectedVersion: senderWallet.Version,
		})
		return callErr
	})
	if err != nil {
		logger.Log.Error("CRITICAL: compensation failed — cannot refund sender",
			slog.String("user_id", senderUserID),
			slog.String("amount", amount.String()),
			slog.Any("error", err),
		)
		return
	}

	ledgerErr := s.ledgerBreaker.Call(func() error {
		var callErr error
		_, callErr = s.ledgerClient.RecordLedgerEntry(ctx, &pbLedger.RecordEntryRequest{
			TransactionId: transferID,
			WalletId:      senderWallet.Id,
			Type:          "credit",
			Amount:        amount.String(),
		})
		return callErr
	})
	if ledgerErr != nil {
		logger.Log.Error("CRITICAL: compensation failed — cannot record balancing ledger credit",
			slog.String("transfer_id", transferID),
			slog.String("user_id", senderUserID),
			slog.String("amount", amount.String()),
			slog.Any("error", ledgerErr),
		)
	}

	logger.Log.Info("Compensation: sender refunded after failed transfer",
		slog.String("transfer_id", transferID),
		slog.String("user_id", senderUserID),
		slog.String("amount", amount.String()),
	)
}

func (s *transactionService) SettleTransferTx(ctx context.Context, cb transferModel.TransferCallback) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	transfer, err := s.transferRepo.GetByIdempotencyKeyTx(ctx, tx, cb.IdempotencyKey)
	if err != nil {
		return err
	}
	if transfer == nil {
		return customErr.NewAppError(http.StatusNotFound, "TRANSFER_NOT_FOUND", "Transfer not found.")
	}

	if transfer.Status == "settled" || transfer.Status == "failed" {
		return tx.Commit()
	}

	status := cb.Status
	if status == "" {
		status = "settled"
	}
	if status != "settled" && status != "failed" {
		status = "failed"
	}

	if status == "failed" {
		s.refundSender(ctx, transfer.SenderUserID, transfer.Amount, 0, transfer.ID)
	}

	if err := s.transferRepo.UpdateStatusTx(ctx, tx, transfer.ID, status); err != nil {
		return err
	}

	eventType := "transfer.settled"
	if status == "failed" {
		eventType = "transfer.failed"
	}

	event := transferModel.TransferSettledEvent{
		EventID:         uuid.NewString(),
		EventType:       eventType,
		TransferID:      transfer.ID,
		SenderUserID:    transfer.SenderUserID,
		ReceiverEmail:   transfer.ReceiverEmail,
		Amount:          transfer.Amount.String(),
		Currency:        transfer.Currency,
		Status:          status,
		ExternalEwallet: transfer.ExternalEwallet,
		OccurredAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	outbox := transferModel.TransferOutboxEvent{
		ID:          event.EventID,
		EventType:   event.EventType,
		AggregateID: transfer.ID,
		Payload:     string(payload),
		Status:      "pending",
	}
	if err := s.outboxRepo.CreateTx(ctx, tx, &outbox); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Log.Info("Transfer settled and outbox event queued",
		slog.String("transfer_id", transfer.ID),
		slog.String("status", status),
		slog.String("event_id", event.EventID),
	)
	return nil
}

func (s *transactionService) ReconcilePendingTransfers(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, sender_wallet_id, receiver_email, amount, idempotency_key
		FROM transactions
		WHERE type = 'external_transfer' AND status = 'pending' AND created_at < DATE_SUB(NOW(), INTERVAL 5 MINUTE)`)
	if err != nil {
		return fmt.Errorf("query pending transfers: %w", err)
	}
	defer rows.Close()

	var stale []transferModel.OutboundTransfer
	for rows.Next() {
		var t transferModel.OutboundTransfer
		if err := rows.Scan(&t.ID, &t.SenderWalletID, &t.ReceiverEmail, &t.Amount, &t.IdempotencyKey); err != nil {
			continue
		}
		stale = append(stale, t)
	}

	if len(stale) == 0 {
		return nil
	}

	logger.Log.Info("Reconciling stale pending transfers", "count", len(stale))

	for _, t := range stale {
		status, err := s.queryMonolithTransferStatus(ctx, t.ID)
		if err != nil {
			logger.Log.Warn("Could not query monolith for transfer status during reconciliation",
				slog.String("transfer_id", t.ID),
				slog.Any("error", err),
			)
			continue
		}

		if status == "settled" || status == "failed" {
			logger.Log.Info("Reconciliation: settling stale transfer",
				slog.String("transfer_id", t.ID),
				slog.String("status", status),
			)
			_ = s.SettleTransferTx(ctx, transferModel.TransferCallback{
				TransferID:     t.ID,
				Status:         status,
				ReceiverEmail:  t.ReceiverEmail,
				Amount:         t.Amount.String(),
				IdempotencyKey: t.IdempotencyKey,
			})
		}
	}

	return nil
}

func (s *transactionService) queryMonolithTransferStatus(ctx context.Context, transferID string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/transfers/external/%s/status", s.monolithBaseURL, transferID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "failed", nil
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("monolith returned status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Status, nil
}

func VerifyWebhookSignature(payload []byte, signature string, secret string) error {
	return hmac.Verify(payload, secret, signature)
}
