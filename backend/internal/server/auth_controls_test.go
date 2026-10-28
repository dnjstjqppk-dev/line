package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"
	"time"
)

func TestAuthLimiterBounds(t *testing.T) {
	var l authLimiter
	now := time.Now()
	for i := 0; i < 60; i++ {
		if !l.allow("127.0.0.1:1234", "/api/v1/auth/login", now) {
			t.Fatal("early limit")
		}
	}
	if l.allow("127.0.0.1:5678", "/api/v1/auth/login", now) {
		t.Fatal("port bypass")
	}
	if !l.allow("127.0.0.1:5678", "/api/v1/auth/login", now.Add(time.Minute)) {
		t.Fatal("window failed to expire")
	}
}

func TestPushTokenStorageProtection(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	encoded := base64.StdEncoding.EncodeToString(key)
	sealed, err := sealPushToken(encoded, "device", "fcm", "sensitive-token")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("sensitive-token")) {
		t.Fatal("plaintext token")
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	payload := sealed[3:]
	nonce := payload[:gcm.NonceSize()]
	got, err := gcm.Open(nil, nonce, payload[gcm.NonceSize():], []byte("device:fcm"))
	if err != nil || string(got) != "sensitive-token" {
		t.Fatalf("decrypt: %v", err)
	}
	if _, err := gcm.Open(nil, nonce, payload[gcm.NonceSize():], []byte("other:fcm")); err == nil {
		t.Fatal("cross-device ciphertext accepted")
	}
	if _, err := sealPushToken("", "device", "fcm", "secret"); err == nil {
		t.Fatal("missing key accepted")
	}
}
