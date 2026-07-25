package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/security"
	"github.com/gin-gonic/gin"
)

func setupCSRFRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CSRFMiddleware())

	r.GET("/api/v1/csrf-token", func(c *gin.Context) {
		token, _ := c.Cookie("csrf_token")
		c.JSON(http.StatusOK, gin.H{"csrf_token": token})
	})

	r.POST("/api/v1/transfer", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.GET("/api/v1/balance", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"balance": 1000})
	})

	return r
}

func TestCSRF_GET_SetsCookie(t *testing.T) {
	r := setupCSRFRouter()
	req := httptest.NewRequest("GET", "/api/v1/csrf-token", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Verify CSRF cookie was set
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "csrf_token=") {
		t.Error("CSRF cookie was not set on GET request")
	}

	// Verify cookie is not HttpOnly (frontend must read it)
	if strings.Contains(setCookie, "HttpOnly") {
		t.Error("CSRF cookie should not be HttpOnly - frontend needs to read it")
	}

	// Verify SameSite attribute is set
	if !strings.Contains(setCookie, "SameSite") {
		t.Error("CSRF cookie should have SameSite attribute")
	}
}

func TestCSRF_POST_WithoutToken_Rejected(t *testing.T) {
	r := setupCSRFRouter()
	req := httptest.NewRequest("POST", "/api/v1/transfer", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (CSRF token missing), got %d", w.Code)
	}

	// Check error response
	body := w.Body.String()
	if !strings.Contains(body, "CSRF_TOKEN_INVALID") {
		t.Errorf("expected CSRF_TOKEN_INVALID error code, got: %s", body)
	}
}

func TestCSRF_POST_WithValidToken_Accepted(t *testing.T) {
	r := setupCSRFRouter()

	// Step 1: GET to obtain CSRF token cookie
	getReq := httptest.NewRequest("GET", "/api/v1/csrf-token", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	// Extract token from Set-Cookie header
	setCookie := getW.Header().Get("Set-Cookie")
	token := extractCookieValue(setCookie, "csrf_token")

	if token == "" {
		t.Fatal("failed to extract CSRF token from cookie")
	}

	// Step 2: POST with matching X-CSRF-Token header
	postReq := httptest.NewRequest("POST", "/api/v1/transfer", nil)
	postReq.Header.Set("X-CSRF-Token", token)
	postReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	postW := httptest.NewRecorder()

	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusOK {
		t.Errorf("expected 200 (valid CSRF token), got %d: %s", postW.Code, postW.Body.String())
	}

	// Verify successful response
	body := postW.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Errorf("expected success response, got: %s", body)
	}
}

func TestCSRF_POST_WithMismatchedToken_Rejected(t *testing.T) {
	r := setupCSRFRouter()

	postReq := httptest.NewRequest("POST", "/api/v1/transfer", nil)
	postReq.Header.Set("X-CSRF-Token", "attacker_forged_token")
	postReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "real_token_from_server"})
	postW := httptest.NewRecorder()

	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusForbidden {
		t.Errorf("expected 403 (token mismatch), got %d", postW.Code)
	}

	body := postW.Body.String()
	if !strings.Contains(body, "CSRF_TOKEN_INVALID") {
		t.Errorf("expected CSRF_TOKEN_INVALID error code, got: %s", body)
	}
}

func TestCSRF_POST_WithCookieOnly_Rejected(t *testing.T) {
	r := setupCSRFRouter()

	// Attacker can set cookie via subdomain or XSS, but can't set header cross-site
	postReq := httptest.NewRequest("POST", "/api/v1/transfer", nil)
	postReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "valid_looking_token"})
	// No X-CSRF-Token header set
	postW := httptest.NewRecorder()

	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusForbidden {
		t.Errorf("expected 403 (missing header), got %d", postW.Code)
	}
}

func TestCSRF_POST_WithHeaderOnly_Rejected(t *testing.T) {
	r := setupCSRFRouter()

	postReq := httptest.NewRequest("POST", "/api/v1/transfer", nil)
	postReq.Header.Set("X-CSRF-Token", "some_token")
	// No cookie set
	postW := httptest.NewRecorder()

	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusForbidden {
		t.Errorf("expected 403 (missing cookie), got %d", postW.Code)
	}
}

func TestCSRF_SafeMethods_NoTokenRequired(t *testing.T) {
	r := setupCSRFRouter()

	safeMethods := []string{"GET", "HEAD", "OPTIONS"}

	for _, method := range safeMethods {
		req := httptest.NewRequest(method, "/api/v1/balance", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Safe methods should succeed without CSRF token
		if method == "GET" && w.Code != http.StatusOK {
			t.Errorf("%s request without CSRF token should succeed, got %d", method, w.Code)
		}

		// Verify cookie is set on safe methods
		setCookie := w.Header().Get("Set-Cookie")
		if method == "GET" && !strings.Contains(setCookie, "csrf_token=") {
			t.Errorf("%s request should set CSRF cookie", method)
		}
	}
}

func TestCSRF_PUT_RequiresToken(t *testing.T) {
	r := setupCSRFRouter()
	r.PUT("/api/v1/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"updated": true})
	})

	// PUT without token should fail
	req := httptest.NewRequest("PUT", "/api/v1/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("PUT without CSRF token should return 403, got %d", w.Code)
	}
}

func TestCSRF_DELETE_RequiresToken(t *testing.T) {
	r := setupCSRFRouter()
	r.DELETE("/api/v1/account", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	})

	// DELETE without token should fail
	req := httptest.NewRequest("DELETE", "/api/v1/account", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("DELETE without CSRF token should return 403, got %d", w.Code)
	}
}

func TestCSRF_PATCH_RequiresToken(t *testing.T) {
	r := setupCSRFRouter()
	r.PATCH("/api/v1/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"patched": true})
	})

	// PATCH without token should fail
	req := httptest.NewRequest("PATCH", "/api/v1/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("PATCH without CSRF token should return 403, got %d", w.Code)
	}
}

func TestCSRF_TokenGeneration_IsRandom(t *testing.T) {
	token1, err := security.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	token2, err := security.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token1 == token2 {
		t.Error("generated tokens should be unique")
	}

	// Token should be hex-encoded (64 characters for 32 bytes)
	if len(token1) != 64 {
		t.Errorf("expected token length 64, got %d", len(token1))
	}
}

func TestCSRF_WebhookBypass(t *testing.T) {
	r := setupCSRFRouter()
	r.POST("/api/v1/payments/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"webhook": "received"})
	})

	// POST to webhook path without CSRF token should succeed
	req := httptest.NewRequest("POST", "/api/v1/payments/webhook", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("webhook request without CSRF token should succeed with 200, got %d", w.Code)
	}
}

// extractCookieValue parses a Set-Cookie header and extracts the value for the given cookie name.
func extractCookieValue(setCookie, name string) string {
	parts := strings.Split(setCookie, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, name+"=") {
			return strings.TrimPrefix(p, name+"=")
		}
	}
	return ""
}
