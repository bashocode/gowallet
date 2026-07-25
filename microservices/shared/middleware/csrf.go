package middleware

import (
	"net/http"
	"strings"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

// CSRFMiddleware implements the Double Submit Cookie pattern.
// - GET/HEAD/OPTIONS: ensures CSRF cookie is set (generates if missing).
// - POST/PUT/PATCH/DELETE: validates X-CSRF-Token header matches csrf_token cookie.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for webhooks (authenticated via HMAC signatures or API keys)
		if isWebhookRequest(c) {
			c.Next()
			return
		}

		// Safe methods don't modify state — just ensure the cookie exists
		if isSafeMethod(c.Request.Method) {
			ensureCSRFCookie(c)
			c.Next()
			return
		}

		// State-changing methods: validate token
		if !security.ValidateCSRFToken(c) {
			c.Error(customErr.NewAppError(
				http.StatusForbidden,
				"CSRF_TOKEN_INVALID",
				"CSRF token is missing or invalid. Include the X-CSRF-Token header matching the csrf_token cookie.",
			))
			c.Abort()
			return
		}

		c.Next()
	}
}

func isSafeMethod(method string) bool {
	return method == "GET" || method == "HEAD" || method == "OPTIONS"
}

func isWebhookRequest(c *gin.Context) bool {
	path := c.Request.URL.Path
	if strings.Contains(path, "/webhook") {
		return true
	}
	if c.GetHeader("Stripe-Signature") != "" || c.GetHeader("X-Signature") != "" || c.GetHeader("X-API-Key") != "" {
		return true
	}
	return false
}

// ensureCSRFCookie checks if the CSRF cookie exists. If not, generates and sets it.
func ensureCSRFCookie(c *gin.Context) {
	_, err := c.Cookie("csrf_token")
	if err != nil {
		token, err := security.GenerateCSRFToken()
		if err != nil {
			c.Error(customErr.ErrInternalServer)
			c.Abort()
			return
		}
		security.SetCSRFCookie(c, token)
	}
}
