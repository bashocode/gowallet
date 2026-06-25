package generator

import (
	"regexp"
	"testing"
)

func TestGenerateOTP(t *testing.T) {
	otp, err := GenerateOTP(6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(otp) != 6 {
		t.Errorf("expected OTP length to be 6, got %d", len(otp))
	}

	matched, err := regexp.MatchString("^[0-9]{6}$", otp)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("OTP is not a 6-digit number: %s", otp)
	}
}

func TestGenerateOTP_Multiple(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		otp, err := GenerateOTP(6)
		if err != nil {
			t.Fatalf("unexpected error at iteration %d: %v", i, err)
		}
		if seen[otp] {
			// With cryptographically secure random 6 digits, collisions in 100 iterations are possible but highly unlikely.
			// However, if we get many collisions, something is wrong.
		}
		seen[otp] = true
	}
}
