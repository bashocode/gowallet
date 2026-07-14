package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	otpGenerator "github.com/bashocode/gowallet/microservices/user-service/internal/otp/generator"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/publisher"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error)
	UpdateAvatar(ctx context.Context, id string, path string) error
	DeleteAccount(ctx context.Context, id string) error
	GenerateAndSendOTP(ctx context.Context, userID string, email string, otpType string) error
	VerifyEmail(ctx context.Context, userID string, code string) error
	RequestPasswordReset(ctx context.Context, email string) error
	VerifyPasswordReset(ctx context.Context, email string, code string) (string, error)
	ResetPassword(ctx context.Context, id string, newPassword string) error
	GetAllUsers(ctx context.Context, params model.PaginationParams) ([]*model.User, *model.PaginationMeta, error)
}

type userService struct {
	db                     *sql.DB
	rdb                    *redis.Client
	userRepo               repository.UserRepository
	walletClient           pbWallet.WalletServiceClient
	otpRepo                repository.OTPRepository
	rtRepo                 repository.RefreshTokenRepository
	notificationOutboxRepo repository.NotificationOutboxRepository
	baseURL                string
}

func NewUserService(
	db *sql.DB,
	rdb *redis.Client,
	uRepo repository.UserRepository,
	wClient pbWallet.WalletServiceClient,
	otpRepo repository.OTPRepository,
	notificationOutboxRepo repository.NotificationOutboxRepository,
	baseURL string,
) UserService {
	return &userService{
		db:                     db,
		rdb:                    rdb,
		userRepo:               uRepo,
		walletClient:           wClient,
		otpRepo:                otpRepo,
		rtRepo:                 repository.NewMySQLRefreshTokenRepository(db),
		notificationOutboxRepo: notificationOutboxRepo,
		baseURL:                baseURL,
	}
}

func (s *userService) Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error) {
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "this email already registered.")
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hashedBytes),
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback()

	if err := s.userRepo.CreateTx(ctx, tx, user); err != nil {
		return nil, customErr.ErrInternalServer
	}

	_, err = s.walletClient.CreateWallet(ctx, &pbWallet.CreateWalletRequest{
		UserId: user.ID,
	})
	if err != nil {
		logger.Log.Error("failed to create wallet via gRPC", "error", err)
		return nil, customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	if err := s.GenerateAndSendOTP(ctx, user.ID, user.Email, "email_verification"); err != nil {
		logger.Log.Error("failed to generate and send otp during registration", "error", err)
	}

	return s.userRepo.GetByID(ctx, user.ID)
}

func (s *userService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	u, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}
	return u, nil
}

func (s *userService) UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	user.FullName = req.FullName
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, customErr.ErrInternalServer
	}

	return s.userRepo.GetByID(ctx, id)
}

func (s *userService) UpdateAvatar(ctx context.Context, id string, path string) error {
	return s.userRepo.UpdateAvatar(ctx, id, path)
}

func (s *userService) DeleteAccount(ctx context.Context, id string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	if err := s.userRepo.SoftDelete(ctx, user.ID); err != nil {
		return customErr.ErrInternalServer
	}
	return nil
}

func (s *userService) VerifyEmail(ctx context.Context, userID string, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return customErr.ErrInternalServer
	}
	defer tx.Rollback()

	otp, err := s.otpRepo.GetActiveOTPTx(ctx, tx, userID, code, "email_verification")
	if err != nil {
		return customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	if err := s.userRepo.UpdateVerificationStatusTx(ctx, tx, userID, true); err != nil {
		return customErr.ErrInternalServer
	}

	if err := s.otpRepo.MarkAsUsedTx(ctx, tx, otp.ID); err != nil {
		return customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		return customErr.ErrInternalServer
	}
	return nil
}

func (s *userService) GenerateAndSendOTP(ctx context.Context, userID string, emailAddr string, otpType string) error {
	otpCode, err := otpGenerator.GenerateOTP(6)
	if err != nil {
		return customErr.ErrInternalServer
	}

	var subject string
	var body string
	switch otpType {
	case "email_verification":
		subject = "GoWallet - Verify Your Email"
		body = fmt.Sprintf("Please verify your email by clicking the following link:\n\n%s/api/v1/users/verify-email?user_id=%s&code=%s\n\nThis link will expire in 15 minutes.\n\nThank you!", s.baseURL, userID, otpCode)
	case "password_reset":
		subject = "GoWallet - Reset Your Password"
		body = fmt.Sprintf("Your password reset code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", otpCode)
	default:
		subject = "GoWallet - Security Code"
		body = fmt.Sprintf("Your code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", otpCode)
	}

	eventID := uuid.New().String()
	evt := publisher.NotificationEvent{
		EventID:    eventID,
		To:         emailAddr,
		Subject:    subject,
		Body:       body,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		logger.Log.Error("failed to marshal notification event", "error", err)
		return customErr.ErrInternalServer
	}

	// Atomically save OTP + outbox event in a single transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Log.Error("failed to begin transaction for OTP", "error", err)
		return customErr.ErrInternalServer
	}
	defer tx.Rollback()

	otp := &model.OTP{
		ID:        uuid.New().String(),
		UserID:    userID,
		Code:      otpCode,
		Type:      otpType,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}
	if err := s.otpRepo.CreateTx(ctx, tx, otp); err != nil {
		logger.Log.Error("failed to save otp in tx", "error", err)
		return customErr.ErrInternalServer
	}

	outboxEvent := &model.NotificationOutboxEvent{
		ID:          eventID,
		EventType:   "notification.send_email",
		AggregateID: userID,
		Payload:     payload,
		Status:      "pending",
		Attempts:    0,
	}
	if err := s.notificationOutboxRepo.CreateTx(ctx, tx, outboxEvent); err != nil {
		logger.Log.Error("failed to save notification outbox event in tx", "error", err)
		return customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		logger.Log.Error("failed to commit OTP+outbox transaction", "error", err)
		return customErr.ErrInternalServer
	}

	return nil
}

func (s *userService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmailNoErrorNotFound(ctx, email)
	if err != nil {
		return customErr.ErrInternalServer
	}
	if user == nil {
		return nil
	}

	return s.GenerateAndSendOTP(ctx, user.ID, user.Email, "password_reset")
}

func (s *userService) VerifyPasswordReset(ctx context.Context, email string, code string) (string, error) {
	user, err := s.userRepo.GetByEmailNoErrorNotFound(ctx, email)
	if err != nil || user == nil {
		return "", customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", customErr.ErrInternalServer
	}
	defer tx.Rollback()

	otp, err := s.otpRepo.GetActiveOTPTx(ctx, tx, user.ID, code, "password_reset")
	if err != nil {
		return "", customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	if err := s.otpRepo.MarkAsUsedTx(ctx, tx, otp.ID); err != nil {
		return "", customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		return "", customErr.ErrInternalServer
	}

	return user.ID, nil
}

func (s *userService) ResetPassword(ctx context.Context, id string, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return customErr.ErrInternalServer
	}

	if err := s.userRepo.UpdatePassword(ctx, id, string(hashedPassword)); err != nil {
		return customErr.ErrInternalServer
	}

	if err := s.rtRepo.RevokeAllByUserID(ctx, id); err != nil {
		return customErr.ErrInternalServer
	}
	return nil
}

func (s *userService) GetAllUsers(ctx context.Context, params model.PaginationParams) ([]*model.User, *model.PaginationMeta, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	users, total, err := s.userRepo.GetAll(ctx, params)
	if err != nil {
		logger.Log.Error("Failed to fetch all users", "error", err)
		return nil, nil, customErr.ErrInternalServer
	}

	totalPage := int(math.Ceil(float64(total) / float64(params.Limit)))
	if totalPage == 0 {
		totalPage = 1
	}

	meta := &model.PaginationMeta{
		Page:      params.Page,
		Limit:     params.Limit,
		Total:     total,
		TotalPage: totalPage,
	}

	return users, meta, nil
}
