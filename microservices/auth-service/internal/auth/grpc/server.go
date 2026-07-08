package grpc

import (
	"context"
	"log/slog"

	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pb "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authGRPCServer struct {
	pb.UnimplementedAuthServiceServer
	rtRepo repository.RefreshTokenRepository
}

func NewAuthGRPCServer(rtRepo repository.RefreshTokenRepository) pb.AuthServiceServer {
	return &authGRPCServer{rtRepo: rtRepo}
}

func (s *authGRPCServer) CleanupExpiredOTPs(ctx context.Context, _ *pb.CleanupRequest) (*pb.CleanupResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] CleanupExpiredOTPs triggered by scheduler-service")
	// OTPs are currently issued by user-service; auth-service has no OTP store yet.
	// Reserved for future OTP cleanup logic once OTP storage moves here.
	return &pb.CleanupResponse{DeletedCount: 0}, nil
}

func (s *authGRPCServer) CleanupExpiredRefreshTokens(ctx context.Context, _ *pb.CleanupRequest) (*pb.CleanupResponse, error) {
	logger.Log.InfoContext(ctx, "[gRPC] CleanupExpiredRefreshTokens triggered by scheduler-service")

	deleted, err := s.rtRepo.DeleteExpired(ctx)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[gRPC] Failed to delete expired refresh tokens", slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "failed to cleanup expired refresh tokens: %v", err)
	}

	logger.Log.InfoContext(ctx, "[gRPC] Expired refresh tokens cleaned", slog.Int64("deleted_count", deleted))
	return &pb.CleanupResponse{DeletedCount: int32(deleted)}, nil
}
