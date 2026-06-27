package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/middleware"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	logger.InitLogger()
}

type MockTransactionService struct {
	mock.Mock
}

func (m *MockTransactionService) Transfer(ctx context.Context, senderUserID string, req model.TransferRequest) (*model.Transaction, error) {
	args := m.Called(ctx, senderUserID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transaction), args.Error(1)
}

func (m *MockTransactionService) GetHistory(ctx context.Context, userID string, params model.PaginationParams) ([]model.Transaction, *model.PaginationMeta, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]model.Transaction), args.Get(1).(*model.PaginationMeta), args.Error(2)
}

func (m *MockTransactionService) TopUp(ctx context.Context, userID string, req model.TopUpRequest) (*model.Transaction, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transaction), args.Error(1)
}


func TestTransferHandler_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid request should bind correctly", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		router.POST("/transfer", h.Transfer)

		reqBody := map[string]interface{}{
			"receiver_email":  "receiver@example.com",
			"amount":          50000.50,
			"description":     "split bill",
			"idempotency_key": "unique-key-123",
		}
		body, _ := json.Marshal(reqBody)

		expectedTx := &model.Transaction{
			ID:               "tx-123",
			ReceiverWalletID: "wallet-abc",
			Amount:           decimal.NewFromFloat(50000.50),
		}

		mockSvc.On("Transfer", mock.Anything, "user-123", mock.MatchedBy(func(req model.TransferRequest) bool {
			return req.ReceiverEmail == "receiver@example.com" &&
				req.Amount.Equal(decimal.NewFromFloat(50000.50)) &&
				req.IdempotencyKey == "unique-key-123"
		})).Return(expectedTx, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Invalid negative amount should fail validation", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		router.POST("/transfer", h.Transfer)

		reqBody := map[string]interface{}{
			"receiver_email":  "receiver@example.com",
			"amount":          -10.0,
			"description":     "split bill",
			"idempotency_key": "unique-key-123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTopUpHandler_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid topup request should bind correctly", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		router.POST("/topup", h.TopUp)

		reqBody := map[string]interface{}{
			"amount":          100000.0,
			"idempotency_key": "unique-topup-123",
		}
		body, _ := json.Marshal(reqBody)

		expectedTx := &model.Transaction{
			ID:               "tx-topup-123",
			ReceiverWalletID: "wallet-123",
			Amount:           decimal.NewFromFloat(100000.0),
		}

		mockSvc.On("TopUp", mock.Anything, "user-123", mock.MatchedBy(func(req model.TopUpRequest) bool {
			return req.Amount.Equal(decimal.NewFromFloat(100000.0)) &&
				req.IdempotencyKey == "unique-topup-123"
		})).Return(expectedTx, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/topup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Invalid negative topup amount should fail validation", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		router.POST("/topup", h.TopUp)

		reqBody := map[string]interface{}{
			"amount":          -50.0,
			"idempotency_key": "unique-topup-123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/topup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_InvalidUserIDType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Transfer should return unauthorized if user_id is not string", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 123) // int, not string
			c.Next()
		})
		router.POST("/transfer", h.Transfer)

		reqBody := map[string]interface{}{
			"receiver_email":  "receiver@example.com",
			"amount":          50000.50,
			"description":     "split bill",
			"idempotency_key": "unique-key-123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("GetHistory should return unauthorized if user_id is not string", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 123) // int, not string
			c.Next()
		})
		router.GET("/history", h.GetHistory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/history", nil)

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("TopUp should return unauthorized if user_id is not string", func(t *testing.T) {
		mockSvc := new(MockTransactionService)
		h := NewTransactionHandler(mockSvc)

		router := gin.New()
		router.Use(middleware.ErrorHandler())
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 123) // int, not string
			c.Next()
		})
		router.POST("/topup", h.TopUp)

		reqBody := map[string]interface{}{
			"amount":          100000.0,
			"idempotency_key": "unique-topup-123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/topup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

