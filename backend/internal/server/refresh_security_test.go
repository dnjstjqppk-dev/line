package server

import (
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRefreshExpiryRevocationAndSingleUse(t *testing.T) {
	app, err := New(Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "refresh.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	c := &apiClient{t: t, srv: srv}
	if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": "refresh@example.test", "password": "RefreshStrong#2026!", "display_name": "Refresh"}); st != 201 {
		t.Fatalf("register: %d %v", st, out)
	}
	login := func() map[string]any {
		st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": "refresh@example.test", "password": "RefreshStrong#2026!"})
		if st != 200 {
			t.Fatalf("login: %d %v", st, out)
		}
		return out
	}
	refresh := func(token string) int {
		st, _ := c.req("POST", "/api/v1/auth/refresh", map[string]any{"refresh_token": token})
		return st
	}
	expired := login()
	if _, err := app.db.Exec("UPDATE identity.sessions SET expires_at=$1 WHERE id=$2", "2000-01-01T00:00:00Z", expired["session_id"]); err != nil {
		t.Fatal(err)
	}
	if st := refresh(expired["refresh_token"].(string)); st != 401 {
		t.Fatalf("expired refresh: %d", st)
	}
	revoked := login()
	if _, err := app.db.Exec("UPDATE identity.sessions SET revoked_at=now() WHERE id=$1", revoked["session_id"]); err != nil {
		t.Fatal(err)
	}
	if st := refresh(revoked["refresh_token"].(string)); st != 401 {
		t.Fatalf("revoked refresh: %d", st)
	}
	active := login()
	token := active["refresh_token"].(string)
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			st := refresh(token)
			if st == 200 {
				winners.Add(1)
			} else if st != 401 {
				t.Errorf("refresh status: %d", st)
			}
		}()
	}
	close(start)
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("successful rotations: %d", winners.Load())
	}
	if st := refresh(token); st != 401 {
		t.Fatalf("replay: %d", st)
	}
}
