package server

import (
	"context"
	"database/sql"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"time"
)

// Store only the digest; atomic DELETE RETURNING consumes a ticket once across
// all replicas sharing the database, without load-balancer affinity.
func (a *App) issueWSTicket(ctx context.Context, userID string, sessionID ...string) (string, error) {
	ticket, err := randomSecret(32)
	if err != nil {
		return "", err
	}
	if len(sessionID) == 1 {
		ticket += "." + sessionID[0]
	}
	now := time.Now().Unix()
	if _, err = a.db.ExecContext(ctx, "DELETE FROM realtime.ws_tickets WHERE expires_at_unix <= $1", now); err != nil {
		return "", err
	}
	_, err = a.db.ExecContext(ctx, "INSERT INTO realtime.ws_tickets(token_hash,user_id,expires_at_unix) VALUES($1,$2,$3)", sha256hex(ticket), userID, now+30)
	return ticket, err
}

func (a *App) consumeWSTicket(ctx context.Context, ticket string) (string, error) {
	var userID string
	var expires int64
	err := a.db.QueryRowContext(ctx, "DELETE FROM realtime.ws_tickets WHERE token_hash=$1 RETURNING user_id,expires_at_unix", sha256hex(ticket)).Scan(&userID, &expires)
	if err != nil {
		return "", err
	}
	if expires <= time.Now().Unix() {
		return "", sql.ErrNoRows
	}
	return userID, nil
}

func (a *App) wsTicket(w http.ResponseWriter, r *http.Request) {
	ticket, e := a.issueWSTicket(r.Context(), claims(r).Subject, claims(r).SessionID)
	if e != nil {
		fail(w, 500, "ticket unavailable")
		return
	}
	send(w, 200, map[string]any{"ticket": ticket, "expires_in": 30})
}
func (a *App) websocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("ticket")
	if token == "" {
		fail(w, 401, "ticket required")
		return
	}
	uid, ticketErr := a.consumeWSTicket(r.Context(), token)
	if ticketErr == sql.ErrNoRows {
		fail(w, 401, "invalid or expired ticket")
		return
	}
	if ticketErr != nil {
		fail(w, 503, "ticket storage unavailable")
		return
	}
	_, sessionID, _ := strings.Cut(token, ".")
	if !a.sessionActive(r.Context(), uid, sessionID) {
		fail(w, 401, "session unavailable")
		return
	}
	upgrader := websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		} // native clients may omit Origin
		for _, allowed := range a.cfg.Origins {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}
		return false
	}}
	conn, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(70 * time.Second)) })
	messages, unsubscribe := a.hub.Subscribe(uid)
	defer unsubscribe()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg := <-messages:
			if !a.sessionActive(r.Context(), uid, sessionID) {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(websocket.TextMessage, msg) != nil {
				return
			}
		case <-ticker.C:
			if !a.sessionActive(r.Context(), uid, sessionID) {
				return
			}
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		case <-finished:
			return
		case <-r.Context().Done():
			return
		}
	}
}
