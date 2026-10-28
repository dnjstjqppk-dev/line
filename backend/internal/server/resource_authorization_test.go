package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestMediaDeviceAndCallAuthorization(t *testing.T) {
	app, err := New(Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "resources.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway", PushTokenKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	login := func(email string) *apiClient {
		c := &apiClient{t: t, srv: srv}
		if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": "ResourceStrong#2026!", "display_name": email}); st != 201 {
			t.Fatalf("register: %d %v", st, out)
		}
		st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": "ResourceStrong#2026!"})
		if st != 200 {
			t.Fatalf("login: %d %v", st, out)
		}
		c.token = out["access_token"].(string)
		c.userID = out["user_id"].(string)
		c.devID = out["device_id"].(string)
		return c
	}
	alice, bob, eve := login("media-alice@example.test"), login("media-bob@example.test"), login("media-eve@example.test")
	st, conv := alice.req("POST", "/api/v1/conversations", map[string]any{"kind": "group", "title": "Private", "member_ids": []string{bob.userID}})
	if st != 201 {
		t.Fatalf("conversation: %d %v", st, conv)
	}
	cid := conv["id"].(string)
	st, msg := alice.req("POST", "/api/v1/conversations/"+cid+"/messages", map[string]any{"client_id": "private-file-message", "kind": "text", "body": "File"})
	if st != 201 {
		t.Fatalf("message: %d %v", st, msg)
	}
	mid := msg["message"].(map[string]any)["id"].(string)
	metadata := map[string]any{"message_id": mid, "original_name": "test.txt", "mime_type": "text/plain", "size_bytes": 4, "content_base64": "RmlsZQ=="}
	for _, c := range []*apiClient{bob, eve} {
		if st, _ := c.req("POST", "/api/v1/attachments", metadata); st != 403 {
			t.Fatalf("foreign message attachment: %d", st)
		}
	}
	st, attachment := alice.req("POST", "/api/v1/attachments", metadata)
	if st != 201 {
		t.Fatalf("attachment: %d %v", st, attachment)
	}
	path := "/api/v1/attachments/" + attachment["id"].(string)
	if st, _ := bob.req("GET", path, nil); st != 200 {
		t.Fatalf("member attachment: %d", st)
	}
	if st, _ := eve.req("GET", path, nil); st != 403 {
		t.Fatalf("outsider attachment: %d", st)
	}
	for _, route := range []string{"/api/v1/presence/heartbeat", "/api/v1/push-tokens"} {
		body := map[string]any{"device_id": alice.devID}
		if route == "/api/v1/push-tokens" {
			body["provider"] = "fcm"
			body["token"] = "local-test-token"
		}
		if st, _ := eve.req("POST", route, body); st != 403 {
			t.Fatalf("foreign device %s: %d", route, st)
		}
		if st, _ := alice.req("POST", route, body); st != 200 && st != 201 {
			t.Fatalf("own device %s: %d", route, st)
		}
	}
	st, call := alice.req("POST", "/api/v1/calls", map[string]any{"conversation_id": cid, "kind": "voice"})
	if st != 201 {
		t.Fatalf("call: %d %v", st, call)
	}
	qualityPath := "/api/v1/calls/" + call["id"].(string) + "/quality"
	quality := map[string]any{"rtt_ms": 10, "jitter_ms": 1, "packet_loss_ratio": 0}
	if st, _ := eve.req("POST", qualityPath, quality); st != 403 {
		t.Fatalf("outsider quality: %d", st)
	}
	if st, _ := bob.req("POST", qualityPath, quality); st != 201 {
		t.Fatalf("member quality: %d", st)
	}
	if _, err := app.db.Exec("UPDATE chat.conversation_members SET left_at=now() WHERE conversation_id=$1 AND user_id=$2", cid, bob.userID); err != nil {
		t.Fatal(err)
	}
	if st, _ := bob.req("GET", path, nil); st != 403 {
		t.Fatalf("former member attachment: %d", st)
	}
	if _, err := app.db.Exec("UPDATE identity.devices SET revoked_at=now() WHERE id=$1", alice.devID); err != nil {
		t.Fatal(err)
	}
	if st, _ := alice.req("POST", "/api/v1/push-tokens", map[string]any{"device_id": alice.devID, "provider": "fcm", "token": "revoked-token"}); st != 401 {
		t.Fatalf("revoked device push: %d", st)
	}
}
