package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitLogger()
}

func TestErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("no error occurs", func(t *testing.T) {
		r := gin.New()
		r.Use(ErrorHandler())
		r.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
	})

	t.Run("custom AppError occurs", func(t *testing.T) {
		r := gin.New()
		r.Use(ErrorHandler())
		r.GET("/test", func(c *gin.Context) {
			c.Error(customErr.NewAppError(http.StatusBadRequest, "CUSTOM_BAD_REQUEST", "Custom bad request message"))
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp["success"].(bool))

		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "CUSTOM_BAD_REQUEST", errField["code"])
		assert.Equal(t, "Custom bad request message", errField["message"])
	})

	t.Run("unhandled standard error occurs", func(t *testing.T) {
		r := gin.New()
		r.Use(ErrorHandler())
		r.GET("/test", func(c *gin.Context) {
			c.Error(errors.New("something went wrong internally"))
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp["success"].(bool))

		errField := resp["error"].(map[string]interface{})
		assert.Equal(t, "INTERNAL_SERVER_ERROR", errField["code"])
		assert.Equal(t, "Something went wrong on the server, please try again later.", errField["message"])
	})
}
