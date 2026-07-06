package grpc

import (
	"context"
	"log/slog"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	pb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TransactionGRPCServer implements the gRPC TransactionServiceServer interface.
type TransactionGRPCServer struct {
	pb.UnimplementedTransactionServiceServer
	svc service.TransactionService
}

// NewTransactionGRPCServer creates a new gRPC server with the given service.
func NewTransactionGRPCServer(svc service.TransactionService) *TransactionGRPCServer {
	return &TransactionGRPCServer{svc: svc}
}

// TopUp handles top-up requests from internal services (e.g., payment-service) via gRPC.
func (s *TransactionGRPCServer) TopUp(ctx context.Context, req *pb.TopUpRequest) (*pb.TopUpResponse, error) {
	logger.Log.Info("gRPC TopUp called", slog.String("user_id", req.UserId), slog.String("amount", req.Amount))

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount: %s", req.Amount)
	}

	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	tx, err := s.svc.TopUp(ctx, req.UserId, model.TopUpRequest{
		Amount:         amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		logger.Log.Error("gRPC TopUp failed", slog.String("user_id", req.UserId), slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "top-up failed: %v", err)
	}

	logger.Log.Info("gRPC TopUp successful", slog.String("transaction_id", tx.ID), slog.String("status", tx.Status))
	return &pb.TopUpResponse{
		TransactionId: tx.ID,
		Status:        tx.Status,
	}, nil
}
