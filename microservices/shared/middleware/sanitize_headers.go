package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets HTTP response headers that help prevent XSS
// and other browser-based attacks.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Content-Security-Policy: Most powerful XSS defense.
		// 'self' = only allow scripts from same origin.
		// 'none' for inline scripts (no <script> tags, no onclick= handlers).
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self' wss:; "+ // Allow WebSocket
				"frame-ancestors 'none';")

		// X-Content-Type-Options: Prevent MIME-type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-Frame-Options: Prevent clickjacking (older browsers)
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection: Legacy browser XSS filter (modern browsers use CSP)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Limit referrer information leakage
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}
