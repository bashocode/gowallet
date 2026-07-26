package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const ExpectedIssuer = "gowallet-auth-service"
const ExpectedAudience = "gowallet-api"

type JWTClaims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func GenerateToken(secretKey string, userID string, email string, role string, duration time.Duration) (string, error) {
	return GenerateTokenWithType(secretKey, userID, email, role, "access", duration)
}

func GenerateTokenWithType(secretKey string, userID string, email string, role string, tokenType string, duration time.Duration) (string, error) {
	if secretKey == "" {
		return "", errors.New("jwt secret key is required")
	}
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ExpectedIssuer,
			Audience:  jwt.ClaimStrings{ExpectedAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%s-%s-%d", userID, tokenType, now.UnixNano()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateToken(secretKey string, tokenString string) (*JWTClaims, error) {
	return ValidateTokenWithType(secretKey, tokenString, "")
}

func ValidateTokenWithType(secretKey string, tokenString string, expectedType string) (*JWTClaims, error) {
	if secretKey == "" {
		return nil, errors.New("jwt secret key is required")
	}
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		// Pin signing algorithm to HMAC-SHA256 only
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	if expectedType != "" && claims.TokenType != expectedType {
		return nil, fmt.Errorf("token type mismatch: expected %s, got %s", expectedType, claims.TokenType)
	}

	if claims.Issuer != ExpectedIssuer {
		return nil, fmt.Errorf("invalid token issuer: expected %s, got %s", ExpectedIssuer, claims.Issuer)
	}

	var audValid bool
	for _, aud := range claims.Audience {
		if aud == ExpectedAudience {
			audValid = true
			break
		}
	}
	if !audValid {
		return nil, fmt.Errorf("invalid token audience: expected %s", ExpectedAudience)
	}

	return claims, nil
}
