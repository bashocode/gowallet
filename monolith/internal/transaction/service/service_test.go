package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	ledgerRepo "github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	txRepo "github.com/bashocode/gowallet/monolith/internal/transaction/repository"
	userModel "github.com/bashocode/gowallet/monolith/internal/user/model"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletModel "github.com/bashocode/gowallet/monolith/internal/wallet/model"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/go-redis/redismock/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTransfer_Success(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	senderUserID := "sender-123"
	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromFloat(200.0),
		Description:    "Gift",
		IdempotencyKey: "unique-key",
	}

	// 1. check idempotency
	mockTxRepo.On("GetByIdempotencyKey", ctx, req.IdempotencyKey).Return(nil, errors.New("not found"))

	// 2. get receiver
	receiverUser := &userModel.User{ID: "receiver-123", Email: req.ReceiverEmail}
	mockUserRepo.On("GetByEmail", ctx, req.ReceiverEmail).Return(receiverUser, nil)

	// Begin TX
	dbMock.ExpectBegin()
	dbMock.ExpectCommit()

	// 3. get sender and receiver wallet
	senderWallet := &walletModel.Wallet{ID: "wallet-sender", UserID: senderUserID, Balance: decimal.NewFromFloat(1000.0), Version: 1}
	receiverWallet := &walletModel.Wallet{ID: "wallet-receiver", UserID: "receiver-123", Balance: decimal.NewFromFloat(500.0), Version: 2}

	mockWalletRepo.On("GetByUserID", ctx, senderUserID).Return(senderWallet, nil)
	mockWalletRepo.On("GetByUserID", ctx, "receiver-123").Return(receiverWallet, nil)

	// 4. Update balances
	mockWalletRepo.On("UpdateBalanceTx", ctx, mock.Anything, senderWallet.ID, decimal.NewFromFloat(200.0), senderWallet.Version).Return(nil)
	mockWalletRepo.On("UpdateBalanceTx", ctx, mock.Anything, receiverWallet.ID, decimal.NewFromFloat(-200.0), receiverWallet.Version).Return(nil)

	// 5. Create transaction
	mockTxRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil)

	// 6. Create ledger entries
	mockLedgerRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil).Twice()

	// 7. Expect cache invalidation
	senderCacheKey := "wallet:user:" + senderUserID
	receiverCacheKey := "wallet:user:" + receiverUser.ID
	mockRedis.ExpectDel(senderCacheKey, receiverCacheKey).SetVal(2)

	txRes, err := svc.Transfer(ctx, senderUserID, req)

	assert.NoError(t, err)
	assert.NotNil(t, txRes)
	assert.Equal(t, "success", txRes.Status)
	assert.Equal(t, req.Amount, txRes.Amount)

	// Sleep slightly to let the async Redis Del goroutine complete
	time.Sleep(10 * time.Millisecond)

	mockTxRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockWalletRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
	assert.NoError(t, mockRedis.ExpectationsWereMet())
}

func TestTransfer_IdempotencyCached(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	senderUserID := "sender-123"
	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromFloat(200.0),
		Description:    "Gift",
		IdempotencyKey: "unique-key",
	}

	cachedTx := &model.Transaction{ID: "tx-existing", Status: "success", Amount: decimal.NewFromFloat(200.0)}
	mockTxRepo.On("GetByIdempotencyKey", ctx, req.IdempotencyKey).Return(cachedTx, nil)

	txRes, err := svc.Transfer(ctx, senderUserID, req)

	assert.NoError(t, err)
	assert.Equal(t, cachedTx, txRes)
}

func TestTransfer_ReceiverNotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	senderUserID := "sender-123"
	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		IdempotencyKey: "unique-key",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, req.IdempotencyKey).Return(nil, errors.New("not found"))
	mockUserRepo.On("GetByEmail", ctx, req.ReceiverEmail).Return(nil, errors.New("not found"))

	txRes, err := svc.Transfer(ctx, senderUserID, req)

	assert.Error(t, err)
	assert.Nil(t, txRes)
}

func TestTransfer_SelfTransferNotAllowed(t *testing.T) {
	db, dbMock, _ := sqlmock.New()
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	senderUserID := "sender-123"
	req := model.TransferRequest{
		ReceiverEmail:  "sender@example.com",
		Amount:         decimal.NewFromFloat(200.0),
		IdempotencyKey: "unique-key",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, req.IdempotencyKey).Return(nil, errors.New("not found"))

	receiverUser := &userModel.User{ID: senderUserID, Email: req.ReceiverEmail}
	mockUserRepo.On("GetByEmail", ctx, req.ReceiverEmail).Return(receiverUser, nil)

	dbMock.ExpectBegin()

	// Both return same wallet ID
	senderWallet := &walletModel.Wallet{ID: "wallet-shared", UserID: senderUserID}
	mockWalletRepo.On("GetByUserID", ctx, senderUserID).Return(senderWallet, nil)

	txRes, err := svc.Transfer(ctx, senderUserID, req)

	assert.Error(t, err)
	assert.Nil(t, txRes)
}

func TestTransfer_InsufficientBalance(t *testing.T) {
	db, dbMock, _ := sqlmock.New()
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	senderUserID := "sender-123"
	req := model.TransferRequest{
		ReceiverEmail:  "receiver@example.com",
		Amount:         decimal.NewFromFloat(1200.0), // more than balance
		IdempotencyKey: "unique-key",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, req.IdempotencyKey).Return(nil, errors.New("not found"))

	receiverUser := &userModel.User{ID: "receiver-123", Email: req.ReceiverEmail}
	mockUserRepo.On("GetByEmail", ctx, req.ReceiverEmail).Return(receiverUser, nil)

	dbMock.ExpectBegin()

	senderWallet := &walletModel.Wallet{ID: "wallet-sender", UserID: senderUserID, Balance: decimal.NewFromFloat(1000.0)}
	receiverWallet := &walletModel.Wallet{ID: "wallet-receiver", UserID: "receiver-123", Balance: decimal.NewFromFloat(500.0)}

	mockWalletRepo.On("GetByUserID", ctx, senderUserID).Return(senderWallet, nil)
	mockWalletRepo.On("GetByUserID", ctx, "receiver-123").Return(receiverWallet, nil)

	txRes, err := svc.Transfer(ctx, senderUserID, req)

	assert.Error(t, err)
	assert.Nil(t, txRes)
}

func TestGetHistory_Success(t *testing.T) {
	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(nil, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	userID := "user-123"
	wallet := &walletModel.Wallet{ID: "wallet-123"}
	params := model.PaginationParams{Page: 1, Limit: 10}

	expectedHistory := []model.Transaction{
		{ID: "tx-1", ReceiverWalletID: "wallet-123", Amount: decimal.NewFromFloat(500.0)},
	}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)
	mockTxRepo.On("GetHistory", ctx, wallet.ID, params).Return(expectedHistory, int64(1), nil)

	txs, meta, err := svc.GetHistory(ctx, userID, params)

	assert.NoError(t, err)
	assert.Equal(t, expectedHistory, txs)
	assert.Equal(t, int64(1), meta.Total)
	assert.Equal(t, 1, meta.TotalPage)
}

func TestGetHistory_WalletNotFound(t *testing.T) {
	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(nil, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	userID := "user-123"
	params := model.PaginationParams{Page: 1, Limit: 10}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("not found"))

	txs, meta, err := svc.GetHistory(ctx, userID, params)

	assert.Error(t, err)
	assert.Nil(t, txs)
	assert.Nil(t, meta)
}

func TestTopUp_Success(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockTxRepo := new(txRepo.MockTransactionRepository)
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockLedgerRepo := new(ledgerRepo.MockLedgerRepository)
	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()

	svc := NewTransactionService(db, rdb, mockTxRepo, mockUserRepo, mockWalletRepo, mockLedgerRepo)

	ctx := context.TODO()
	userID := "user-123"
	req := model.TopUpRequest{
		Amount:         decimal.NewFromFloat(500.0),
		IdempotencyKey: "unique-topup-key",
	}

	wallet := &walletModel.Wallet{ID: "wallet-123", UserID: userID, Balance: decimal.NewFromFloat(100.0), Version: 1}

	mockWalletRepo.On("GetByUserID", ctx, userID).Return(wallet, nil)

	dbMock.ExpectBegin()
	dbMock.ExpectCommit()

	// UpdateBalanceTx should be called with req.Amount.Neg() which is -500.0 to add balance
	mockWalletRepo.On("UpdateBalanceTx", ctx, mock.Anything, wallet.ID, decimal.NewFromFloat(-500.0), wallet.Version).Return(nil)

	// CreateTx for transaction
	mockTxRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil)

	// CreateTx for credit ledger entry
	mockLedgerRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil)

	cacheKey := "wallet:user:" + userID
	mockRedis.ExpectDel(cacheKey).SetVal(1)

	txRes, err := svc.TopUp(ctx, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, txRes)
	assert.Equal(t, "success", txRes.Status)
	assert.Equal(t, req.Amount, txRes.Amount)

	time.Sleep(10 * time.Millisecond)

	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
	assert.NoError(t, dbMock.ExpectationsWereMet())
	assert.NoError(t, mockRedis.ExpectationsWereMet())
}

