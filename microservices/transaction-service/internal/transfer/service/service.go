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

	"github.com/bashocode/gowallet/microservices/shared/hmac"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/circuitbreaker"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transfer/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transfer/repository"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	minTransferAmount = 1000   // IDR 1.000
	maxTransferAmount = 500000 // IDR 500.000 (per-transaction limit for learning path)
	httpClientTimeout = 10 * time.Second
)

type TransferService interface {
	CreateExternalTransfer(ctx context.Context, senderUserID string, req model.ExternalTransferRequest) (*model.OutboundTransfer, error)
	SettleTransferTx(ctx context.Context, cb model.TransferCallback) error
	ReconcilePendingTransfers(ctx context.Context) error
}

type transferService struct {
	db              *sql.DB
	transferRepo    repository.OutboundTransferRepository
	outboxRepo      repository.TransferOutboxRepository
	walletClient    pbWallet.WalletServiceClient
	walletBreaker   *circuitbreaker.CircuitBreaker
	monolithBaseURL string
	webhookSecret   string
	httpClient      *http.Client
}

func NewTransferService(
	db *sql.DB,
	transferRepo repository.OutboundTransferRepository,
	outboxRepo repository.TransferOutboxRepository,
	walletClient pbWallet.WalletServiceClient,
	monolithBaseURL string,
	webhookSecret string,
) TransferService {
	if monolithBaseURL == "" {
		monolithBaseURL = "http://localhost:8080"
	}
	if webhookSecret == "" {
		webhookSecret = "gowallet-webhook-secret-change-me"
	}
	return &transferService{
		db:              db,
		transferRepo:    transferRepo,
		outboxRepo:      outboxRepo,
		walletClient:    walletClient,
		walletBreaker:   circuitbreaker.New(3, 30*time.Second),
		monolithBaseURL: monolithBaseURL,
		webhookSecret:   webhookSecret,
		httpClient:      &http.Client{Timeout: httpClientTimeout},
	}
}

// CreateExternalTransfer implements a production-like cross-ewallet transfer:
//
//  1. Validate amount limits (min/max).
//  2. Validate receiver exists in the monolith (pre-flight check, no debit yet).
//  3. Get sender wallet + check balance sufficient.
//  4. Debit sender + create outbound_transfers row in one logical unit
//     (gRPC debit first, then record; if record fails, refund = saga compensation).
//  5. Notify monolith synchronously (not fire-and-forget). If monolith is
//     unreachable, refund sender and mark transfer as failed.
//  6. Return the transfer record. Settlement happens when monolith calls back.
func (s *transferService) CreateExternalTransfer(ctx context.Context, senderUserID string, req model.ExternalTransferRequest) (*model.OutboundTransfer, error) {
	// 1. Validate amount limits
	if req.Amount.LessThan(decimal.NewFromInt(minTransferAmount)) {
		return nil, fmt.Errorf("amount below minimum transfer limit of %d IDR", minTransferAmount)
	}
	if req.Amount.GreaterThan(decimal.NewFromInt(maxTransferAmount)) {
		return nil, fmt.Errorf("amount exceeds maximum transfer limit of %d IDR", maxTransferAmount)
	}

	// 2. Idempotency check
	existing, _ := s.transferRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// 3. Pre-flight: validate receiver exists in monolith (no debit yet)
	if err := s.validateReceiver(ctx, req.ReceiverEmail); err != nil {
		return nil, fmt.Errorf("receiver validation failed: %w", err)
	}

	// 4. Get sender wallet + check balance
	var senderWallet *pbWallet.WalletResponse
	err := s.walletBreaker.Call(func() error {
		var callErr error
		senderWallet, callErr = s.walletClient.GetWalletByUserID(ctx, &pbWallet.GetWalletRequest{UserId: senderUserID})
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("sender wallet not found: %w", err)
	}

	senderBalance, err := decimal.NewFromString(senderWallet.Balance)
	if err != nil {
		return nil, fmt.Errorf("invalid sender balance: %w", err)
	}
	if senderBalance.LessThan(req.Amount) {
		return nil, fmt.Errorf("insufficient balance: have %s, need %s", senderBalance.String(), req.Amount.String())
	}

	// 5. Debit sender wallet
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
		return nil, fmt.Errorf("failed to debit sender: %w", err)
	}

	// 6. Create outbound transfer record. If this fails after debit, we must
	//    refund the sender (saga compensation).
	transferID := uuid.New().String()
	transfer := &model.OutboundTransfer{
		ID:              transferID,
		SenderUserID:    senderUserID,
		ReceiverEmail:   req.ReceiverEmail,
		Amount:          req.Amount,
		Currency:        "IDR",
		ExternalEwallet: "monolith",
		Status:          "pending",
		IdempotencyKey:  req.IdempotencyKey,
	}
	if err := s.transferRepo.Create(ctx, transfer); err != nil {
		// Compensation: refund sender
		s.refundSender(ctx, senderUserID, req.Amount, senderWallet.Version)
		return nil, fmt.Errorf("failed to create transfer record (sender refunded): %w", err)
	}

	// 7. Notify monolith synchronously. If it fails, refund sender and mark
	//    transfer as failed.
	if err := s.notifyMonolith(ctx, transfer); err != nil {
		logger.Log.Error("Monolith notification failed, refunding sender",
			slog.String("transfer_id", transferID),
			slog.Any("error", err),
		)
		s.refundSender(ctx, senderUserID, req.Amount, senderWallet.Version)
		_ = s.transferRepo.UpdateStatusTx(ctx, nil, transferID, "failed")
		// UpdateStatusTx needs a tx; use a direct update instead:
		_, _ = s.db.ExecContext(ctx, `UPDATE outbound_transfers SET status = 'failed' WHERE id = ?`, transferID)
		return nil, fmt.Errorf("monolith unreachable, sender refunded: %w", err)
	}

	return transfer, nil
}

// validateReceiver does a pre-flight GET to the monolith to verify the receiver
// email exists before we debit the sender. This prevents "debit then discover
// receiver invalid" scenarios.
func (s *transferService) validateReceiver(ctx context.Context, receiverEmail string) error {
	url := fmt.Sprintf("%s/api/v1/users/email/%s", s.monolithBaseURL, receiverEmail)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("build receiver validation request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("monolith unreachable during receiver validation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("receiver email not found in monolith: %s", receiverEmail)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("monolith returned status %d during receiver validation", resp.StatusCode)
	}
	return nil
}

// notifyMonolith POSTs the transfer to the monolith's external transfer
// endpoint synchronously. The monolith will credit the receiver and call back
// GoWallet's webhook when done.
func (s *transferService) notifyMonolith(ctx context.Context, transfer *model.OutboundTransfer) error {
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

// refundSender credits the sender back the debited amount. Used in saga
// compensation when the transfer cannot proceed after debit.
func (s *transferService) refundSender(ctx context.Context, senderUserID string, amount decimal.Decimal, originalVersion int32) {
	// Re-read wallet to get latest version (optimistic lock may have changed)
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

	logger.Log.Info("Compensation: sender refunded after failed transfer",
		slog.String("user_id", senderUserID),
		slog.String("amount", amount.String()),
	)
}

// SettleTransferTx is called by the webhook handler when the monolith calls
// back. It updates the transfer status and inserts an outbox event in one SQL
// transaction (Episode 35 transactional outbox).
func (s *transferService) SettleTransferTx(ctx context.Context, cb model.TransferCallback) error {
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
		return fmt.Errorf("transfer not found: %s", cb.IdempotencyKey)
	}

	// Idempotency: already settled
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

	// If monolith says transfer failed, refund the sender
	if status == "failed" {
		s.refundSender(ctx, transfer.SenderUserID, transfer.Amount, 0)
	}

	if err := s.transferRepo.UpdateStatusTx(ctx, tx, transfer.ID, status); err != nil {
		return err
	}

	eventType := "transfer.settled"
	if status == "failed" {
		eventType = "transfer.failed"
	}

	event := model.TransferSettledEvent{
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

	outbox := model.TransferOutboxEvent{
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

// ReconcilePendingTransfers scans for transfers that have been pending too long
// (e.g. callback was lost). For each, it queries the monolith for the current
// status and settles or fails accordingly. This is the safety net that prevents
// transfers from being stuck in "pending" forever.
func (s *transferService) ReconcilePendingTransfers(ctx context.Context) error {
	// Find transfers pending for more than 5 minutes
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, sender_user_id, receiver_email, amount, idempotency_key
		FROM outbound_transfers
		WHERE status = 'pending' AND created_at < DATE_SUB(NOW(), INTERVAL 5 MINUTE)`)
	if err != nil {
		return fmt.Errorf("query pending transfers: %w", err)
	}
	defer rows.Close()

	var stale []model.OutboundTransfer
	for rows.Next() {
		var t model.OutboundTransfer
		if err := rows.Scan(&t.ID, &t.SenderUserID, &t.ReceiverEmail, &t.Amount, &t.IdempotencyKey); err != nil {
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
			_ = s.SettleTransferTx(ctx, model.TransferCallback{
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

// queryMonolithTransferStatus asks the monolith for the current status of a
// transfer. Used by the reconciliation worker.
func (s *transferService) queryMonolithTransferStatus(ctx context.Context, transferID string) (string, error) {
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
		return "failed", nil // monolith doesn't know about it → treat as failed
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

// VerifyWebhookSignature checks the HMAC-SHA256 signature on incoming webhook
// callbacks from the monolith. This prevents unauthorized parties from
// settling transfers.
func VerifyWebhookSignature(payload []byte, signature string, secret string) error {
	return hmac.Verify(payload, secret, signature)
}
