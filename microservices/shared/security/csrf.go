package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// GenerateCSRFToken generates a cryptographically random hex string.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetCSRFCookie sets the CSRF token as a cookie that the frontend can read
// (NOT HttpOnly — the frontend must read it to send it as a header).
func SetCSRFCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode) // Lax so cross-site GET navigation works
	c.SetCookie(
		csrfCookieName,
		token,
		3600*24, // 24 hours
		"/",
		"",
		true,  // secure (HTTPS only)
		false, // NOT httpOnly — frontend JS must read this cookie
	)
}

// GetCSRFCookie reads the CSRF token from the cookie.
func GetCSRFCookie(c *gin.Context) (string, error) {
	return c.Cookie(csrfCookieName)
}

// RotateCSRFToken generates a new CSRF token and updates the cookie.
// Useful during login, logout, or sensitive privilege elevation events.
func RotateCSRFToken(c *gin.Context) (string, error) {
	token, err := GenerateCSRFToken()
	if err != nil {
		return "", err
	}
	SetCSRFCookie(c, token)
	return token, nil
}

// ValidateCSRFToken checks that the X-CSRF-Token header matches the csrf_token cookie.
func ValidateCSRFToken(c *gin.Context) bool {
	cookieToken, err := c.Cookie(csrfCookieName)
	if err != nil {
		return false
	}

	headerToken := c.GetHeader(csrfHeaderName)
	if headerToken == "" {
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}
