package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAndValidateToken(t *testing.T) {
	userID := "user-123"
	email := "test@example.com"
	duration := 15 * time.Minute

	// Test GenerateToken
	token, err := GenerateToken(userID, email, duration)
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
	duration := -5 * time.Minute // expired 5 minutes ago

	expiredToken, err := GenerateToken(userID, email, duration)
	assert.NoError(t, err)
	assert.NotEmpty(t, expiredToken)

	claims, err = ValidateToken(expiredToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}
