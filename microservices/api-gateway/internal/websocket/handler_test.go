package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(hub *Hub, rdb *redis.Client, jwtSecret string, allowedOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", WebSocketHandler(hub, rdb, jwtSecret, allowedOrigins))
	return router
}

func generateTestToken(jwtSecret, userID string, tokenType string, expiration time.Duration) string {
	token, _ := auth.GenerateTokenWithType(jwtSecret, userID, "test@example.com", "user", tokenType, expiration)
	return token
}

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single origin",
			input:    "https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "multiple origins",
			input:    "https://example.com,https://test.com,https://app.com",
			expected: []string{"https://example.com", "https://test.com", "https://app.com"},
		},
		{
			name:     "with spaces",
			input:    "https://example.com, https://test.com , https://app.com",
			expected: []string{"https://example.com", "https://test.com", "https://app.com"},
		},
		{
			name:     "with empty segments",
			input:    "https://example.com,,https://test.com",
			expected: []string{"https://example.com", "https://test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAllowedOrigins(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebSocketHandler_MissingToken(t *testing.T) {
	hub := NewHub()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, "test-secret", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Missing authentication token")
}

func TestWebSocketHandler_InvalidToken(t *testing.T) {
	hub := NewHub()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, "test-secret", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token=invalid-token", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

func TestWebSocketHandler_ExpiredToken(t *testing.T) {
	jwtSecret := "test-secret"
	userID := "user123"

	// Generate expired token
	expiredToken := generateTestToken(jwtSecret, userID, "access", -1*time.Hour)

	hub := NewHub()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, jwtSecret, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/ws?token=%s", expiredToken), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

func TestWebSocketHandler_WrongTokenType(t *testing.T) {
	jwtSecret := "test-secret"
	userID := "user123"

	// Generate refresh token instead of access token
	refreshToken := generateTestToken(jwtSecret, userID, "refresh", time.Hour)

	hub := NewHub()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, jwtSecret, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/ws?token=%s", refreshToken), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebSocketHandler_BlacklistedToken(t *testing.T) {
	jwtSecret := "test-secret"
	userID := "user123"

	token := generateTestToken(jwtSecret, userID, "access", time.Hour)

	// Setup Redis with blacklisted token
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	blacklistKey := fmt.Sprintf("blacklist:%s", token)
	rdb.Set(ctx, blacklistKey, "1", time.Hour)
	defer rdb.Del(ctx, blacklistKey)

	hub := NewHub()
	router := setupTestRouter(hub, rdb, jwtSecret, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/ws?token=%s", token), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token has been revoked")
}

func TestWebSocketHandler_SuccessfulUpgrade(t *testing.T) {
	jwtSecret := "test-secret"
	userID := "user123"
	token := generateTestToken(jwtSecret, userID, "access", time.Hour)

	hub := NewHub()
	go hub.Run()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, jwtSecret, nil)

	// Create test server
	server := httptest.NewServer(router)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token

	// Attempt WebSocket connection
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Verify client is registered in hub
	time.Sleep(50 * time.Millisecond)
	assert.True(t, hub.IsUserConnected(userID))
	assert.Equal(t, 1, hub.GetConnectionCount())
}

func TestWebSocketHandler_OriginValidation(t *testing.T) {
	jwtSecret := "test-secret"
	userID := "user123"
	token := generateTestToken(jwtSecret, userID, "access", time.Hour)

	allowedOrigins := []string{"https://example.com", "https://app.example.com"}

	hub := NewHub()
	go hub.Run()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	router := setupTestRouter(hub, rdb, jwtSecret, allowedOrigins)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token

	tests := []struct {
		name          string
		origin        string
		shouldSucceed bool
	}{
		{
			name:          "allowed origin",
			origin:        "https://example.com",
			shouldSucceed: true,
		},
		{
			name:          "another allowed origin",
			origin:        "https://app.example.com",
			shouldSucceed: true,
		},
		{
			name:          "disallowed origin",
			origin:        "https://malicious.com",
			shouldSucceed: false,
		},
		{
			name:          "no origin header",
			origin:        "",
			shouldSucceed: true, // No origin = non-browser client, allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer := websocket.DefaultDialer
			headers := http.Header{}
			if tt.origin != "" {
				headers.Set("Origin", tt.origin)
			}

			conn, resp, err := dialer.Dial(wsURL, headers)

			if tt.shouldSucceed {
				require.NoError(t, err)
				assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
				conn.Close()
			} else {
				assert.Error(t, err)
				assert.NotEqual(t, http.StatusSwitchingProtocols, resp.StatusCode)
			}
		})
	}
}
