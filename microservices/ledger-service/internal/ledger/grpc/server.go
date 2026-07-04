package grpc

import (
	"context"

	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/model"
	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/repository"
	pb "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ledgerGRPCServer struct {
	pb.UnimplementedLedgerServiceServer
	repo repository.LedgerRepository
}

func NewLedgerGRPCServer(repo repository.LedgerRepository) pb.LedgerServiceServer {
	return &ledgerGRPCServer{repo: repo}
}

func (s *ledgerGRPCServer) RecordLedgerEntry(ctx context.Context, req *pb.RecordEntryRequest) (*pb.RecordEntryResponse, error) {
	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount: %v", err)
	}

	entry := &model.LedgerEntry{
		ID:            uuid.New().String(),
		TransactionID: req.GetTransactionId(),
		WalletID:      req.GetWalletId(),
		EntryType:     req.GetType(),
		Amount:        amount,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record ledger entry: %v", err)
	}

	return &pb.RecordEntryResponse{
		EntryId: entry.ID,
		Success: true,
	}, nil
}

func (s *ledgerGRPCServer) RecordLedgerEntries(ctx context.Context, req *pb.RecordEntriesRequest) (*pb.RecordEntriesResponse, error) {
	entries := make([]*model.LedgerEntry, 0, len(req.GetEntries()))
	for _, e := range req.GetEntries() {
		amount, err := decimal.NewFromString(e.GetAmount())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid amount: %v", err)
		}
		entries = append(entries, &model.LedgerEntry{
			ID:            uuid.New().String(),
			TransactionID: e.GetTransactionId(),
			WalletID:      e.GetWalletId(),
			EntryType:     e.GetType(),
			Amount:        amount,
		})
	}

	if err := s.repo.CreateBatch(ctx, entries); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record ledger entries: %v", err)
	}

	return &pb.RecordEntriesResponse{Success: true}, nil
}

func (s *ledgerGRPCServer) GetBalanceFromLedger(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	balance, err := s.repo.GetBalanceByWalletID(ctx, req.GetWalletId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to calculate balance: %v", err)
	}

	return &pb.BalanceResponse{
		CalculatedBalance: balance.String(),
	}, nil
}

func (s *ledgerGRPCServer) GetEntriesByWalletID(ctx context.Context, req *pb.GetEntriesRequest) (*pb.EntriesResponse, error) {
	entries, err := s.repo.GetEntriesByWalletID(ctx, req.GetWalletId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get ledger entries: %v", err)
	}

	pbEntries := make([]*pb.LedgerEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, &pb.LedgerEntry{
			Id:            e.ID,
			TransactionId: e.TransactionID,
			WalletId:      e.WalletID,
			EntryType:     e.EntryType,
			Amount:        e.Amount.String(),
			CreatedAt:     e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &pb.EntriesResponse{Entries: pbEntries}, nil
}
