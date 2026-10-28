package server

import (
	"encoding/base64"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestResumableUpload(t *testing.T) {
	cfg := Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "upload.db")), Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()

	c := &apiClient{t: t, srv: srv}
	if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": "up@example.test", "password": "UploadStrong#2026!", "display_name": "Up"}); st != 201 {
		t.Fatalf("register: %d %v", st, out)
	}
	st, out := c.req("POST", "/api/v1/auth/login", map[string]any{"email": "up@example.test", "password": "UploadStrong#2026!"})
	if st != 200 {
		t.Fatalf("login: %d %v", st, out)
	}
	c.token = out["access_token"].(string)

	data := []byte("hello world chunked upload test")
	encoded := base64.StdEncoding.EncodeToString(data)

	// Init upload
	st, out = c.req("POST", "/api/v1/uploads/init", map[string]any{
		"original_name": "test.txt",
		"mime_type":     "text/plain",
		"total_size":    len(data),
		"chunk_size":    1024,
	})
	if st != 201 {
		t.Fatalf("init upload: %d %v", st, out)
	}
	uploadID := out["upload_id"].(string)

	// Upload chunk
	st, out = c.req("PUT", "/api/v1/uploads/"+uploadID+"/chunks/0", map[string]any{
		"content_base64": encoded,
	})
	if st != 200 {
		t.Fatalf("upload chunk: %d %v", st, out)
	}

	// Complete upload
	st, out = c.req("POST", "/api/v1/uploads/"+uploadID+"/complete", map[string]any{})
	if st != 201 {
		t.Fatalf("complete upload: %d %v", st, out)
	}
	if out["status"] != "ready" {
		t.Fatalf("expected ready, got %v", out["status"])
	}
}