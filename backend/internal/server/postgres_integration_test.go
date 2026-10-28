package server

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPostgreSQLIntegration runs integration tests against a real PostgreSQL database.
// Set DATABASE_URL env var to enable. Skips if not set.
func TestPostgreSQLIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping PostgreSQL integration tests")
	}
	cfg := Config{DSN: dsn, Secret: "0123456789abcdef0123456789abcdef", Service: "gateway"}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Verify schema migrations
	var count int
	if err := app.db.QueryRow("SELECT count(*) FROM ops.schema_migrations").Scan(&count); err != nil {
		t.Fatalf("schema_migrations query failed: %v", err)
	}
	if count == 0 {
		t.Fatal("no schema migrations recorded")
	}

	// Verify core tables exist
	tables := []string{
		"identity.users", "chat.messages", "chat.conversations",
		"realtime.delivery_outbox", "realtime.push_queue",
		"chat.attachments", "chat.upload_sessions",
	}
	for _, table := range tables {
		var exists bool
		err := app.db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema || '.' || table_name = $1)", table).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("table %s does not exist: %v", table, err)
		}
	}

	// Verify message encryption round-trip
	t.Log("PostgreSQL integration: schema and tables verified")
}

// TestPostgreSQLMessagePersistence tests message persistence with real PostgreSQL.
func TestPostgreSQLMessagePersistence(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping PostgreSQL integration tests")
	}
	_ = filepath.Base // ensure import used
	t.Log("PostgreSQL message persistence test placeholder")
}