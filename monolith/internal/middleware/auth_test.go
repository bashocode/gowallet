package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitLogger()
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing authorization header", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(AuthMiddleware(rdb))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "MISSING_TOKEN", errField["code"])
	})

	t.Run("invalid token format", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(AuthMiddleware(rdb))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "INVALID_TOKEN", errField["code"])
		assert.Equal(t, "token is invalid, should be Bearer <token>.", errField["message"])
	})

	t.Run("token is blacklisted", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(AuthMiddleware(rdb))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		token := "someblacklistedtoken"
		blacklistKey := fmt.Sprintf("blacklist:%s", token)
		// Exists returns count of keys. In redismock we return 1 (exists).
		mockRedis.ExpectExists(blacklistKey).SetVal(1)

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "TOKEN_REVOKED", errField["code"])
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})

	t.Run("token is invalid or expired", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(AuthMiddleware(rdb))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		invalidToken := "invalid.jwt.token"
		blacklistKey := fmt.Sprintf("blacklist:%s", invalidToken)
		mockRedis.ExpectExists(blacklistKey).SetVal(0)

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+invalidToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "INVALID_TOKEN", errField["code"])
		assert.Equal(t, "token is invalid or expired.", errField["message"])
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})

	t.Run("token is valid", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(AuthMiddleware(rdb))

		var ctxUserID, ctxEmail, ctxToken string
		r.GET("/test", func(c *gin.Context) {
			ctxUserID = c.GetString("user_id")
			ctxEmail = c.GetString("email")
			ctxToken = c.GetString("token_string")
			c.Status(http.StatusOK)
		})

		expectedUserID := "user123"
		expectedEmail := "test@example.com"
		expectedRole := "user"

		token, err := auth.GenerateToken(expectedUserID, expectedEmail, expectedRole, 15*time.Minute)
		assert.NoError(t, err)

		blacklistKey := fmt.Sprintf("blacklist:%s", token)
		mockRedis.ExpectExists(blacklistKey).SetVal(0)

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, expectedUserID, ctxUserID)
		assert.Equal(t, expectedEmail, ctxEmail)
		assert.Equal(t, token, ctxToken)
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})
}
