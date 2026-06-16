package service

import (
	"context"
	"net/http"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error) {
	// 1. check if the email is already registered
	existing, _ := s.repo.GetByEmail(ctx, req.Email)
	if existing != nil {
		// return custom AppError
		return nil, customError.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "this email already registered.")
	}

	// hash the password with bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		// return internal server error
		return nil, customError.ErrInternalServer
	}

	// 2. create new user object
	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hashedBytes),
	}

	// 3. store it to the database
	if err := s.repo.Create(ctx, user); err != nil {
		// return internal server error
		return nil, customError.ErrInternalServer
	}

	return s.repo.GetByID(ctx, user.ID)
}

func (s *userService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, id)

	if err != nil {
		return nil, customError.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	return u, nil
}

func (s *userService) UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, customError.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	user.FullName = req.FullName
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, customError.ErrInternalServer
	}

	return s.repo.GetByID(ctx, id)
}

func (s *userService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	// find by email
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, customError.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// verify the hash password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, customError.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// generate access token 15 minutes
	accessToken, err := auth.GenerateToken(user.ID, user.Email, 15*time.Minute)
	if err != nil {
		return nil, customError.ErrInternalServer
	}

	// generate refresh token 7 days
	refreshToken, err := auth.GenerateToken(user.ID, user.Email, 7*24*time.Hour)
	if err != nil {
		return nil, customError.ErrInternalServer
	}

	// return the tokens
	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
