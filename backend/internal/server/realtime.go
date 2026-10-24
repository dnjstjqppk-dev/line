package server

import (
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"time"
)

func (a *App) wsTicket(w http.ResponseWriter, r *http.Request) {
	ticket, e := a.hub.IssueTicket(claims(r).Subject)
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
	uid, ok := a.hub.ConsumeTicket(token)
	if !ok {
		fail(w, 401, "invalid or expired ticket")
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
			if conn.WriteMessage(websocket.TextMessage, msg) != nil {
				return
			}
		case <-ticker.C:
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
