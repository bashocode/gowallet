package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserPaginationParamsOffset(t *testing.T) {
	p := PaginationParams{Page: 2, Limit: 20}
	assert.Equal(t, 20, p.Offset())
}

func TestUserJSON(t *testing.T) {
	user := User{
		ID:           "u1",
		FullName:     "Test User",
		Email:        "test@user.com",
		PasswordHash: "secret-hash",
	}

	data, err := json.Marshal(user)
	assert.NoError(t, err)

	// Verify that PasswordHash is omitted because of `json:"-"`
	if strings.Contains(string(data), "secret-hash") {
		t.Errorf("expected User JSON to not contain password hash, got: %s", string(data))
	}

	var decoded User
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, decoded.ID)
	assert.Equal(t, user.FullName, decoded.FullName)
}
