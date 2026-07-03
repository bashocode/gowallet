package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/auth"
	"github.com/bashocode/gowallet/microservices/shared/config"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/user-service/internal/email"
	otpGenerator "github.com/bashocode/gowallet/microservices/user-service/internal/otp/generator"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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
	GetGoogleLoginURL(ctx context.Context) (string, error)
	HandleGoogleCallback(ctx context.Context, code string, state string) (*model.LoginResponse, error)
	GetAllUsers(ctx context.Context, params model.PaginationParams) ([]*model.User, *model.PaginationMeta, error)
}

type userService struct {
	db          *sql.DB
	rdb         *redis.Client
	userRepo    repository.UserRepository
	walletClient pbWallet.WalletServiceClient
	otpRepo      repository.OTPRepository
	rtRepo       repository.RefreshTokenRepository
	emailSender  email.EmailSender
}

func NewUserService(
	db *sql.DB,
	rdb *redis.Client,
	uRepo repository.UserRepository,
	wClient pbWallet.WalletServiceClient,
	otpRepo repository.OTPRepository,
	emailSender email.EmailSender,
) UserService {
	return &userService{
		db:           db,
		rdb:          rdb,
		userRepo:     uRepo,
		walletClient: wClient,
		otpRepo:      otpRepo,
		rtRepo:       repository.NewMySQLRefreshTokenRepository(db),
		emailSender:  emailSender,
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

	otp := &model.OTP{
		ID:        uuid.New().String(),
		UserID:    userID,
		Code:      otpCode,
		Type:      otpType,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}

	if err := s.otpRepo.Create(ctx, otp); err != nil {
		logger.Log.Error("failed to save otp", "error", err)
		return customErr.ErrInternalServer
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var subject string
		var body string

		switch otpType {
		case "email_verification":
			subject = "GoWallet - Verify Your Email"
			body = fmt.Sprintf("Your verification code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", otpCode)
		case "password_reset":
			subject = "GoWallet - Reset Your Password"
			body = fmt.Sprintf("Your password reset code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", otpCode)
		default:
			subject = "GoWallet - Security Code"
			body = fmt.Sprintf("Your code is %s\n\nThis code will expire in 15 minutes.\n\nThank you!", otpCode)
		}

		_ = s.emailSender.SendEmail(bgCtx, emailAddr, subject, body)
	}()

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

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *userService) getOAuthConfig() *oauth2.Config {
	cfg := config.LoadConfig()

	return &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func (s *userService) GetGoogleLoginURL(ctx context.Context) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)

	stateKey := fmt.Sprintf("oauth:state:%s", state)
	err := s.rdb.Set(ctx, stateKey, "valid", 10*time.Minute).Err()
	if err != nil {
		return "", err
	}

	url := s.getOAuthConfig().AuthCodeURL(state)
	return url, nil
}

func (s *userService) HandleGoogleCallback(ctx context.Context, code string, state string) (*model.LoginResponse, error) {
	stateKey := fmt.Sprintf("oauth:state:%s", state)
	val, err := s.rdb.Get(ctx, stateKey).Result()
	if err != nil || val != "valid" {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_STATE", "invalid or expired OAuth state - possible CSRF attack")
	}

	s.rdb.Del(ctx, stateKey)

	config := s.getOAuthConfig()

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var googleUser GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	var user *model.User
	user, err = s.userRepo.GetByOAuth(ctx, "google", googleUser.ID)
	if err != nil {
		if err.Error() == "user not found" {
			existingUser, _ := s.userRepo.GetByEmail(ctx, googleUser.Email)
			if existingUser != nil {
				return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "This email is registered using password credentials. Please sign in with email and password.")
			}

			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				return nil, customErr.ErrInternalServer
			}
			defer tx.Rollback()

			provider := "google"
			user = &model.User{
				ID:            uuid.New().String(),
				FullName:      googleUser.Name,
				Email:         googleUser.Email,
				OAuthProvider: &provider,
				OAuthID:       &googleUser.ID,
				PasswordHash:  "",
				AvatarURL:     &googleUser.Picture,
				IsVerified:    true,
			}

			if err := s.userRepo.CreateTx(ctx, tx, user); err != nil {
				return nil, customErr.ErrInternalServer
			}

			_, err = s.walletClient.CreateWallet(ctx, &pbWallet.CreateWalletRequest{
				UserId: user.ID,
			})
			if err != nil {
				logger.Log.Error("failed to create wallet via gRPC for OAuth user", "error", err)
				return nil, customErr.ErrInternalServer
			}

			if err := tx.Commit(); err != nil {
				return nil, customErr.ErrInternalServer
			}
		} else {
			return nil, customErr.ErrInternalServer
		}
	}

	accessToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	refreshToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	newRT := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, newRT); err != nil {
		return nil, customErr.ErrInternalServer
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
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
