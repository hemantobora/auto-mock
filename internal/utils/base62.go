package utils

import (
	"crypto/rand"
	"math/big"
)

const base36Charset = "0123456789abcdefghijklmnopqrstuvwxyz"

// GenerateRandomSuffix returns a 7-character random base36 string.
func GenerateRandomSuffix() (string, error) {
	var result string
	for range 8 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(base36Charset))))
		if err != nil {
			return "", err
		}
		result += string(base36Charset[n.Int64()])
	}
	return result, nil
}
