package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h, e := HashPassword("a-secure-password-123!")
	if e != nil {
		t.Fatal(e)
	}
	if !CheckPassword("a-secure-password-123!", h) {
		t.Fatal("failed verification")
	}
	if CheckPassword("wrong password", h) {
		t.Fatal("wrong password accepted")
	}
}
func TestJWT(t *testing.T) {
	secret := []byte(strings.Repeat("x", 40))
	s, e := Sign("user-1", "user", secret, time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	c, e := Verify(s, secret)
	if e != nil || c.Subject != "user-1" {
		t.Fatalf("claims invalid: %+v %v", c, e)
	}
	_, e = Verify(s+"x", secret)
	if e == nil {
		t.Fatal("tampered token verified")
	}
}
