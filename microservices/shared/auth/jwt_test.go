package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const testSecret = "test-secret-key-must-be-32bytes-long!"

func TestGenerateAndValidateToken(t *testing.T) {
	userID := "user-123"
	email := "test@example.com"
	role := "user"
	duration := 15 * time.Minute

	// Test GenerateToken
	token, err := GenerateToken(testSecret, userID, email, role, duration)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Test ValidateToken with valid token
	claims, err := ValidateToken(testSecret, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.WithinDuration(t, time.Now().Add(duration), claims.ExpiresAt.Time, 2*time.Second)
}

func TestValidateToken_Invalid(t *testing.T) {
	// Test with malformed token
	claims, err := ValidateToken(testSecret, "invalid-token-string")
	assert.Error(t, err)
	assert.Nil(t, claims)

	// Test with expired token
	userID := "user-123"
	email := "test@example.com"
	role := "user"
	duration := -5 * time.Minute // expired 5 minutes ago

	expiredToken, err := GenerateToken(testSecret, userID, email, role, duration)
	assert.NoError(t, err)
	assert.NotEmpty(t, expiredToken)

	claims, err = ValidateToken(testSecret, expiredToken)
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
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   ExpectedIssuer,
			Audience: jwt.ClaimStrings{ExpectedAudience},
		},
	}
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	noneTokenString, _ := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)

	validClaims, err := ValidateToken(testSecret, noneTokenString)
	assert.Error(t, err, "none algorithm token must be rejected")
	assert.Nil(t, validClaims)
}

func TestValidateTokenWithType_Mismatch(t *testing.T) {
	token, err := GenerateTokenWithType(testSecret, "user-1", "test@example.com", "user", "refresh", 10*time.Minute)
	assert.NoError(t, err)

	_, err = ValidateTokenWithType(testSecret, token, "access")
	assert.Error(t, err, "refresh token must not be accepted as access token")
}

func TestValidateToken_SecretKeyRequired(t *testing.T) {
	_, err := GenerateToken("", "user-1", "test@example.com", "user", 10*time.Minute)
	assert.Error(t, err, "secret key must be required")

	_, err = ValidateToken("", "token")
	assert.Error(t, err, "secret key must be required")
}
