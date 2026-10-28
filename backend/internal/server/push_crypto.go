package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

func sealPushToken(keyText, deviceID, provider, token string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyText)
	if err != nil || len(key) != 32 {
		return nil, errors.New("PUSH_TOKEN_ENCRYPTION_KEY must encode 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return append([]byte("v1:"), gcm.Seal(nonce, nonce, []byte(token), []byte(deviceID+":"+provider))...), nil
}
