package utils

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestSafeString(t *testing.T) {
	val, ok := SafeString("test")
	assert.True(t, ok)
	assert.Equal(t, "test", val)

	val, ok = SafeString(123)
	assert.False(t, ok)
	assert.Equal(t, "", val)

	val, ok = SafeString(nil)
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestSafeBool(t *testing.T) {
	val, ok := SafeBool(true)
	assert.True(t, ok)
	assert.True(t, val)

	val, ok = SafeBool("not a bool")
	assert.False(t, ok)
	assert.False(t, val)

	val, ok = SafeBool(nil)
	assert.False(t, ok)
	assert.False(t, val)
}

func TestSafeDecimal(t *testing.T) {
	dec := decimal.NewFromInt(42)
	val, ok := SafeDecimal(dec)
	assert.True(t, ok)
	assert.True(t, val.Equal(dec))

	val, ok = SafeDecimal("not a decimal")
	assert.False(t, ok)
	assert.True(t, val.IsZero())

	val, ok = SafeDecimal(nil)
	assert.False(t, ok)
	assert.True(t, val.IsZero())
}
