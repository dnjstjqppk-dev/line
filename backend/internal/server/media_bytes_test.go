package server

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRealMediaBytesRestartAndDeletion(t *testing.T) {
	cfg := Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "media.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()
	login := func(email string) *apiClient {
		c := &apiClient{t: t, srv: srv}
		if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": "MediaStrong#2026!", "display_name": email}); st != 201 {
			t.Fatalf("register: %d %v", st, out)
		}
		st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": "MediaStrong#2026!"})
		if st != 200 {
			t.Fatalf("login: %d %v", st, out)
		}
		c.token = out["access_token"].(string)
		c.userID = out["user_id"].(string)
		return c
	}
	alice, bob, eve := login("bytes-a@example.test"), login("bytes-b@example.test"), login("bytes-e@example.test")
	data := []byte{0, 1, 2, 3, 255, 128, 10, 13}
	body := map[string]any{"original_name": "../../sample.bin", "mime_type": "application/octet-stream", "size_bytes": len(data), "content_base64": base64.StdEncoding.EncodeToString(data), "checksum_sha256": "wrong"}
	if st, _ := alice.req("POST", "/api/v1/attachments", body); st != 400 {
		t.Fatalf("checksum mismatch: %d", st)
	}
	body["checksum_sha256"] = sha256hex(string(data))
	st, att := alice.req("POST", "/api/v1/attachments", body)
	if st != 201 {
		t.Fatalf("upload: %d %v", st, att)
	}
	id := att["id"].(string)
	download := func(c *apiClient, want int) {
		req, err := http.NewRequest("GET", srv.URL+"/api/v1/attachments/"+id+"/content", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("download status: %d want %d", resp.StatusCode, want)
		}
		if want == 200 && !bytes.Equal(got, data) {
			t.Fatalf("bytes changed: %v", got)
		}
	}
	download(alice, 200)
	download(bob, 404)
	download(eve, 404)
	st, conv := alice.req("POST", "/api/v1/conversations", map[string]any{"kind": "group", "title": "Bytes", "member_ids": []string{bob.userID}})
	if st != 201 {
		t.Fatalf("conversation: %d %v", st, conv)
	}
	message := map[string]any{"client_id": "file-bytes-restart", "kind": "file", "body": "sample.bin", "metadata": map[string]any{"attachment_id": id}}
	route := "/api/v1/conversations/" + conv["id"].(string) + "/messages"
	if st, _ := bob.req("POST", route, message); st != 403 {
		t.Fatalf("foreign attachment association: %d", st)
	}
	if st, out := alice.req("POST", route, message); st != 201 {
		t.Fatalf("send: %d %v", st, out)
	}
	download(bob, 200)
	download(eve, 404)
	if st, _ := alice.req("POST", route, message); st != 200 {
		t.Fatalf("idempotent file retry: %d", st)
	}
	srv.Close()
	app.Close()
	app, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv = httptest.NewServer(app.Handler())
	alice.srv = srv
	bob.srv = srv
	eve.srv = srv
	download(bob, 200)
	if st, _ := bob.req("DELETE", "/api/v1/attachments/"+id, nil); st != 404 {
		t.Fatalf("nonowner delete: %d", st)
	}
	if st, _ := alice.req("DELETE", "/api/v1/attachments/"+id, nil); st != 200 {
		t.Fatalf("delete: %d", st)
	}
	download(alice, 404)
	download(bob, 404)
	var count int
	if err := app.db.QueryRow("SELECT count(*) FROM chat.attachment_blobs WHERE attachment_id=$1", id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("bytes retained: %d %v", count, err)
	}
	body["content_base64"] = base64.StdEncoding.EncodeToString(make([]byte, 512*1024+1))
	body["size_bytes"] = 512*1024 + 1
	body["checksum_sha256"] = ""
	if st, _ := alice.req("POST", "/api/v1/attachments", body); st != 400 {
		t.Fatalf("oversize: %d", st)
	}
}
