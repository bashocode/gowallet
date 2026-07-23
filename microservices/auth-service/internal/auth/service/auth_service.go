package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/model"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/repository"
	sharedAuth "github.com/bashocode/gowallet/microservices/shared/auth"
	"github.com/bashocode/gowallet/microservices/shared/config"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type AuthService interface {
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
	RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error)
	Logout(ctx context.Context, tokenString string) error
	GetGoogleLoginURL(ctx context.Context) (string, error)
	HandleGoogleCallback(ctx context.Context, code string, state string) (*model.LoginResponse, error)
}

type authService struct {
	rdb          *redis.Client
	rtRepo       repository.RefreshTokenRepository
	userClient   pb.UserServiceClient
	walletClient pbWallet.WalletServiceClient
}

func NewAuthService(rdb *redis.Client, rtRepo repository.RefreshTokenRepository, userClient pb.UserServiceClient, walletClient pbWallet.WalletServiceClient) AuthService {
	return &authService{
		rdb:          rdb,
		rtRepo:       rtRepo,
		userClient:   userClient,
		walletClient: walletClient,
	}
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	// Call User Service via gRPC
	userResp, err := s.userClient.GetUserByEmail(ctx, &pb.GetUserByEmailRequest{Email: req.Email})
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// check if user already verify email
	if !userResp.GetIsVerified() {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "EMAIL_NOT_VERIFIED", "Email not verified. Please verify your email.")
	}

	// verify the hash password
	err = bcrypt.CompareHashAndPassword([]byte(userResp.GetPasswordHash()), []byte(req.Password))
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// generate access token 15 minutes
	accessToken, err := sharedAuth.GenerateTokenWithType(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), "access", 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// generate refresh token 7 days
	refreshToken, err := sharedAuth.GenerateTokenWithType(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), "refresh", 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// save hashed token to db
	rt := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    userResp.GetId(),
		Token:     hashToken(refreshToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, rt); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// return the raw tokens to user
	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error) {
	hashedOldToken := hashToken(oldTokenString)

	// 1. Look up token in db
	rt, err := s.rtRepo.GetByToken(ctx, hashedOldToken)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token invalid.")
	}

	// 2. Token breach detection
	if rt.Revoked {
		_ = s.rtRepo.RevokeAllByUserID(ctx, rt.UserID)
		return nil, customErr.NewAppError(http.StatusUnauthorized, "TOKEN_BREACH_DETECTED", "Token breach detected. Please login again.")
	}

	// 3. Check if token is expired
	if time.Now().After(rt.ExpiresAt) {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "EXPIRED_REFRESH_TOKEN", "Refresh token expired. Please login again.")
	}

	// 4. Revoke old token
	if err := s.rtRepo.Revoke(ctx, hashedOldToken); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 5. Get user details from user service via gRPC to generate new JWT
	userResp, err := s.userClient.GetUserByID(ctx, &pb.GetUserRequest{Id: rt.UserID})
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 6. Generate access token & new refresh token
	newAccessToken, err := sharedAuth.GenerateTokenWithType(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), "access", 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	newRefreshTokenString, err := sharedAuth.GenerateTokenWithType(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), "refresh", 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 7. Save new Refresh Token to Database
	newRT := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    userResp.GetId(),
		Token:     hashToken(newRefreshTokenString),
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

func (s *authService) Logout(ctx context.Context, tokenString string) error {
	// Validate token
	claims, err := sharedAuth.ValidateToken(tokenString)
	if err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired.")
	}

	// Revoke all refresh tokens from user that logged out
	if err := s.rtRepo.RevokeAllByUserID(ctx, claims.UserID); err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "REVOKE_FAILED", "Failed to revoke refresh token.")
	}

	// Calculate remaining active token time
	expirationTime := claims.ExpiresAt.Time
	timeLeft := time.Until(expirationTime)

	if timeLeft <= 0 {
		return nil
	}

	// Insert into redis blacklist
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	err = s.rdb.Set(ctx, blacklistKey, "logged_out", timeLeft).Err()
	if err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}

// --- Google OAuth ---

type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *authService) getOAuthConfig() *oauth2.Config {
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

func (s *authService) GetGoogleLoginURL(ctx context.Context) (string, error) {
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

func (s *authService) HandleGoogleCallback(ctx context.Context, code string, state string) (*model.LoginResponse, error) {
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

	var googleUser googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Try to get user by email
	userResp, err := s.userClient.GetUserByEmail(ctx, &pb.GetUserByEmailRequest{Email: googleUser.Email})
	if err != nil {
		// User not found — create via gRPC
		userResp, err = s.userClient.CreateUser(ctx, &pb.CreateUserRequest{
			FullName:      googleUser.Name,
			Email:         googleUser.Email,
			OauthProvider: "google",
			OauthId:       googleUser.ID,
			AvatarUrl:     googleUser.Picture,
		})
		if err != nil {
			logger.Log.Error("failed to create OAuth user via gRPC", "error", err)
			return nil, customErr.ErrInternalServer
		}

		// Create wallet for new user via gRPC
		_, err = s.walletClient.CreateWallet(ctx, &pbWallet.CreateWalletRequest{
			UserId: userResp.GetId(),
		})
		if err != nil {
			logger.Log.Error("failed to create wallet via gRPC for OAuth user", "error", err)
			return nil, customErr.ErrInternalServer
		}
	} else {
		// User exists — check if they registered with password (not OAuth)
		if userResp.GetPasswordHash() != "" {
			return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "This email is registered using password credentials. Please sign in with email and password.")
		}
	}

	// Generate tokens
	accessToken, err := sharedAuth.GenerateToken(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	refreshToken, err := sharedAuth.GenerateToken(userResp.GetId(), userResp.GetEmail(), userResp.GetRole(), 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	newRT := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    userResp.GetId(),
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
