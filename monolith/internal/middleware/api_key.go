package middleware

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/gin-gonic/gin"
)

func APIKeyMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" || c.GetHeader("X-API-Key") != secret {
			c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key"))
			c.Abort()
			return
		}

		c.Next()
	}
}
