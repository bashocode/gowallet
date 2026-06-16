package service

import (
	"context"
	"net/http"

	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/repository"
	"github.com/google/uuid"
)

type UserService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error)
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

	// 2. create new user object
	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: req.Password, // TODO: hashing the password
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
