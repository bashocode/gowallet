package grpc

import (
	"context"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"github.com/google/uuid"
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
		Id:         u.ID,
		FullName:   u.FullName,
		Email:      u.Email,
		Role:       u.Role,
		IsVerified: u.IsVerified,
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
		IsVerified:   u.IsVerified,
	}, nil
}

func (s *userGRPCServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.UserResponse, error) {
	logger.Log.Info("gRPC Request: CreateUser", "email", req.GetEmail())

	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.GetFullName(),
		Email:        req.GetEmail(),
		PasswordHash: "",
		Role:         "user",
		IsVerified:   true, // OAuth users are pre-verified
	}

	if req.GetOauthProvider() != "" {
		provider := req.GetOauthProvider()
		oauthID := req.GetOauthId()
		user.OAuthProvider = &provider
		user.OAuthID = &oauthID
	}

	if req.GetAvatarUrl() != "" {
		avatarURL := req.GetAvatarUrl()
		user.AvatarURL = &avatarURL
	}

	if err := s.repo.Create(ctx, user); err != nil {
		logger.Log.Warn("gRPC CreateUser failed", "email", req.GetEmail(), "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	return &pb.UserResponse{
		Id:         user.ID,
		FullName:   user.FullName,
		Email:      user.Email,
		Role:       user.Role,
		IsVerified: user.IsVerified,
	}, nil
}
