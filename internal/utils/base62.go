
package utils

import (
    "crypto/rand"
    "math/big"
)

const base62Charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateRandomSuffix returns a 7-character random base62 string.
func GenerateRandomSuffix() (string, error) {
    var result string
    for i := 0; i < 7; i++ {
        n, err := rand.Int(rand.Reader, big.NewInt(int64(len(base62Charset))))
        if err != nil {
            return "", err
        }
        result += string(base62Charset[n.Int64()])
    }
    return result, nil
}
