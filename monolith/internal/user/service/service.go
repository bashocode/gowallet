package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	"github.com/bashocode/gowallet/monolith/internal/email"
	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/otp/generator"
	otpModel "github.com/bashocode/gowallet/monolith/internal/otp/model"
	otpRepository "github.com/bashocode/gowallet/monolith/internal/otp/repository"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/repository"
	refreshRepository "github.com/bashocode/gowallet/monolith/internal/user/repository"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletModel "github.com/bashocode/gowallet/monolith/internal/wallet/model"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type UserService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
	UpdateAvatar(ctx context.Context, id string, path string) error
	DeleteAccount(ctx context.Context, id string) error
	Logout(ctx context.Context, tokenString string) error
	GenerateAndSendOTP(ctx context.Context, userID string, email string, otpType string) error
	VerifyEmail(ctx context.Context, userID string, code string) error
	RequestPasswordReset(ctx context.Context, email string) error
	VerifyPasswordReset(ctx context.Context, email string, code string) (string, error)
	ResetPassword(ctx context.Context, email string, newPassword string) error
	GetGoogleLoginURL() string
	HandleGoogleCallback(ctx context.Context, code string) (*model.LoginResponse, error)
	RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error)
	GetAllUsers(ctx context.Context, params model.PaginationParams) ([]*model.User, *model.PaginationMeta, error)
}

type userService struct {
	db          *sql.DB
	rdb         *redis.Client
	userRepo    userRepo.UserRepository
	walletRepo  walletRepo.WalletRepository
	otpRepo     otpRepository.OTPRepository
	rtRepo      refreshRepository.RefreshTokenRepository
	emailSender email.EmailSender
}

func NewUserService(
	db *sql.DB,
	rdb *redis.Client,
	uRepo repository.UserRepository,
	wRepo walletRepo.WalletRepository,
	otpRepo otpRepository.OTPRepository,
	emailSender email.EmailSender,
) UserService {
	return &userService{
		db:          db,
		rdb:         rdb,
		userRepo:    uRepo,
		walletRepo:  wRepo,
		otpRepo:     otpRepo,
		rtRepo:      repository.NewMySQLRefreshTokenRepository(db),
		emailSender: emailSender,
	}
}

func (s *userService) Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error) {
	// 1. check if the email is already registered
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existing != nil {
		// return custom AppError
		return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "this email already registered.")
	}

	// hash the password with bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		// return internal server error
		return nil, customErr.ErrInternalServer
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
		return nil, customErr.ErrInternalServer
	}

	// we should rollback if anything error or panic in the middle
	defer tx.Rollback()

	// store user to db with a tx connection
	if err := s.userRepo.CreateTx(ctx, tx, user); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// create wallet for the user
	wallet := &walletModel.Wallet{
		ID:       uuid.New().String(),
		UserID:   user.ID,
		Balance:  decimal.Zero,
		Currency: "IDR",
		Status:   "active",
	}

	if err := s.walletRepo.CreateTx(ctx, tx, wallet); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// commit the transaction if all of the step is success
	if err := tx.Commit(); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// Generate and send OTP
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

func (s *userService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	// find by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// verify the hash password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// generate access token 15 minutes
	accessToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// generate refresh token 7 days
	refreshToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// save token to db
	rt := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, rt); err != nil {
		return nil, customErr.ErrInternalServer
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
		return customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}

	if err := s.userRepo.SoftDelete(ctx, user.ID); err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}

func (s *userService) Logout(ctx context.Context, tokenString string) error {
	// validate token
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired.")
	}

	// revoke all refresh token from users that logged out
	if err := s.rtRepo.RevokeAllByUserID(ctx, claims.UserID); err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "REVOKE_FAILED", "Failed to revoke refresh token.")
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
		return customErr.ErrInternalServer
	}

	return nil
}

func (s *userService) VerifyEmail(ctx context.Context, userID string, code string) error {
	// 1. Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return customErr.ErrInternalServer
	}
	defer tx.Rollback()

	// 2. Get active OTP inside the transaction (acquires FOR UPDATE lock)
	otp, err := s.otpRepo.GetActiveOTPTx(ctx, tx, userID, code, "email_verification")
	if err != nil {
		// Custom AppError: OTP not found or expired
		return customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	// 3. Mark user as verified
	if err := s.userRepo.UpdateVerificationStatusTx(ctx, tx, userID, true); err != nil {
		return customErr.ErrInternalServer
	}

	// 4. Mark OTP as used
	if err := s.otpRepo.MarkAsUsedTx(ctx, tx, otp.ID); err != nil {
		return customErr.ErrInternalServer
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}

func (s *userService) GenerateAndSendOTP(ctx context.Context, userID string, emailAddr string, otpType string) error {
	otpCode, err := generator.GenerateOTP(6)
	if err != nil {
		return customErr.ErrInternalServer
	}

	otpModel := &otpModel.OTP{
		ID:        uuid.New().String(),
		UserID:    userID,
		Code:      otpCode,
		Type:      otpType,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}

	if err := s.otpRepo.Create(ctx, otpModel); err != nil {
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

		s.emailSender.SendEmail(bgCtx, emailAddr, subject, body)
	}()

	return nil
}

func (s *userService) RequestPasswordReset(ctx context.Context, email string) error {
	// find by email
	user, err := s.userRepo.GetByEmailNoErrorNotFound(ctx, email)
	if err != nil {
		return customErr.ErrInternalServer
	}
	if user == nil {
		// return nil to prevent email enumeration attacks
		return nil
	}

	return s.GenerateAndSendOTP(ctx, user.ID, user.Email, "password_reset")
}

func (s *userService) VerifyPasswordReset(ctx context.Context, email string, code string) (string, error) {
	// 1. Get user by email
	user, err := s.userRepo.GetByEmailNoErrorNotFound(ctx, email)
	if err != nil || user == nil {
		return "", customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	// 2. Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", customErr.ErrInternalServer
	}
	defer tx.Rollback()

	// 3. Get active OTP inside transaction (locks the row)
	otp, err := s.otpRepo.GetActiveOTPTx(ctx, tx, user.ID, code, "password_reset")
	if err != nil {
		return "", customErr.NewAppError(http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code.")
	}

	// 4. Mark OTP as used inside transaction
	if err := s.otpRepo.MarkAsUsedTx(ctx, tx, otp.ID); err != nil {
		return "", customErr.ErrInternalServer
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return "", customErr.ErrInternalServer
	}

	return user.ID, nil
}

func (s *userService) ResetPassword(ctx context.Context, id string, newPassword string) error {
	// hash password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return customErr.ErrInternalServer
	}

	// update password
	if err := s.userRepo.UpdatePassword(ctx, id, string(hashedPassword)); err != nil {
		return customErr.ErrInternalServer
	}

	// revoke all refresh token from user that reset the password
	if err := s.rtRepo.RevokeAllByUserID(ctx, id); err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}

// Helper struct for Google UserInfo API response
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *userService) getOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func (s *userService) GetGoogleLoginURL() string {
	oauthStateString := "random-state-string" // In production, use a secure random token stored in Redis/session to prevent CSRF
	return s.getOAuthConfig().AuthCodeURL(oauthStateString)
}

func (s *userService) HandleGoogleCallback(ctx context.Context, code string) (*model.LoginResponse, error) {
	config := s.getOAuthConfig()

	// 1. Exchange the auth code for a token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// 2. Fetch userinfo using access token
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

	// 3. Find or Create user in database
	var user *model.User
	user, err = s.userRepo.GetByOAuth(ctx, "google", googleUser.ID)
	if err != nil {
		// If user not found, register new user
		if err.Error() == "user not found" {
			// Check if email already registered via normal email/password
			existingUser, _ := s.userRepo.GetByEmail(ctx, googleUser.Email)
			if existingUser != nil {
				return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "This email is registered using password credentials. Please sign in with email and password.")
			}

			// Begin database transaction to guarantee user & wallet creation consistency
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
				PasswordHash:  "", // Null password
				AvatarURL:     &googleUser.Picture,
				IsVerified:    true, // Google verified the email
			}

			// Save user
			if err := s.userRepo.CreateTx(ctx, tx, user); err != nil {
				return nil, customErr.ErrInternalServer
			}

			// Create wallet for the new user
			wallet := &walletModel.Wallet{
				ID:       uuid.New().String(),
				UserID:   user.ID,
				Balance:  decimal.Zero,
				Currency: "IDR",
				Status:   "active",
				Version:  1,
			}
			if err := s.walletRepo.CreateTx(ctx, tx, wallet); err != nil {
				return nil, customErr.ErrInternalServer
			}

			if err := tx.Commit(); err != nil {
				return nil, customErr.ErrInternalServer
			}
		} else {
			return nil, customErr.ErrInternalServer
		}
	}

	// 4. Generate JWT token
	accessToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	refreshToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	err = s.rdb.Set(ctx, "refresh_token:"+user.ID, refreshToken, 7*24*time.Hour).Err()
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *userService) RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error) {
	// 1. look token in db
	rt, err := s.rtRepo.GetByToken(ctx, oldTokenString)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token invalid.")
	}

	// 2. TOKEN REUSE DETECTION: if revoked token reused -> HACKER DETECTED!
	if rt.Revoked {
		// revoke all active session for this user
		_ = s.rtRepo.RevokeAllByUserID(ctx, rt.UserID)
		return nil, customErr.NewAppError(http.StatusUnauthorized, "TOKEN_BREACH_DETECTED", "Token breach detected. Please login again.")
	}

	// 3. check if token is expired
	if time.Now().After(rt.ExpiresAt) {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "EXPIRED_REFRESH_TOKEN", "Refresh token expired. Please login again.")
	}

	// 4. Revoke old token
	if err := s.rtRepo.Revoke(ctx, oldTokenString); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 5. Get user detail to generate new JWT
	user, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 6. Generate Access Token & New Refresh Token (Rotation)
	newAccessToken, err := auth.GenerateToken(user.ID, user.Email, user.Role, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	newRefreshTokenString, err := auth.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 7. Save new Refresh Token to Database
	newRT := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     newRefreshTokenString,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, newRT); err != nil {
		return nil, customErr.ErrInternalServer
	}

	return &model.LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenString,
	}, nil
}

func (s *userService) GetAllUsers(ctx context.Context, params model.PaginationParams) ([]*model.User, *model.PaginationMeta, error) {
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
