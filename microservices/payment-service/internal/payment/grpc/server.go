package grpc

import (
	"context"
	"time"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	pb "github.com/bashocode/gowallet/microservices/payment-service/proto/payment"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentGRPCServer struct {
	pb.UnimplementedPaymentServiceServer
	outboxRepo repository.OutboxRepository
}

func NewPaymentGRPCServer(outboxRepo repository.OutboxRepository) pb.PaymentServiceServer {
	return &paymentGRPCServer{
		outboxRepo: outboxRepo,
	}
}

func (s *paymentGRPCServer) FetchEventsToArchive(
	ctx context.Context,
	req *pb.FetchEventsToArchiveRequest,
) (*pb.FetchEventsToArchiveResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] FetchEventsToArchive called", "min_age_seconds", req.MinAgeSeconds, "limit", req.Limit)

	minAge := time.Duration(req.MinAgeSeconds) * time.Second
	events, err := s.outboxRepo.FetchEventsToArchive(ctx, minAge, int(req.Limit))
	if err != nil {
		logger.Log.ErrorContext(ctx, "[gRPC] FetchEventsToArchive failed", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch outbox events: %v", err)
	}

	var pbEvents []*pb.OutboxEvent
	for _, ev := range events {
		pbEvents = append(pbEvents, &pb.OutboxEvent{
			Id:        ev.ID,
			EventType: ev.EventType,
			Payload:   string(ev.Payload),
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt.Format(time.RFC3339),
		})
	}

	return &pb.FetchEventsToArchiveResponse{Events: pbEvents}, nil
}

func (s *paymentGRPCServer) DeleteArchivedEvents(
	ctx context.Context,
	req *pb.DeleteArchivedEventsRequest,
) (*pb.DeleteArchivedEventsResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] DeleteArchivedEvents called", "count", len(req.Ids))

	err := s.outboxRepo.DeleteArchivedEvents(ctx, req.Ids)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[gRPC] DeleteArchivedEvents failed", "error", err)
		return &pb.DeleteArchivedEventsResponse{Success: false, Error: err.Error()}, nil
	}

	return &pb.DeleteArchivedEventsResponse{Success: true}, nil
}
