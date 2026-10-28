package server

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedWSTickets(t *testing.T) {
	cfg := Config{DSN: "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "tickets.db")), Secret: "0123456789abcdef0123456789abcdef"}
	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var userID string
	if err := first.db.QueryRow("INSERT INTO identity.users(email,password_hash,display_name) VALUES($1,$2,$3) RETURNING id", "ticket@example.test", "unused", "Ticket").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ticket, err := first.issueWSTicket(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := second.consumeWSTicket(ctx, ticket); err != nil || got != userID {
		t.Fatalf("cross-replica consume: %q %v", got, err)
	}
	if _, err := first.consumeWSTicket(ctx, ticket); err != sql.ErrNoRows {
		t.Fatalf("replay: %v", err)
	}
	ticket, err = first.issueWSTicket(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := first.db.QueryRow("SELECT token_hash FROM realtime.ws_tickets").Scan(&stored); err != nil || stored == ticket {
		t.Fatalf("stored raw ticket or query failed: %v", err)
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			app := first
			if i%2 != 0 {
				app = second
			}
			if _, err := app.consumeWSTicket(ctx, ticket); err == nil {
				winners.Add(1)
			} else if err != sql.ErrNoRows {
				t.Errorf("consume: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("successful consumers: %d", winners.Load())
	}
	ticket, err = first.issueWSTicket(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.db.Exec("UPDATE realtime.ws_tickets SET expires_at_unix=$1", time.Now().Unix()-1); err != nil {
		t.Fatal(err)
	}
	if _, err := second.consumeWSTicket(ctx, ticket); err != sql.ErrNoRows {
		t.Fatalf("expired: %v", err)
	}
}
