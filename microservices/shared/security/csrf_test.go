package security_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

func TestGenerateCSRFToken(t *testing.T) {
	token, err := security.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("generated token should not be empty")
	}

	// Token should be 64 characters (32 bytes hex-encoded)
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}

	// Verify it's hex (only contains 0-9, a-f)
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("token contains non-hex character: %c", c)
		}
	}
}

func TestGenerateCSRFToken_UniqueTokens(t *testing.T) {
	tokens := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		token, err := security.GenerateCSRFToken()
		if err != nil {
			t.Fatalf("failed to generate token at iteration %d: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("duplicate token generated: %s", token)
		}
		tokens[token] = true
	}

	if len(tokens) != iterations {
		t.Errorf("expected %d unique tokens, got %d", iterations, len(tokens))
	}
}

func TestSetCSRFCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	token := "test_csrf_token_1234567890abcdef1234567890abcdef1234567890abcdef"
	security.SetCSRFCookie(c, token)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies were set")
	}

	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			csrfCookie = cookie
			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("csrf_token cookie was not set")
	}

	if csrfCookie.Value != token {
		t.Errorf("expected cookie value %s, got %s", token, csrfCookie.Value)
	}

	if csrfCookie.Path != "/" {
		t.Errorf("expected cookie path /, got %s", csrfCookie.Path)
	}

	if csrfCookie.MaxAge != 3600*24 {
		t.Errorf("expected cookie MaxAge 86400, got %d", csrfCookie.MaxAge)
	}

	if csrfCookie.Secure != true {
		t.Error("cookie should be Secure")
	}

	if csrfCookie.HttpOnly != false {
		t.Error("cookie should NOT be HttpOnly (frontend needs to read it)")
	}

	if csrfCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", csrfCookie.SameSite)
	}
}

func TestGetCSRFCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Test when cookie doesn't exist
	c.Request = httptest.NewRequest("GET", "/", nil)
	_, err := security.GetCSRFCookie(c)
	if err == nil {
		t.Error("expected error when cookie doesn't exist")
	}

	// Test when cookie exists
	expectedToken := "test_token_abc123"
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "csrf_token="+expectedToken)
	c.Request = req

	token, err := security.GetCSRFCookie(c)
	if err != nil {
		t.Errorf("unexpected error getting cookie: %v", err)
	}

	if token != expectedToken {
		t.Errorf("expected token %s, got %s", expectedToken, token)
	}
}

func TestRotateCSRFToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	newToken, err := security.RotateCSRFToken(c)
	if err != nil {
		t.Fatalf("unexpected error rotating CSRF token: %v", err)
	}

	if newToken == "" {
		t.Error("rotated token should not be empty")
	}

	if len(newToken) != 64 {
		t.Errorf("expected token length 64, got %d", len(newToken))
	}

	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			csrfCookie = cookie
			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("csrf_token cookie was not set upon rotation")
	}

	if csrfCookie.Value != newToken {
		t.Errorf("expected cookie value to match new token %s, got %s", newToken, csrfCookie.Value)
	}
}

func TestValidateCSRFToken_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	token := "valid_token_1234567890abcdef"

	// Set both cookie and header with same value
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Cookie", "csrf_token="+token)
	req.Header.Set("X-CSRF-Token", token)
	c.Request = req

	if !security.ValidateCSRFToken(c) {
		t.Error("validation should pass when cookie and header match")
	}
}

func TestValidateCSRFToken_MismatchedTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Cookie", "csrf_token=token1")
	req.Header.Set("X-CSRF-Token", "token2")
	c.Request = req

	if security.ValidateCSRFToken(c) {
		t.Error("validation should fail when tokens don't match")
	}
}

func TestValidateCSRFToken_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-CSRF-Token", "some_token")
	c.Request = req

	if security.ValidateCSRFToken(c) {
		t.Error("validation should fail when cookie is missing")
	}
}

func TestValidateCSRFToken_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Cookie", "csrf_token=some_token")
	c.Request = req

	if security.ValidateCSRFToken(c) {
		t.Error("validation should fail when header is missing")
	}
}

func TestValidateCSRFToken_EmptyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Cookie", "csrf_token=token1")
	req.Header.Set("X-CSRF-Token", "")
	c.Request = req

	if security.ValidateCSRFToken(c) {
		t.Error("validation should fail when header is empty")
	}
}

func TestValidateCSRFToken_ConstantTimeComparison(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test verifies that the function uses constant-time comparison
	// We can't directly measure timing, but we can verify the behavior

	testCases := []struct {
		name        string
		cookieToken string
		headerToken string
		shouldPass  bool
	}{
		{"exact match", "abc123", "abc123", true},
		{"case mismatch", "abc123", "ABC123", false},
		{"different length", "abc123", "abc12", false},
		{"completely different", "abc123", "xyz789", false},
		{"empty tokens", "", "", false}, // Empty tokens should NOT pass - no CSRF protection
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("POST", "/", nil)
			req.Header.Set("Cookie", "csrf_token="+tc.cookieToken)
			req.Header.Set("X-CSRF-Token", tc.headerToken)
			c.Request = req

			result := security.ValidateCSRFToken(c)
			if result != tc.shouldPass {
				t.Errorf("expected %v, got %v", tc.shouldPass, result)
			}
		})
	}
}
