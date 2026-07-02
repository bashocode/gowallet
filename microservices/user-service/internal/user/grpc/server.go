package grpc

import (
	"context"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userGRPCServer struct {
	pb.UnimplementedUserServiceServer
	repo repository.UserRepository
}

func NewUserGRPCServer(repo repository.UserRepository) pb.UserServiceServer {
	return &userGRPCServer{repo: repo}
}

func (s *userGRPCServer) GetUserByID(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	logger.Log.Info("gRPC Request: GetUserByID", "userID", req.GetId())

	u, err := s.repo.GetByID(ctx, req.GetId())
	if err != nil {
		logger.Log.Warn("gRPC GetUserByID failed", "userID", req.GetId(), "error", err)
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &pb.UserResponse{
		Id:       u.ID,
		FullName: u.FullName,
		Email:    u.Email,
		Role:     u.Role,
	}, nil
}

func (s *userGRPCServer) GetUserByEmail(ctx context.Context, req *pb.GetUserByEmailRequest) (*pb.UserResponse, error) {
	logger.Log.Info("gRPC Request: GetUserByEmail", "email", req.GetEmail())

	u, err := s.repo.GetByEmail(ctx, req.GetEmail())
	if err != nil {
		logger.Log.Warn("gRPC GetUserByEmail failed", "email", req.GetEmail(), "error", err)
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &pb.UserResponse{
		Id:           u.ID,
		FullName:     u.FullName,
		Email:        u.Email,
		PasswordHash: u.PasswordHash, // Sent securely via internal gRPC
		Role:         u.Role,
	}, nil
}
