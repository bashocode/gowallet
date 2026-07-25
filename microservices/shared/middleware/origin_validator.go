package middleware

import (
	"net/http"
	"net/url"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/gin-gonic/gin"
)

// OriginValidator validates that the Origin or Referer header matches allowed domains.
// This provides an additional layer of CSRF defense beyond the Double Submit Cookie pattern.
func OriginValidator(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(c *gin.Context) {
		// Skip for same-origin requests without Origin header (normal browser behavior)
		origin := c.GetHeader("Origin")
		if origin == "" {
			// Check Referer as fallback
			referer := c.GetHeader("Referer")
			if referer == "" {
				// No Origin and no Referer — could be a direct API call (Postman) or an attack
				// Allow for GET (safe), block for state-changing
				if !isSafeMethod(c.Request.Method) {
					c.Error(customErr.NewAppError(http.StatusForbidden, "ORIGIN_MISSING",
						"Origin or Referer header is required for this request."))
					c.Abort()
					return
				}
				c.Next()
				return
			}
			origin = extractOrigin(referer)
		}

		if !allowed[origin] {
			c.Error(customErr.NewAppError(http.StatusForbidden, "ORIGIN_NOT_ALLOWED",
				"Request origin is not allowed."))
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractOrigin extracts the origin (scheme + host) from a referer URL.
func extractOrigin(referer string) string {
	u, err := url.Parse(referer)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
