package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCorrelationID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with existing X-Correlation-ID header", func(t *testing.T) {
		r := gin.New()
		r.Use(CorrelationID())

		var contextCorrelationID string
		r.GET("/test", func(c *gin.Context) {
			if val, ok := c.Request.Context().Value(logger.CorrelationIDKey).(string); ok {
				contextCorrelationID = val
			}
			c.Status(http.StatusOK)
		})

		existingID := uuid.New().String()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Correlation-ID", existingID)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, existingID, w.Header().Get("X-Correlation-ID"))
		assert.Equal(t, existingID, contextCorrelationID)
	})

	t.Run("without X-Correlation-ID header", func(t *testing.T) {
		r := gin.New()
		r.Use(CorrelationID())

		var contextCorrelationID string
		r.GET("/test", func(c *gin.Context) {
			if val, ok := c.Request.Context().Value(logger.CorrelationIDKey).(string); ok {
				contextCorrelationID = val
			}
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		respHeaderID := w.Header().Get("X-Correlation-ID")
		assert.NotEmpty(t, respHeaderID)
		assert.Equal(t, respHeaderID, contextCorrelationID)

		// Verify it is a valid UUID
		_, err := uuid.Parse(respHeaderID)
		assert.NoError(t, err)
	})
}
