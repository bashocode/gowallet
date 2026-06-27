package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/monolith/internal/otp/model"
	"github.com/stretchr/testify/mock"
)

type MockOTPRepository struct {
	mock.Mock
}

func (m *MockOTPRepository) Create(ctx context.Context, o *model.OTP) error {
	args := m.Called(ctx, o)
	return args.Error(0)
}

func (m *MockOTPRepository) GetActiveOTP(ctx context.Context, userID string, code string, otpType string) (*model.OTP, error) {
	args := m.Called(ctx, userID, code, otpType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.OTP), args.Error(1)
}

func (m *MockOTPRepository) GetActiveOTPTx(ctx context.Context, tx *sql.Tx, userID string, code string, otpType string) (*model.OTP, error) {
	args := m.Called(ctx, tx, userID, code, otpType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.OTP), args.Error(1)
}

func (m *MockOTPRepository) MarkAsUsed(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOTPRepository) MarkAsUsedTx(ctx context.Context, tx *sql.Tx, id string) error {
	args := m.Called(ctx, tx, id)
	return args.Error(0)
}
