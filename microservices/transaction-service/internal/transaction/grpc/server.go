package grpc

import (
	"context"
	"log/slog"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	pb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TransactionGRPCServer implements the gRPC TransactionServiceServer interface.
type TransactionGRPCServer struct {
	pb.UnimplementedTransactionServiceServer
	svc  service.TransactionService
	repo repository.TransactionRepository
}

// NewTransactionGRPCServer creates a new gRPC server with the given service.
func NewTransactionGRPCServer(svc service.TransactionService, repo repository.TransactionRepository) *TransactionGRPCServer {
	return &TransactionGRPCServer{svc: svc, repo: repo}
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

// GenerateDailyReport is triggered by scheduler-service at end of day to
// produce a CSV/aggregate report of the day's transactions. The transaction
// service owns its DB, so the report is generated here and a reference (URL or
// count) is returned to the scheduler.
func (s *TransactionGRPCServer) GenerateDailyReport(ctx context.Context, _ *pb.ReportRequest) (*pb.ReportResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] GenerateDailyReport triggered by scheduler-service")

	count, err := s.svc.GenerateDailyReport(ctx)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[gRPC] GenerateDailyReport failed", slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "daily report generation failed: %v", err)
	}

	reportURL := "" // reserved for future object-storage URL once export is wired
	logger.Log.InfoContext(ctx, "[gRPC] Daily report generated", slog.Int("total_transactions", count))
	return &pb.ReportResponse{
		ReportUrl:         reportURL,
		TotalTransactions: int32(count),
	}, nil
}

func (s *TransactionGRPCServer) FetchEventsToArchive(
	ctx context.Context,
	req *pb.FetchEventsToArchiveRequest,
) (*pb.FetchEventsToArchiveResponse, error) {
	logger.Log.InfoContext(
		ctx,
		"[gRPC] FetchEventsToArchive called",
		slog.Int64("min_age_seconds", req.MinAgeSeconds),
		slog.Int("limit", int(req.Limit)),
	)

	minAge := time.Duration(req.MinAgeSeconds) * time.Second
	events, err := s.repo.FetchEventsToArchive(ctx, minAge, int(req.Limit))
	if err != nil {
		logger.Log.ErrorContext(
			ctx,
			"[gRPC] FetchEventsToArchive failed",
			slog.Any("error", err),
		)
		return nil, status.Errorf(codes.Internal, "failed to fetch events: %v", err)
	}

	pbEvents := make([]*pb.OutboxEvent, len(events))
	for i, ev := range events {
		pbEvents[i] = &pb.OutboxEvent{
			Id:        ev.ID,
			EventType: ev.EventType,
			Payload:   ev.Payload,
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt.Format(time.RFC3339),
		}
	}

	logger.Log.InfoContext(
		ctx,
		"[gRPC] FetchEventsToArchive success",
		slog.Int("count", len(events)),
	)
	return &pb.FetchEventsToArchiveResponse{Events: pbEvents}, nil
}

func (s *TransactionGRPCServer) DeleteArchivedEvents(
	ctx context.Context,
	req *pb.DeleteArchivedEventsRequest,
) (*pb.DeleteArchivedEventsResponse, error) {
	logger.Log.InfoContext(
		ctx,
		"[gRPC] DeleteArchivedEvents called",
		slog.Int("count", len(req.Ids)),
	)

	if err := s.repo.DeleteArchivedEvents(ctx, req.Ids); err != nil {
		logger.Log.ErrorContext(ctx, "[gRPC] DeleteArchivedEvents failed", slog.Any("error", err))
		return &pb.DeleteArchivedEventsResponse{Success: false, Error: err.Error()}, nil
	}

	logger.Log.InfoContext(ctx, "[gRPC] DeleteArchivedEvents success")
	return &pb.DeleteArchivedEventsResponse{Success: true}, nil
}
