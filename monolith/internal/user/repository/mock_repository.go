package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/stretchr/testify/mock"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, u *model.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockUserRepository) CreateTx(ctx context.Context, tx *sql.Tx, u *model.User) error {
	args := m.Called(ctx, tx, u)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmailNoErrorNotFound(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, u *model.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateAvatar(ctx context.Context, id string, path string) error {
	args := m.Called(ctx, id, path)
	return args.Error(0)
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateVerificationStatus(ctx context.Context, id string, verified bool) error {
	args := m.Called(ctx, id, verified)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateVerificationStatusTx(ctx context.Context, tx *sql.Tx, id string, verified bool) error {
	args := m.Called(ctx, tx, id, verified)
	return args.Error(0)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	args := m.Called(ctx, id, passwordHash)
	return args.Error(0)
}

func (m *MockUserRepository) GetByOAuth(ctx context.Context, provider, oauthID string) (*model.User, error) {
	args := m.Called(ctx, provider, oauthID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetAll(ctx context.Context, params model.PaginationParams) ([]*model.User, int64, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*model.User), args.Get(1).(int64), args.Error(2)
}

type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) Create(ctx context.Context, rt *model.RefreshToken) error {
	args := m.Called(ctx, rt)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) GetByToken(ctx context.Context, token string) (*model.RefreshToken, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

