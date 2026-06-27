package middleware

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/gin-gonic/gin"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the role from AuthMiddleware
		userRole, exists := c.Get("role")

		if !exists {
			c.Error(customErr.NewAppError(http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this api."))
			c.Abort()
			return
		}

		// Check whether user role is registered in allowedRoles
		roleStr := userRole.(string)
		isAllowed := false
		for _, role := range allowedRoles {
			if roleStr == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.Error(customErr.NewAppError(http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You don't have permission to this api."))
			c.Abort()
			return
		}

		c.Next()
	}
}
