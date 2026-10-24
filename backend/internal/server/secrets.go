package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func randomSecret(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, e := rand.Read(buf); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
func sha256hex(s string) string { x := sha256.Sum256([]byte(s)); return hex.EncodeToString(x[:]) }
