package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	"github.com/bashocode/gowallet/monolith/internal/email"
	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	otpModel "github.com/bashocode/gowallet/monolith/internal/otp/model"
	otpRepository "github.com/bashocode/gowallet/monolith/internal/otp/repository"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/repository"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletModel "github.com/bashocode/gowallet/monolith/internal/wallet/model"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
	UpdateAvatar(ctx context.Context, id string, path string) error
	DeleteAccount(ctx context.Context, id string) error
	Logout(ctx context.Context, tokenString string) error
	VerifyEmail(ctx context.Context, userID string, code string) error
}

type userService struct {
	db          *sql.DB
	rdb         *redis.Client
	userRepo    userRepo.UserRepository
	walletRepo  walletRepo.WalletRepository
	otpRepo     otpRepository.OTPRepository
	emailSender email.EmailSender
}

func NewUserService(db *sql.DB, rdb *redis.Client, uRepo repository.UserRepository, wRepo walletRepo.WalletRepository, otpRepo otpRepository.OTPRepository, emailSender email.EmailSender) UserService {
	return &userService{
		db:          db,
		rdb:         rdb,
		userRepo:    uRepo,
		walletRepo:  wRepo,
		otpRepo:     otpRepo,
		emailSender: emailSender,
	}
}

func (s *userService) Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error) {
	// 1. check if the email is already registered
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
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

	// begin transaction database
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customError.ErrInternalServer
	}

	// we should rollback if anything error or panic in the middle
	defer tx.Rollback()

	// store user to db with a tx connection
	if err := s.userRepo.CreateTx(ctx, tx, user); err != nil {
		return nil, customError.ErrInternalServer
	}

	// create wallet for the user
	wallet := &walletModel.Wallet{
		ID:       uuid.New().String(),
		UserID:   user.ID,
		Balance:  0.0,
		Currency: "IDR",
		Status:   "active",
	}

	if err := s.walletRepo.CreateTx(ctx, tx, wallet); err != nil {
		return nil, customError.ErrInternalServer
	}

	// commit the transaction if all of the step is success
	if err := tx.Commit(); err != nil {
		return nil, customError.ErrInternalServer
	}

	// Generate OTP
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, customError.ErrInternalServer
	}
	otpCode := fmt.Sprintf("%06d", n.Int64())
	fmt.Println("otp codes", otpCode)

	otpModel := &otpModel.OTP{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Code:      otpCode,
		Type:      "email_verification",
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}

	// save to db
	if err := s.otpRepo.Create(ctx, otpModel); err != nil {
		logger.Log.Error("failed to save otp", "error", err)
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		subject := "GoWallet - Verify Your Email"
		body := fmt.Sprintf("Hello %s,\n\nYour verification code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", user.FullName, otpCode)

		s.emailSender.SendEmail(bgCtx, user.Email, subject, body)
	}()

	return s.userRepo.GetByID(ctx, user.ID)
}

func (s *userService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	u, err := s.userRepo.GetByID(ctx, id)

	if err != nil {
		return nil, customError.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	return u, nil
}

func (s *userService) UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, customError.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	user.FullName = req.FullName
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, customError.ErrInternalServer
	}

	return s.userRepo.GetByID(ctx, id)
}

func (s *userService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	// find by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
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

func (s *userService) UpdateAvatar(ctx context.Context, id string, path string) error {
	return s.userRepo.UpdateAvatar(ctx, id, path)
}

func (s *userService) DeleteAccount(ctx context.Context, id string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return customError.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	if err := s.userRepo.SoftDelete(ctx, user.ID); err != nil {
		return customError.ErrInternalServer
	}

	return nil
}

func (s *userService) Logout(ctx context.Context, tokenString string) error {
	// validate token
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		return customError.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired.")
	}

	// calculate the remaining active token
	expirationTime := claims.ExpiresAt.Time
	timeLeft := time.Until(expirationTime)

	if timeLeft <= 0 {
		return nil // token already expired, no need to blacklist
	}

	// insert into redis blacklist
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	err = s.rdb.Set(ctx, blacklistKey, "logged_out", timeLeft).Err()
	if err != nil {
		return customError.ErrInternalServer
	}

	return nil
}

func (s *userService) VerifyEmail(ctx context.Context, userID string, code string) error {
	// 1. Get active OTP
	otp, err := s.otpRepo.GetActiveOTP(ctx, userID, code, "email_verification")
	if err != nil {
		// Custom AppError: OTP not found or expired
		return customError.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	// 2. Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return customError.ErrInternalServer
	}
	defer tx.Rollback()

	// 3. Mark user as verified
	if err := s.userRepo.UpdateVerificationStatusTx(ctx, tx, userID, true); err != nil {
		return customError.ErrInternalServer
	}

	// 4. Mark OTP as used
	if err := s.otpRepo.MarkAsUsedTx(ctx, tx, otp.ID); err != nil {
		return customError.ErrInternalServer
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return customError.ErrInternalServer
	}

	return nil
}
