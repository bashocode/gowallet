package grpc

import (
	"context"
	"log/slog"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/service"
	pb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type walletGRPCServer struct {
	pb.UnimplementedWalletServiceServer
	service service.WalletService
}

func NewWalletGRPCServer(service service.WalletService) pb.WalletServiceServer {
	return &walletGRPCServer{service: service}
}

func (s *walletGRPCServer) CreateWallet(ctx context.Context, req *pb.CreateWalletRequest) (*pb.WalletResponse, error) {
	w := &model.Wallet{
		ID:       uuid.New().String(),
		UserID:   req.GetUserId(),
		Balance:  decimal.Zero,
		Currency: "IDR",
		Status:   "active",
		Version:  1,
	}

	err := s.service.Create(ctx, w)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create wallet: %v", err)
	}

	return &pb.WalletResponse{
		Id:      w.ID,
		UserId:  w.UserID,
		Balance: w.Balance.String(),
		Version: w.Version,
	}, nil
}

func (s *walletGRPCServer) GetWalletByUserID(ctx context.Context, req *pb.GetWalletRequest) (*pb.WalletResponse, error) {
	w, err := s.service.GetByUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "wallet not found: %v", err)
	}

	return &pb.WalletResponse{
		Id:      w.ID,
		UserId:  w.UserID,
		Balance: w.Balance.String(),
		Version: w.Version,
	}, nil
}

func (s *walletGRPCServer) UpdateWalletBalance(ctx context.Context, req *pb.UpdateBalanceRequest) (*pb.WalletResponse, error) {
	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount: %v", err)
	}

	w, err := s.service.UpdateBalanceWithOwnerCheck(ctx, req.GetUserId(), amount, req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "concurrent update failure or insufficient balance: %v", err)
	}

	return &pb.WalletResponse{
		Id:      w.ID,
		UserId:  w.UserID,
		Balance: w.Balance.String(),
		Version: w.Version,
	}, nil
}

// ReconcileBalances is triggered by scheduler-service to audit wallet balance
// integrity. It returns the count of wallets whose stored balance disagrees
// with the expected invariant (balance >= 0). The domain-owning service is the
// only one allowed to read its own DB (Database-per-Service boundary).
func (s *walletGRPCServer) ReconcileBalances(ctx context.Context, _ *pb.ReconcileRequest) (*pb.ReconcileResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] ReconcileBalances triggered by scheduler-service")

	mismatches, total, err := s.service.ReconcileAll(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reconciliation failed: %v", err)
	}

	logger.Log.InfoContext(ctx, "[gRPC] Balance reconciliation completed",
		slog.Int("mismatches", mismatches), slog.Int("total_wallets", total))

	return &pb.ReconcileResponse{
		MismatchCount: int32(mismatches),
		TotalWallets:  int32(total),
	}, nil
}
