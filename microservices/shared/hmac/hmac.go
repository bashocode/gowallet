package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Sign computes the HMAC-SHA256 signature of payload using secret.
// The signature is returned as a lowercase hex string.
func Sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether the provided signature matches the HMAC-SHA256 of
// payload using secret. It uses hmac.Equal to avoid timing attacks.
func Verify(payload []byte, secret string, signature string) error {
	expected, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actual := mac.Sum(nil)

	if !hmac.Equal(expected, actual) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
