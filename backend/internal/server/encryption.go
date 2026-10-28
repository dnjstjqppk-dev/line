package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// messageCipher provides AES-256-GCM encryption for message bodies at rest.
type messageCipher struct {
	gcm cipher.AEAD
}

func newMessageCipher(key string) (*messageCipher, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("encryption key must be >=32 bytes")
	}
	block, err := aes.NewCipher([]byte(key[:32]))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &messageCipher{gcm: gcm}, nil
}

func (mc *messageCipher) encrypt(plaintext string) string {
	if mc == nil || plaintext == "" {
		return plaintext
	}
	nonce := make([]byte, mc.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}
	ct := mc.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct)
}

func (mc *messageCipher) decrypt(ciphertext string) string {
	if mc == nil || ciphertext == "" {
		return ciphertext
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return ciphertext
	}
	if len(data) < mc.gcm.NonceSize() {
		return ciphertext
	}
	nonce := data[:mc.gcm.NonceSize()]
	ct := data[mc.gcm.NonceSize():]
	pt, err := mc.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return ciphertext
	}
	return string(pt)
}