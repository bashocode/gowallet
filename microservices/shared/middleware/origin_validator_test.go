package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/gin-gonic/gin"
)

func setupOriginValidatorRouter(allowedOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.OriginValidator(allowedOrigins))

	r.GET("/api/v1/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "public"})
	})

	r.POST("/api/v1/action", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	return r
}

func TestOriginValidator_AllowedOrigin_Accepted(t *testing.T) {
	allowedOrigins := []string{
		"https://wallet.example.com",
		"https://admin.wallet.example.com",
	}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Origin", "https://wallet.example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed origin, got %d", w.Code)
	}
}

func TestOriginValidator_DisallowedOrigin_Rejected(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed origin, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "ORIGIN_NOT_ALLOWED") {
		t.Errorf("expected ORIGIN_NOT_ALLOWED error code, got: %s", body)
	}
}

func TestOriginValidator_AllowedReferer_Accepted(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Referer", "https://wallet.example.com/dashboard")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed referer, got %d", w.Code)
	}
}

func TestOriginValidator_DisallowedReferer_Rejected(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Referer", "https://evil.com/attack")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed referer, got %d", w.Code)
	}
}

func TestOriginValidator_NoOriginOrReferer_GET_Allowed(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("GET", "/api/v1/data", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET without Origin/Referer should be allowed, got %d", w.Code)
	}
}

func TestOriginValidator_NoOriginOrReferer_POST_Rejected(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST without Origin/Referer should be rejected, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "ORIGIN_MISSING") {
		t.Errorf("expected ORIGIN_MISSING error code, got: %s", body)
	}
}

func TestOriginValidator_MultipleAllowedOrigins(t *testing.T) {
	allowedOrigins := []string{
		"https://wallet.example.com",
		"https://admin.wallet.example.com",
		"http://localhost:3000",
	}
	r := setupOriginValidatorRouter(allowedOrigins)

	testCases := []struct {
		origin   string
		expected int
	}{
		{"https://wallet.example.com", http.StatusOK},
		{"https://admin.wallet.example.com", http.StatusOK},
		{"http://localhost:3000", http.StatusOK},
		{"https://evil.com", http.StatusForbidden},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("POST", "/api/v1/action", nil)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != tc.expected {
			t.Errorf("origin %s: expected %d, got %d", tc.origin, tc.expected, w.Code)
		}
	}
}

func TestOriginValidator_OriginPriorityOverReferer(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Origin", "https://wallet.example.com")
	req.Header.Set("Referer", "https://evil.com/attack")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Origin header should take priority over Referer, got %d", w.Code)
	}
}

func TestOriginValidator_RefererWithPath_ExtractsOrigin(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Referer", "https://wallet.example.com/dashboard/transactions?id=123")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("should extract origin from referer URL, got %d", w.Code)
	}
}

func TestOriginValidator_InvalidReferer_Rejected(t *testing.T) {
	allowedOrigins := []string{"https://wallet.example.com"}
	r := setupOriginValidatorRouter(allowedOrigins)

	req := httptest.NewRequest("POST", "/api/v1/action", nil)
	req.Header.Set("Referer", "not-a-valid-url")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("invalid referer should be rejected, got %d", w.Code)
	}
}
