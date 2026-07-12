package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/middleware"
	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	logger.InitLogger()
}

type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Wallet), args.Error(1)
}

func TestGetMyWallet_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &MockWalletService{}
	h := NewWalletHandler(mockSvc)

	w := &model.Wallet{ID: "w1", UserID: "user1"}
	mockSvc.On("GetWalletByUserID", mock.Anything, "user1").Return(w, nil)

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user1")
		c.Next()
	})
	r.GET("/wallets/me", h.GetMyWallet)

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/wallets/me", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestGetMyWallet_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &MockWalletService{}
	h := NewWalletHandler(mockSvc)

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.GET("/wallets/me", h.GetMyWallet)

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/wallets/me", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetMyWallet_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := &MockWalletService{}
	h := NewWalletHandler(mockSvc)

	mockSvc.On("GetWalletByUserID", mock.Anything, "user1").Return(nil, errors.New("db error"))

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user1")
		c.Next()
	})
	r.GET("/wallets/me", h.GetMyWallet)

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/wallets/me", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func (m *MockWalletService) ValidateEmailInquiry(ctx context.Context, email string) (*model.WalletInquiry, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.WalletInquiry), args.Error(1)
}
