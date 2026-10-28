package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIdentityAccountLifecycle(t *testing.T) {
	cfg := Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "identity.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()

	c := &apiClient{t: t, srv: srv}
	if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": "acct@example.test", "password": "OldStrongPass#2026!", "display_name": "Acct"}); st != 201 {
		t.Fatalf("register: %d %v", st, out)
	}
	st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": "acct@example.test", "password": "OldStrongPass#2026!", "platform": "web", "device_name": "test"})
	if st != 200 {
		t.Fatalf("login: %d %v", st, out)
	}
	c.token = out["access_token"].(string)

	// invalid change requests
	if st, _ := c.req("PATCH", "/api/v1/auth/password", map[string]any{"current_password": "wrong", "new_password": "NewStrongPass#2026!"}); st != 401 {
		t.Fatalf("wrong current password: %d", st)
	}
	if st, _ := c.req("PATCH", "/api/v1/auth/password", map[string]any{"current_password": "OldStrongPass#2026!", "new_password": "short"}); st != 400 {
		t.Fatalf("short new password: %d", st)
	}
	if st, out := c.req("PATCH", "/api/v1/auth/password", map[string]any{"current_password": "OldStrongPass#2026!", "new_password": "NewStrongPass#2026!"}); st != 200 || out["status"] != "password_changed" {
		t.Fatalf("change password: %d %v", st, out)
	}
	// current session remains usable
	if st, _ := c.req("GET", "/api/v1/me", nil); st != 200 {
		t.Fatalf("me after password change: %d", st)
	}
	// old password no longer works
	c2 := &apiClient{t: t, srv: srv}
	if st, _ := c2.req("POST", "/api/v1/auth/login", map[string]any{"email": "acct@example.test", "password": "OldStrongPass#2026!"}); st != 401 {
		t.Fatalf("old password login: %d", st)
	}
	// new password works
	if st, out := c2.req("POST", "/api/v1/auth/login", map[string]any{"email": "acct@example.test", "password": "NewStrongPass#2026!", "platform": "web", "device_name": "test"}); st != 200 {
		t.Fatalf("new password login: %d %v", st, out)
	}
	c2.token = out["access_token"].(string)

	// recovery
	st, out = c.req("POST", "/api/v1/auth/recover", map[string]any{"email": "acct@example.test"})
	if st != 200 || out["status"] != "recovery_token_sent_if_account_exists" {
		t.Fatalf("recover: %d %v", st, out)
	}
	challengeID, _ := out["challenge_id"].(string)
	code, _ := out["recovery_code"].(string)
	if challengeID == "" || code == "" {
		t.Fatalf("missing recovery challenge: %v", out)
	}
	// wrong code
	if st, _ := c.req("POST", "/api/v1/auth/recover/verify", map[string]any{"challenge_id": challengeID, "recovery_code": code + "x", "new_password": "RecoveredStrong#2026!"}); st != 401 {
		t.Fatalf("wrong recovery code: %d", st)
	}
	// correct code
	if st, out := c.req("POST", "/api/v1/auth/recover/verify", map[string]any{"challenge_id": challengeID, "recovery_code": code, "new_password": "RecoveredStrong#2026!"}); st != 200 || out["status"] != "password_reset" {
		t.Fatalf("verify recovery: %d %v", st, out)
	}
	// consumed challenge cannot be reused
	if st, _ := c.req("POST", "/api/v1/auth/recover/verify", map[string]any{"challenge_id": challengeID, "recovery_code": code, "new_password": "AnotherStrong#2026!"}); st != 401 {
		t.Fatalf("reused recovery challenge: %d", st)
	}
	// nonexistent email returns generic success without challenge
	st, out = c.req("POST", "/api/v1/auth/recover", map[string]any{"email": "missing@example.test"})
	if st != 200 || out["challenge_id"] != nil {
		t.Fatalf("missing account recovery: %d %v", st, out)
	}

	// login with recovered password
	st, out = c2.req("POST", "/api/v1/auth/login", map[string]any{"email": "acct@example.test", "password": "RecoveredStrong#2026!", "platform": "web", "device_name": "test"})
	if st != 200 {
		t.Fatalf("recovered password login: %d %v", st, out)
	}
	c2.token = out["access_token"].(string)
	if st, _ := c2.req("DELETE", "/api/v1/me", map[string]any{"password": "wrong"}); st != 401 {
		t.Fatalf("wrong delete password: %d", st)
	}
	if st, out := c2.req("DELETE", "/api/v1/me", map[string]any{"password": "RecoveredStrong#2026!"}); st != 200 || out["status"] != "account_deleted" {
		t.Fatalf("delete account: %d %v", st, out)
	}
	// login after deletion fails
	if st, _ := c2.req("POST", "/api/v1/auth/login", map[string]any{"email": "acct@example.test", "password": "RecoveredStrong#2026!"}); st != 401 {
		t.Fatalf("login after deletion: %d", st)
	}
	// authenticated access after deletion fails
	if st, _ := c2.req("GET", "/api/v1/me", nil); st != 401 {
		t.Fatalf("me after deletion: %d", st)
	}
}
