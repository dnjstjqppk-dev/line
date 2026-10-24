package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestApplyAllPhasesSchema(t *testing.T) {
	conn, err := sql.Open("pgsqlite", "sqlite://file:test_schema?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open pgsqlite: %v", err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)

	schemaPath := filepath.Join("..", "..", "..", "database", "001_all_phases.sql")
	if err := ApplySchemaFile(context.Background(), conn, schemaPath); err != nil {
		t.Fatalf("ApplySchemaFile failed: %v", err)
	}

	var id string
	err = conn.QueryRow(`INSERT INTO identity.users(email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id`,
		"alice@example.com", "hash123", "Alice").Scan(&id)
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %q", id)
	}
}

