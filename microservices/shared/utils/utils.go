package utils

import "github.com/shopspring/decimal"

// SafeString converts an interface to a string safely without panicking.
// Returns the string value and true if assertion succeeded, or empty string and false otherwise.
func SafeString(val interface{}) (string, bool) {
	if val == nil {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// SafeBool converts an interface to a bool safely without panicking.
// Returns the bool value and true if assertion succeeded, or false and false otherwise.
func SafeBool(val interface{}) (bool, bool) {
	if val == nil {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// SafeDecimal converts an interface to a decimal.Decimal safely without panicking.
// Returns the decimal value and true if assertion succeeded, or Zero and false otherwise.
func SafeDecimal(val interface{}) (decimal.Decimal, bool) {
	if val == nil {
		return decimal.Zero, false
	}
	d, ok := val.(decimal.Decimal)
	return d, ok
}
