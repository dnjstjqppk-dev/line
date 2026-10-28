package server

import (
	"github.com/gorilla/websocket"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionAndQRLifecycle(t *testing.T) {
	app, err := New(Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "sessions.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	var seeded int
	if err := app.db.QueryRow("SELECT count(*) FROM identity.users").Scan(&seeded); err != nil || seeded != 0 {
		t.Fatalf("unexpected seeded accounts: %d %v", seeded, err)
	}
	c := &apiClient{t: t, srv: srv}
	if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": "session@example.test", "password": "SessionStrong#2026!", "display_name": "Session"}); st != 201 {
		t.Fatalf("register: %d %v", st, out)
	}
	login := func() (*apiClient, map[string]any) {
		st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": "session@example.test", "password": "SessionStrong#2026!"})
		if st != 200 {
			t.Fatalf("login: %d %v", st, out)
		}
		return &apiClient{t: t, srv: srv, token: out["access_token"].(string), userID: out["user_id"].(string), devID: out["device_id"].(string)}, out
	}
	first, session := login()
	second, _ := login()
	ws := first.connectWS()
	defer ws.Close()
	_, ticket := first.req("POST", "/api/v1/ws-ticket", nil)
	if st, out := first.req("POST", "/api/v1/auth/logout", map[string]any{"refresh_token": session["refresh_token"]}); st != 200 {
		t.Fatalf("logout: %d %v", st, out)
	}
	if st, _ := first.req("GET", "/api/v1/me", nil); st != 401 {
		t.Fatalf("revoked access: %d", st)
	}
	if st, _ := second.req("GET", "/api/v1/me", nil); st != 200 {
		t.Fatalf("independent session: %d", st)
	}
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/ws?ticket="+ticket["ticket"].(string), nil)
	if conn != nil {
		conn.Close()
	}
	if err == nil || resp == nil || resp.StatusCode != 401 {
		t.Fatal("revoked ticket accepted")
	}
	app.hub.Notify([]string{first.userID}, []byte(`{"type":"private.after_logout"}`))
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, data, err := ws.ReadMessage(); err == nil {
		t.Fatalf("revoked socket received: %s", data)
	}
	if st, _ := second.req("DELETE", "/api/v1/devices/"+second.devID, nil); st != 200 {
		t.Fatalf("revoke device: %d", st)
	}
	if st, _ := second.req("GET", "/api/v1/me", nil); st != 401 {
		t.Fatalf("revoked device access: %d", st)
	}
	active, _ := login()
	challenge := func() map[string]any {
		st, out := c.req("POST", "/api/v1/auth/qr/challenge", map[string]any{})
		if st != 201 {
			t.Fatalf("challenge: %d %v", st, out)
		}
		return map[string]any{"challenge_id": out["challenge_id"], "qr_token": out["qr_token"]}
	}
	expired := challenge()
	if _, err := app.db.Exec("UPDATE identity.verification_challenges SET expires_at=$1 WHERE id=$2", "2000-01-01T00:00:00Z", expired["challenge_id"]); err != nil {
		t.Fatal(err)
	}
	if st, _ := active.req("POST", "/api/v1/auth/qr/approve", expired); st != 409 {
		t.Fatalf("expired approval: %d", st)
	}
	if st, _ := c.req("POST", "/api/v1/auth/qr/poll", expired); st != 401 {
		t.Fatalf("expired polling: %d", st)
	}
	qr := challenge()
	parallel := func(route string, client *apiClient) {
		var wins atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				st, _ := client.req("POST", route, qr)
				if st == 200 {
					wins.Add(1)
				} else if st != 409 {
					t.Errorf("QR race status: %d", st)
				}
			}()
		}
		close(start)
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("QR successes: %d", wins.Load())
		}
	}
	parallel("/api/v1/auth/qr/approve", active)
	parallel("/api/v1/auth/qr/poll", c)
	if st, _ := c.req("POST", "/api/v1/auth/qr/poll", qr); st != 409 {
		t.Fatalf("QR replay: %d", st)
	}
}
