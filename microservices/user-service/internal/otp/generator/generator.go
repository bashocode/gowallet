package generator

import (
	"crypto/rand"
)

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
		if val < 250 {
			otp[i] = charset[val%10]
			i++
		}
	}
	return string(otp), nil
}
