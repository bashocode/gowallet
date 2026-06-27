package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	logger.InitLogger()
}

type MockLedgerService struct {
	mock.Mock
}

func (m *MockLedgerService) ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Get(1).(decimal.Decimal), args.Get(2).(decimal.Decimal), args.Error(3)
}

func (m *MockLedgerService) GetMutationHistory(ctx context.Context, userID string) ([]model.LedgerEntry, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.LedgerEntry), args.Error(1)
}

func TestGetMutations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockSvc := &MockLedgerService{}
		h := NewLedgerHandler(mockSvc)

		mockSvc.On("GetMutationHistory", mock.Anything, "user1").Return([]model.LedgerEntry{
			{ID: "entry1", WalletID: "w1", EntryType: "credit", Amount: decimal.NewFromInt(100)},
		}, nil)

		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.Use(func(c *gin.Context) {
			c.Set("user_id", "user1")
			c.Next()
		})
		r.GET("/mutations", h.GetMutations)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mutations", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
	})

	t.Run("Unauthorized", func(t *testing.T) {
		mockSvc := &MockLedgerService{}
		h := NewLedgerHandler(mockSvc)

		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.GET("/mutations", h.GetMutations)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mutations", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestReconcile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockSvc := &MockLedgerService{}
		h := NewLedgerHandler(mockSvc)

		mockSvc.On("ReconcileWalletBalance", mock.Anything, "user1").Return(true, decimal.NewFromInt(100), decimal.NewFromInt(100), nil)

		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.Use(func(c *gin.Context) {
			c.Set("user_id", "user1")
			c.Next()
		})
		r.POST("/reconcile", h.Reconcile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/reconcile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
		assert.True(t, resp["is_consistent"].(bool))
	})

	t.Run("Error", func(t *testing.T) {
		mockSvc := &MockLedgerService{}
		h := NewLedgerHandler(mockSvc)

		mockSvc.On("ReconcileWalletBalance", mock.Anything, "user1").Return(false, decimal.Zero, decimal.Zero, errors.New("db error"))

		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.Use(func(c *gin.Context) {
			c.Set("user_id", "user1")
			c.Next()
		})
		r.POST("/reconcile", h.Reconcile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/reconcile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp["success"].(bool))
	})
}
