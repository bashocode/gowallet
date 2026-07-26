package websocket

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bashocode/gowallet/microservices/shared/auth"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin validates the origin of the WebSocket request
	// In production, this should be restricted to allowed origins
	CheckOrigin: func(r *http.Request) bool {
		return true // Will be configured with allowed origins
	},
}

// WebSocketHandler creates a Gin handler for WebSocket connections
func WebSocketHandler(hub *Hub, rdb *redis.Client, jwtSecret string, allowedOrigins []string) gin.HandlerFunc {
	// Configure upgrader with allowed origins
	if len(allowedOrigins) > 0 {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Allow requests without origin (non-browser clients)
			}

			// Check if origin is in allowed list
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		}
	}

	return func(c *gin.Context) {
		// 1. Extract JWT from query parameter
		// WebSocket handshakes cannot set custom headers in browsers,
		// so we use a query parameter instead of Authorization header
		tokenString := c.Query("token")
		if tokenString == "" {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"MISSING_TOKEN",
				"WebSocket token is required. Provide it via ?token=<jwt>",
			))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authentication token",
			})
			return
		}

		// 2. Validate JWT and extract claims
		claims, err := auth.ValidateTokenWithType(jwtSecret, tokenString, "access")
		if err != nil {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"INVALID_TOKEN",
				fmt.Sprintf("Token validation failed: %v", err),
			))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}

		// 3. Check if token is blacklisted in Redis
		ctx := context.Background()
		blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
		exists, err := rdb.Exists(ctx, blacklistKey).Result()
		if err == nil && exists > 0 {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"TOKEN_REVOKED",
				"Token has been revoked",
			))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token has been revoked",
			})
			return
		}

		// 4. Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// Upgrade already writes an error response if it fails
			return
		}

		// 5. Create client and register with hub
		client := &Client{
			UserID: claims.UserID,
			Conn:   conn,
			Send:   make(chan []byte, 64), // Buffered channel for 64 pending messages
			hub:    hub,
		}

		// Register client with hub
		hub.register <- client

		// 6. Start read and write goroutines
		// These will run until the connection closes
		go client.writePump()
		go client.readPump()
	}
}

// ParseAllowedOrigins parses a comma-separated string of allowed origins
func ParseAllowedOrigins(origins string) []string {
	if origins == "" {
		return []string{}
	}

	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
