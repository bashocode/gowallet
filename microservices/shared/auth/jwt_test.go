package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAndValidateToken(t *testing.T) {
	userID := "user-123"
	email := "test@example.com"
	role := "user"
	duration := 15 * time.Minute

	// Test GenerateToken
	token, err := GenerateToken(userID, email, role, duration)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Test ValidateToken with valid token
	claims, err := ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.WithinDuration(t, time.Now().Add(duration), claims.ExpiresAt.Time, 2*time.Second)
}

func TestValidateToken_Invalid(t *testing.T) {
	// Test with malformed token
	claims, err := ValidateToken("invalid-token-string")
	assert.Error(t, err)
	assert.Nil(t, claims)

	// Test with expired token
	userID := "user-123"
	email := "test@example.com"
	role := "user"
	duration := -5 * time.Minute // expired 5 minutes ago

	expiredToken, err := GenerateToken(userID, email, role, duration)
	assert.NoError(t, err)
	assert.NotEmpty(t, expiredToken)

	claims, err = ValidateToken(expiredToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateToken_AlgorithmConfusion(t *testing.T) {
	// Create token with "none" algorithm
	claims := &JWTClaims{
		UserID:    "user-hacker",
		Email:     "hacker@example.com",
		Role:      "admin",
		TokenType: "access",
	}
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	noneTokenString, _ := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)

	validClaims, err := ValidateToken(noneTokenString)
	assert.Error(t, err, "none algorithm token must be rejected")
	assert.Nil(t, validClaims)
}

func TestValidateTokenWithType_Mismatch(t *testing.T) {
	token, err := GenerateTokenWithType("user-1", "test@example.com", "user", "refresh", 10*time.Minute)
	assert.NoError(t, err)

	_, err = ValidateTokenWithType(token, "access")
	assert.Error(t, err, "refresh token must not be accepted as access token")
}

func TestGetSecretKey_ProductionFailClosed(t *testing.T) {
	oldEnv := os.Getenv("APP_ENV")
	oldSecret := os.Getenv("JWT_SECRET")
	defer func() {
		os.Setenv("APP_ENV", oldEnv)
		os.Setenv("JWT_SECRET", oldSecret)
	}()

	os.Setenv("APP_ENV", "production")
	os.Setenv("JWT_SECRET", "short")

	assert.Panics(t, func() {
		getSecretKey()
	}, "must panic if secret < 32 chars in production")
}
