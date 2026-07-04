package grpc

import (
	"context"

	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
	pb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type walletGRPCServer struct {
	pb.UnimplementedWalletServiceServer
	repo repository.WalletRepository
}

func NewWalletGRPCServer(repo repository.WalletRepository) pb.WalletServiceServer {
	return &walletGRPCServer{repo: repo}
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

	err := s.repo.Create(ctx, w)
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
	w, err := s.repo.GetByUserID(ctx, req.GetUserId())
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

	w, err := s.repo.UpdateBalanceWithOwnerCheck(ctx, req.GetUserId(), amount, req.GetExpectedVersion())
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
