package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitLogger()
}

func TestRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := 3
	window := 1 * time.Minute

	t.Run("under the rate limit", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		r.Use(RateLimiter(rdb, limit, window))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		// We expect ClientIP to default to 192.0.2.1 or empty/localhost.
		// Gin's Engine.ServeHTTP sets RemoteAddr. Let's force RemoteAddr/ClientIP.
		req.RemoteAddr = "192.168.1.1:1234"
		ip := "192.168.1.1"

		currentTime := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("rate_limit:%s:%d", ip, currentTime)

		mockRedis.ExpectIncr(key).SetVal(2) // under limit (3)
		mockRedis.ExpectExpire(key, window*2).SetVal(true)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})

	t.Run("exceeding the rate limit", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		// Let's use the ErrorHandler middleware too, so we check if the correct AppError response is rendered
		r.Use(ErrorHandler())
		r.Use(RateLimiter(rdb, limit, window))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.2:1234"
		ip := "192.168.1.2"

		currentTime := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("rate_limit:%s:%d", ip, currentTime)

		mockRedis.ExpectIncr(key).SetVal(4) // above limit (3)
		mockRedis.ExpectExpire(key, window*2).SetVal(true)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "RATE_LIMIT_EXCEEDED")
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})

	t.Run("redis pipeline failure", func(t *testing.T) {
		rdb, mockRedis := redismock.NewClientMock()
		r := gin.New()
		r.Use(ErrorHandler())
		r.Use(RateLimiter(rdb, limit, window))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.3:1234"
		ip := "192.168.1.3"

		currentTime := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("rate_limit:%s:%d", ip, currentTime)

		mockRedis.ExpectIncr(key).SetVal(0)
		mockRedis.ExpectExpire(key, window*2).SetErr(errors.New("redis failure"))

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "INTERNAL_SERVER_ERROR")
		assert.NoError(t, mockRedis.ExpectationsWereMet())
	})
}
