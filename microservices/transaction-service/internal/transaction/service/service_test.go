package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	pbUser "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
)

// newTestDB returns a *sql.DB backed by sqlmock that accepts any BeginTx -> Exec* -> Commit
// sequence (used by the Outbox-pattern Transfer flow which the mock repository records).
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE transactions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	return db
}

// newTestDBPendingOnly is like newTestDB but only expects the initial PENDING insert transaction.
// Used by failure-path tests where the gRPC chain aborts before the SUCCESS+outbox commit.
func newTestDBPendingOnly(t *testing.T) *sql.DB {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	return db
}

// Mock repository
type MockTxRepo struct {
	mu           sync.Mutex
	transactions map[string]*model.Transaction
	outboxEvents []*model.OutboxEvent
}

func NewMockTxRepo() *MockTxRepo {
	return &MockTxRepo{
		transactions: make(map[string]*model.Transaction),
	}
}

func (m *MockTxRepo) Create(ctx context.Context, t *model.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions[t.ID] = t
	return nil
}

func (m *MockTxRepo) CreateTx(ctx context.Context, tx *sql.Tx, t *model.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions[t.ID] = t
	return nil
}

func (m *MockTxRepo) GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.transactions {
		if t.IdempotencyKey == key {
			return t, nil
		}
	}
	return nil, nil
}

func (m *MockTxRepo) GetHistory(ctx context.Context, walletID string, params model.PaginationParams) ([]model.Transaction, int64, error) {
	return nil, 0, nil
}

func (m *MockTxRepo) UpdateStatus(ctx context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.transactions[id]; ok {
		t.Status = status
	}
	return nil
}

func (m *MockTxRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.transactions[id]; ok {
		t.Status = status
	}
	return nil
}

func (m *MockTxRepo) CountToday(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockTxRepo) CreateOutboxTx(ctx context.Context, tx *sql.Tx, event *model.OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outboxEvents = append(m.outboxEvents, event)
	return nil
}

func (m *MockTxRepo) FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]model.OutboxEvent, error) {
	return nil, nil
}

func (m *MockTxRepo) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	return nil
}

func (m *MockTxRepo) GetOutboxEvents() []*model.OutboxEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*model.OutboxEvent, len(m.outboxEvents))
	copy(out, m.outboxEvents)
	return out
}

// Mock gRPC User Client
type MockUserClient struct {
	pbUser.UserServiceClient
	GetUserByEmailFunc func(ctx context.Context, in *pbUser.GetUserByEmailRequest) (*pbUser.UserResponse, error)
}

func (m *MockUserClient) GetUserByEmail(ctx context.Context, in *pbUser.GetUserByEmailRequest, opts ...grpc.CallOption) (*pbUser.UserResponse, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(ctx, in)
	}
	return &pbUser.UserResponse{Id: "receiver-123", Email: in.Email}, nil
}

// Mock gRPC Wallet Client
type MockWalletClient struct {
	pbWallet.WalletServiceClient
	GetWalletByUserIDFunc   func(ctx context.Context, in *pbWallet.GetWalletRequest) (*pbWallet.WalletResponse, error)
	UpdateWalletBalanceFunc func(ctx context.Context, in *pbWallet.UpdateBalanceRequest) (*pbWallet.WalletResponse, error)
}

func (m *MockWalletClient) GetWalletByUserID(ctx context.Context, in *pbWallet.GetWalletRequest, opts ...grpc.CallOption) (*pbWallet.WalletResponse, error) {
	if m.GetWalletByUserIDFunc != nil {
		return m.GetWalletByUserIDFunc(ctx, in)
	}
	return &pbWallet.WalletResponse{Id: "wallet-" + in.UserId, UserId: in.UserId, Balance: "100000", Version: 1}, nil
}

func (m *MockWalletClient) UpdateWalletBalance(ctx context.Context, in *pbWallet.UpdateBalanceRequest, opts ...grpc.CallOption) (*pbWallet.WalletResponse, error) {
	if m.UpdateWalletBalanceFunc != nil {
		return m.UpdateWalletBalanceFunc(ctx, in)
	}
	return &pbWallet.WalletResponse{Id: "wallet-" + in.UserId, UserId: in.UserId, Balance: "100000", Version: in.ExpectedVersion + 1}, nil
}

// Mock gRPC Ledger Client
type MockLedgerClient struct {
	pbLedger.LedgerServiceClient
	RecordLedgerEntryFunc func(ctx context.Context, in *pbLedger.RecordEntryRequest) (*pbLedger.RecordEntryResponse, error)
}

func (m *MockLedgerClient) RecordLedgerEntry(ctx context.Context, in *pbLedger.RecordEntryRequest, opts ...grpc.CallOption) (*pbLedger.RecordEntryResponse, error) {
	if m.RecordLedgerEntryFunc != nil {
		return m.RecordLedgerEntryFunc(ctx, in)
	}
	return &pbLedger.RecordEntryResponse{EntryId: "ledger-entry-123", Success: true}, nil
}

// Mock DLQ Publisher
type MockDLQPublisher struct {
	Published []MockDLQEvent
}

type MockDLQEvent struct {
	Topic   string
	Payload map[string]string
}

func (m *MockDLQPublisher) Publish(ctx context.Context, topic string, payload map[string]string) error {
	m.Published = append(m.Published, MockDLQEvent{Topic: topic, Payload: payload})
	return nil
}

func TestTransfer_HappyPath(t *testing.T) {
	txRepo := NewMockTxRepo()
	uClient := &MockUserClient{}
	wClient := &MockWalletClient{}
	lClient := &MockLedgerClient{}

	svc := NewTransactionService(newTestDB(t), txRepo, nil, nil, nil, nil, uClient, wClient, lClient, &MockDLQPublisher{}, "", "", "")

	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "Test Happy Path",
		IdempotencyKey: "idem-key-1",
	}

	tx, err := svc.Transfer(context.Background(), "sender-123", req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if tx.Status != "success" {
		t.Errorf("Expected status success, got: %s", tx.Status)
	}

	savedTx, _ := txRepo.GetByIdempotencyKey(context.Background(), "idem-key-1")
	if savedTx == nil || savedTx.Status != "success" {
		t.Errorf("Expected saved transaction status to be success")
	}

	// Outbox: a single pending "transfer.completed" event must be recorded.
	events := txRepo.GetOutboxEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 outbox event, got %d", len(events))
	}
	ev := events[0]
	if ev.EventType != "transfer.completed" {
		t.Errorf("Expected event_type transfer.completed, got %s", ev.EventType)
	}
	if ev.Status != "pending" {
		t.Errorf("Expected outbox status pending, got %s", ev.Status)
	}
	if ev.ID == "" {
		t.Errorf("Expected outbox event to have a non-empty ID")
	}
}

func TestTransfer_DebitFails(t *testing.T) {
	txRepo := NewMockTxRepo()
	uClient := &MockUserClient{}
	wClient := &MockWalletClient{
		UpdateWalletBalanceFunc: func(ctx context.Context, in *pbWallet.UpdateBalanceRequest) (*pbWallet.WalletResponse, error) {
			return nil, errors.New("debit failure")
		},
	}
	lClient := &MockLedgerClient{}

	svc := NewTransactionService(newTestDBPendingOnly(t), txRepo, nil, nil, nil, nil, uClient, wClient, lClient, &MockDLQPublisher{}, "", "", "")

	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "Test Debit Fails",
		IdempotencyKey: "idem-key-2",
	}

	_, err := svc.Transfer(context.Background(), "sender-123", req)
	if err == nil {
		t.Fatal("Expected error on debit failure, got nil")
	}

	// Verify status in DB is failed
	savedTx, _ := txRepo.GetByIdempotencyKey(context.Background(), "idem-key-2")
	if savedTx == nil || savedTx.Status != "failed" {
		t.Errorf("Expected saved transaction status to be failed, got: %v", savedTx)
	}

	// No outbox event should be written when the transfer fails.
	if events := txRepo.GetOutboxEvents(); len(events) != 0 {
		t.Errorf("Expected no outbox events on failure, got %d", len(events))
	}
}

func TestTransfer_CreditFails_CompensationSucceeds(t *testing.T) {
	txRepo := NewMockTxRepo()
	uClient := &MockUserClient{}

	var compCalled bool
	var compVersion int32
	senderGetCount := 0

	wClient := &MockWalletClient{
		GetWalletByUserIDFunc: func(ctx context.Context, in *pbWallet.GetWalletRequest) (*pbWallet.WalletResponse, error) {
			// Simulate that sender wallet version changed or normal get wallet
			if in.UserId == "sender-123" {
				senderGetCount++
				if senderGetCount > 1 {
					return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "95000", Version: 2}, nil
				}
				return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "100000", Version: 1}, nil
			}
			return &pbWallet.WalletResponse{Id: "wallet-receiver", UserId: "receiver-123", Balance: "50000", Version: 5}, nil
		},
		UpdateWalletBalanceFunc: func(ctx context.Context, in *pbWallet.UpdateBalanceRequest) (*pbWallet.WalletResponse, error) {
			if in.UserId == "sender-123" {
				amount, _ := decimal.NewFromString(in.Amount)
				if amount.IsPositive() {
					compCalled = true
					compVersion = in.ExpectedVersion
					return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "100000", Version: in.ExpectedVersion + 1}, nil
				}
				// Normal Debit
				return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "95000", Version: 2}, nil
			}
			// Credit fails
			return nil, errors.New("credit failure")
		},
	}
	lClient := &MockLedgerClient{}

	svc := NewTransactionService(newTestDBPendingOnly(t), txRepo, nil, nil, nil, nil, uClient, wClient, lClient, &MockDLQPublisher{}, "", "", "")

	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "Test Credit Fails",
		IdempotencyKey: "idem-key-3",
	}

	_, err := svc.Transfer(context.Background(), "sender-123", req)
	if err == nil {
		t.Fatal("Expected error on credit failure, got nil")
	}

	if !compCalled {
		t.Errorf("Expected compensation to be called for sender wallet")
	}

	if compVersion != 2 {
		t.Errorf("Expected compensation to use re-read version (2), got: %d", compVersion)
	}

	savedTx, _ := txRepo.GetByIdempotencyKey(context.Background(), "idem-key-3")
	if savedTx == nil || savedTx.Status != "failed" {
		t.Errorf("Expected saved transaction status to be failed")
	}
}

func TestTransfer_LedgerFails_CompensationSucceeds(t *testing.T) {
	txRepo := NewMockTxRepo()
	uClient := &MockUserClient{}

	var compDebitCalled bool
	var compCreditCalled bool

	wClient := &MockWalletClient{
		GetWalletByUserIDFunc: func(ctx context.Context, in *pbWallet.GetWalletRequest) (*pbWallet.WalletResponse, error) {
			if in.UserId == "sender-123" {
				return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "95000", Version: 2}, nil
			}
			return &pbWallet.WalletResponse{Id: "wallet-receiver", UserId: "receiver-123", Balance: "55000", Version: 6}, nil
		},
		UpdateWalletBalanceFunc: func(ctx context.Context, in *pbWallet.UpdateBalanceRequest) (*pbWallet.WalletResponse, error) {
			amount, _ := decimal.NewFromString(in.Amount)
			if in.UserId == "sender-123" {
				if amount.IsPositive() {
					compCreditCalled = true
					return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "100000", Version: 3}, nil
				}
				return &pbWallet.WalletResponse{Id: "wallet-sender", UserId: "sender-123", Balance: "95000", Version: 2}, nil
			} else {
				if amount.IsNegative() {
					compDebitCalled = true
					return &pbWallet.WalletResponse{Id: "wallet-receiver", UserId: "receiver-123", Balance: "50000", Version: 7}, nil
				}
				return &pbWallet.WalletResponse{Id: "wallet-receiver", UserId: "receiver-123", Balance: "55000", Version: 6}, nil
			}
		},
	}

	lClient := &MockLedgerClient{
		RecordLedgerEntryFunc: func(ctx context.Context, in *pbLedger.RecordEntryRequest) (*pbLedger.RecordEntryResponse, error) {
			return nil, errors.New("ledger failure")
		},
	}

	svc := NewTransactionService(newTestDBPendingOnly(t), txRepo, nil, nil, nil, nil, uClient, wClient, lClient, &MockDLQPublisher{}, "", "", "")

	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "Test Ledger Fails",
		IdempotencyKey: "idem-key-4",
	}

	_, err := svc.Transfer(context.Background(), "sender-123", req)
	if err == nil {
		t.Fatal("Expected error on ledger failure, got nil")
	}

	if !compDebitCalled || !compCreditCalled {
		t.Errorf("Expected compensation for both sender and receiver wallets")
	}

	savedTx, _ := txRepo.GetByIdempotencyKey(context.Background(), "idem-key-4")
	if savedTx == nil || savedTx.Status != "failed" {
		t.Errorf("Expected saved transaction status to be failed")
	}
}

func TestTransfer_CircuitBreaker(t *testing.T) {
	txRepo := NewMockTxRepo()
	uClient := &MockUserClient{}

	callCount := 0
	wClient := &MockWalletClient{
		GetWalletByUserIDFunc: func(ctx context.Context, in *pbWallet.GetWalletRequest) (*pbWallet.WalletResponse, error) {
			callCount++
			return nil, errors.New("connection timed out")
		},
	}
	lClient := &MockLedgerClient{}

	svc := NewTransactionService(nil, txRepo, nil, nil, nil, nil, uClient, wClient, lClient, &MockDLQPublisher{}, "", "", "")

	// Call 1
	_, err := svc.Transfer(context.Background(), "sender-123", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "CB Test 1",
		IdempotencyKey: "cb-1",
	})
	if err == nil {
		t.Fatal("Expected error")
	}

	// Call 2
	_, err = svc.Transfer(context.Background(), "sender-123", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "CB Test 2",
		IdempotencyKey: "cb-2",
	})
	if err == nil {
		t.Fatal("Expected error")
	}

	// Call 3 (triggers open state)
	_, err = svc.Transfer(context.Background(), "sender-123", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "CB Test 3",
		IdempotencyKey: "cb-3",
	})
	if err == nil {
		t.Fatal("Expected error")
	}

	initialCalls := callCount

	// Call 4 (should trip circuit breaker, avoiding remote call)
	_, err = svc.Transfer(context.Background(), "sender-123", model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromInt(5000),
		Description:    "CB Test 4",
		IdempotencyKey: "cb-4",
	})

	if err == nil {
		t.Fatal("Expected error when circuit breaker is open")
	}

	if err.Error() != "Wallet service is currently unavailable." {
		t.Errorf("Expected service unavailable app error, got: %v", err)
	}

	if callCount != initialCalls {
		t.Errorf("Expected wallet service call to be skipped by circuit breaker, got new call")
	}
}
