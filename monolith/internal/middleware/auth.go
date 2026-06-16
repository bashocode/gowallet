package middleware

import (
	"net/http"
	"strings"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Error(customError.NewAppError(http.StatusUnauthorized, "MISSING_TOKEN", "auth token is missing."))
			c.Abort()
			return
		}

		// split Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Error(customError.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid, should be Bearer <token>."))
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.Error(customError.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired."))
			c.Abort()
			return
		}

		// save to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)

		c.Next()
	}
}
