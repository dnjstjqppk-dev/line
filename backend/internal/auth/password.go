package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PBKDF2-SHA256 as a dependency-free development baseline.
// Production hardening: switch to memory-hard Argon2id with audited parameters.
const rounds = 310000

func derive(password string, salt []byte, iterations int) []byte {
	mac := hmac.New(sha256.New, []byte(password))
	block := append(append([]byte{}, salt...), 0, 0, 0, 1)
	mac.Write(block)
	u := mac.Sum(nil)
	result := append([]byte{}, u...)
	for i := 1; i < iterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}
func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password must be 12-1024 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := derive(password, salt, rounds)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", rounds, base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(key)), nil
}
func CheckPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	n, e := strconv.Atoi(parts[1])
	if e != nil || n < 100000 || n > 1000000 {
		return false
	}
	salt, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil || len(salt) != 16 {
		return false
	}
	expected, e := base64.RawURLEncoding.DecodeString(parts[3])
	if e != nil || len(expected) != 32 {
		return false
	}
	got := derive(password, salt, n)
	return subtle.ConstantTimeCompare(got, expected) == 1
}
