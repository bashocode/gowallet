package grpc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	pb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockWalletRepository struct {
	getFunc       func(ctx context.Context, userID string) (*model.Wallet, error)
	updateFunc    func(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	createFunc    func(ctx context.Context, w *model.Wallet) error
	reconcileFunc func(ctx context.Context) (int, int, error)
}

func (m *mockWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	return m.getFunc(ctx, userID)
}

func (m *mockWalletRepository) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	return m.updateFunc(ctx, userID, amount, expectedVersion)
}

func (m *mockWalletRepository) Create(ctx context.Context, w *model.Wallet) error {
	return m.createFunc(ctx, w)
}

func (m *mockWalletRepository) ReconcileAll(ctx context.Context) (int, int, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(ctx)
	}
	return 0, 0, nil
}

func TestWalletGRPCServer(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()

	mockRepo := &mockWalletRepository{
		getFunc: func(ctx context.Context, userID string) (*model.Wallet, error) {
			if userID == "user-123" {
				return &model.Wallet{
					ID:      "wallet-123",
					UserID:  "user-123",
					Balance: decimal.NewFromInt(1500),
					Version: 1,
				}, nil
			}
			return nil, errors.New("not found")
		},
		updateFunc: func(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
			if userID == "user-123" && expectedVersion == 1 {
				return &model.Wallet{
					ID:      "wallet-123",
					UserID:  "user-123",
					Balance: decimal.NewFromInt(1500).Add(amount),
					Version: 2,
				}, nil
			}
			return nil, errors.New("concurrent update or invalid version")
		},
		createFunc: func(ctx context.Context, w *model.Wallet) error {
			if w.UserID == "user-456" {
				return nil
			}
			return errors.New("failed to create")
		},
	}

	pb.RegisterWalletServiceServer(s, NewWalletGRPCServer(mockRepo))

	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Fatal(nil, "Server exited with error", "error", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough://", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)

	// Test CreateWallet
	t.Run("CreateWallet_Success", func(t *testing.T) {
		resp, err := client.CreateWallet(context.Background(), &pb.CreateWalletRequest{UserId: "user-456"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp.GetUserId() != "user-456" {
			t.Errorf("Expected user ID user-456, got %s", resp.GetUserId())
		}
		if resp.GetBalance() != "0.0" {
			t.Errorf("Expected balance 0.0, got %s", resp.GetBalance())
		}
	})

	// Test GetWalletByUserID
	t.Run("GetWalletByUserID_Success", func(t *testing.T) {
		resp, err := client.GetWalletByUserID(context.Background(), &pb.GetWalletRequest{UserId: "user-123"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp.GetBalance() != "1500.0" {
			t.Errorf("Expected balance 1500.0, got %s", resp.GetBalance())
		}
		if resp.GetVersion() != 1 {
			t.Errorf("Expected version 1, got %d", resp.GetVersion())
		}
	})

	t.Run("GetWalletByUserID_NotFound", func(t *testing.T) {
		_, err := client.GetWalletByUserID(context.Background(), &pb.GetWalletRequest{UserId: "user-999"})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})

	// Test UpdateWalletBalance
	t.Run("UpdateWalletBalance_Success", func(t *testing.T) {
		resp, err := client.UpdateWalletBalance(context.Background(), &pb.UpdateBalanceRequest{
			UserId:          "user-123",
			Amount:          "500.0",
			ExpectedVersion: 1,
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp.GetBalance() != "2000.0" {
			t.Errorf("Expected balance 2000.0, got %s", resp.GetBalance())
		}
		if resp.GetVersion() != 2 {
			t.Errorf("Expected version 2, got %d", resp.GetVersion())
		}
	})
}
