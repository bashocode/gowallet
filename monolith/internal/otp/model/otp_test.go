package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOTPJSON(t *testing.T) {
	now := time.Now()
	otp := OTP{
		ID:        "1",
		UserID:    "user123",
		Code:      "123456",
		Type:      "email_verification",
		ExpiresAt: now.Add(5 * time.Minute),
		Used:      false,
		CreatedAt: now,
	}

	data, err := json.Marshal(otp)
	if err != nil {
		t.Fatalf("failed to marshal OTP: %v", err)
	}

	// Verify that Code is omitted because of `json:"-"`
	if strings.Contains(string(data), "123456") || strings.Contains(string(data), `"code"`) {
		t.Errorf("expected OTP JSON not to contain code field, got: %s", string(data))
	}

	var decoded OTP
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal OTP: %v", err)
	}

	if decoded.ID != otp.ID || decoded.UserID != otp.UserID || decoded.Type != otp.Type || decoded.Used != otp.Used {
		t.Errorf("decoded OTP does not match original: %+v vs %+v", decoded, otp)
	}
}
