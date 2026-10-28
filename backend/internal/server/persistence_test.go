package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestKeepPersistenceAndAuthorization(t *testing.T) {
	cfg := Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "keep.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()
	login := func(email, password string) *apiClient {
		c := &apiClient{t: t, srv: srv}
		if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": password, "display_name": email}); st != 201 {
			t.Fatalf("register: %d %v", st, out)
		}
		status, result := c.req("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password, "platform": "web", "device_name": "test"})
		if status != 200 {
			t.Fatalf("login: %d %v", status, result)
		}
		c.token = result["access_token"].(string)
		return c
	}
	owner := login("keep-owner@example.test", "OwnerStrongPass#2026!")
	outsider := login("keep-other@example.test", "OtherStrongPass#2026!")
	status, conv := owner.req("POST", "/api/v1/conversations", map[string]any{"kind": "self"})
	if status != 201 {
		t.Fatalf("conversation: %d %v", status, conv)
	}
	cid := conv["id"].(string)
	body := map[string]any{"client_id": "persistent-message-1", "kind": "text", "body": "persisted content"}
	status, sent := owner.req("POST", "/api/v1/conversations/"+cid+"/messages", body)
	if status != 201 {
		t.Fatalf("send: %d %v", status, sent)
	}
	mid := sent["message"].(map[string]any)["id"].(string)
	save := map[string]any{"message_id": mid, "body": "forged content", "kind": "image"}
	if st, _ := outsider.req("POST", "/api/v1/saved-messages", save); st != 404 {
		t.Fatalf("outsider save: %d", st)
	}
	ws := owner.connectWS()
	defer ws.Close()
	for i := 0; i < 2; i++ {
		if st, out := owner.req("POST", "/api/v1/saved-messages", save); st != 201 {
			t.Fatalf("save: %d %v", st, out)
		}
		readWSEvent(t, ws, "saved-messages.updated")
	}
	ws.Close()
	outsider.req("DELETE", "/api/v1/saved-messages/"+mid, nil)
	srv.Close()
	app.Close()
	app, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv = httptest.NewServer(app.Handler())
	owner.srv = srv
	outsider.srv = srv
	st, listed := owner.req("GET", "/api/v1/saved-messages", nil)
	if st != 200 {
		t.Fatalf("list: %d %v", st, listed)
	}
	items := listed["saved_messages"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one persistent save: %v", items)
	}
	item := items[0].(map[string]any)
	if item["id"] != mid || item["body"] != "persisted content" || item["kind"] != "text" {
		t.Fatalf("canonical item: %v", item)
	}
	st, duplicate := owner.req("POST", "/api/v1/conversations/"+cid+"/messages", body)
	if st != 200 || duplicate["duplicate"] != true {
		t.Fatalf("retry after restart: %d %v", st, duplicate)
	}
	_, otherList := outsider.req("GET", "/api/v1/saved-messages", nil)
	if len(otherList["saved_messages"].([]any)) != 0 {
		t.Fatal("private Keep leaked")
	}
	if _, err := app.db.Exec("UPDATE chat.conversation_members SET left_at=now() WHERE conversation_id=$1", cid); err != nil {
		t.Fatal(err)
	}
	_, leftList := owner.req("GET", "/api/v1/saved-messages", nil)
	if len(leftList["saved_messages"].([]any)) != 0 {
		t.Fatal("left member retained content access")
	}
	if st, _ := owner.req("POST", "/api/v1/saved-messages", save); st != 404 {
		t.Fatalf("left member save: %d", st)
	}
	if st, _ := owner.req("DELETE", "/api/v1/saved-messages/"+mid, nil); st != 200 {
		t.Fatalf("delete: %d", st)
	}
}
