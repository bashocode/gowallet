package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bashocode/gowallet/monolith/internal/auth"
	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func AuthMiddleware(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Error(customErr.NewAppError(http.StatusUnauthorized, "MISSING_TOKEN", "auth token is missing."))
			c.Abort()
			return
		}

		// split Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Error(customErr.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid, should be Bearer <token>."))
			c.Abort()
			return
		}

		tokenString := parts[1]

		// check if token is in redis blacklist
		blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
		exists, err := rdb.Exists(c.Request.Context(), blacklistKey).Result()
		if err == nil && exists > 0 {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"TOKEN_REVOKED",
				"Login session has ended. Please login again.",
			))
			c.Abort()
			return
		}

		// validate token
		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.Error(customErr.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired."))
			c.Abort()
			return
		}

		// save to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("token_string", tokenString) // store for logout needs

		c.Next()
	}
}
