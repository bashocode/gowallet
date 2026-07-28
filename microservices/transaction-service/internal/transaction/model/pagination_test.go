package model

import (
	"testing"
	"time"
)

func TestEncodeDecodeCursor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	txID := "tx-uuid-12345"

	encoded := EncodeCursor(now, txID)
	if encoded == "" {
		t.Fatal("Expected non-empty cursor string")
	}

	decodedTime, decodedID, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("Unexpected error decoding cursor: %v", err)
	}

	if !decodedTime.Equal(now) {
		t.Errorf("Expected time %v, got %v", now, decodedTime)
	}

	if decodedID != txID {
		t.Errorf("Expected ID %s, got %s", txID, decodedID)
	}
}

func TestDecodeInvalidCursor(t *testing.T) {
	invalidCursors := []string{
		"invalid-base64-!!!",
		"YWJjZA==", // "abcd" without comma
		"YWJjZCxlZmc=", // "abcd,efg" where abcd is not a valid int64 timestamp
	}

	for _, c := range invalidCursors {
		_, _, err := DecodeCursor(c)
		if err == nil {
			t.Errorf("Expected error for invalid cursor %q, got nil", c)
		}
	}
}
