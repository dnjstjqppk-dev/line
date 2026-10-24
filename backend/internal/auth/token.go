package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Claims struct {
	Subject   string `json:"sub"`
	Role      string `json:"role"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func macBytes(secret []byte, data string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func Sign(subject, role string, secret []byte, ttl time.Duration) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("JWT_SECRET must be 32+ bytes")
	}
	head := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().UTC()
	p, e := json.Marshal(Claims{Subject: subject, Role: role, Issuer: "global-messenger", Audience: "messenger-api", IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix()})
	if e != nil {
		return "", e
	}
	s := head + "." + b64(p)
	return s + "." + b64(macBytes(secret, s)), nil
}
func Verify(token string, secret []byte) (Claims, error) {
	var c Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, errors.New("malformed token")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	h, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return c, e
	}
	if json.Unmarshal(h, &header) != nil || header.Alg != "HS256" || header.Typ != "JWT" {
		return c, errors.New("invalid header")
	}
	got, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil {
		return c, e
	}
	if !hmac.Equal(got, macBytes(secret, parts[0]+"."+parts[1])) {
		return c, errors.New("invalid signature")
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return c, e
	}
	if json.Unmarshal(raw, &c) != nil || c.Issuer != "global-messenger" || c.Audience != "messenger-api" || c.Subject == "" || c.ExpiresAt <= time.Now().Unix() || c.IssuedAt > time.Now().Add(time.Minute).Unix() {
		return Claims{}, errors.New("invalid claims")
	}
	return c, nil
}

// Used to reject noncanonical UUID-like IDs passed to route params.
func ValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func ParsePositiveInt(s string, fallback, max int) int {
	v, e := strconv.Atoi(s)
	if e != nil || v < 1 {
		return fallback
	}
	if v > max {
		return max
	}
	return v
}
