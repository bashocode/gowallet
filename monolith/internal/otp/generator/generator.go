package generator

import (
	"crypto/rand"
)

// GenerateOTP generates a cryptographically secure random numeric OTP of the specified length.
// It avoids using rand.Int by reading from crypto/rand.Reader directly and using rejection
// sampling to eliminate modulo bias.
func GenerateOTP(length int) (string, error) {
	const charset = "0123456789"
	otp := make([]byte, length)
	num := make([]byte, 1)
	for i := 0; i < length; {
		_, err := rand.Read(num)
		if err != nil {
			return "", err
		}
		val := num[0]
		// 256 % 10 = 6. 256 - 6 = 250.
		// To avoid modulo bias, discard values >= 250.
		if val < 250 {
			otp[i] = charset[val%10]
			i++
		}
	}
	return string(otp), nil
}
