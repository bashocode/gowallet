package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

// SanitizeBody middleware reads the request body, sanitizes all string fields,
// and writes the clean body back so downstream handlers read safe data.
func SanitizeBody(sanitizer *security.Sanitizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip sanitization for GET / DELETE (no body) and file uploads
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
			c.Next()
			return
		}

		// Skip webhook endpoints (e.g. Stripe, provider webhooks) to preserve exact raw bytes needed for HMAC signature verification
		if strings.HasSuffix(c.Request.URL.Path, "/webhook") || strings.Contains(c.Request.URL.Path, "/webhook") {
			c.Next()
			return
		}

		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			c.Next()
			return
		}

		// Limit JSON body size to 2MB to prevent memory exhaustion
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)

		// Read body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		defer c.Request.Body.Close()

		// Parse JSON to map
		var data map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			// Not valid JSON — let the handler deal with it
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			c.Next()
			return
		}

		// Sanitize all string fields recursively
		sanitizer.SanitizeMap(data)

		// Re-encode and replace body
		cleanBytes, err := json.Marshal(data)
		if err != nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			c.Next()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(cleanBytes))
		c.Request.ContentLength = int64(len(cleanBytes))
		c.Next()
	}
}
