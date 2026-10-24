package realtime

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Hub struct {
	mu      sync.RWMutex
	sockets map[string]map[chan []byte]struct{}
	tickets map[string]Ticket
}
type Ticket struct {
	UserID  string
	Expires time.Time
}

func New() *Hub {
	return &Hub{sockets: make(map[string]map[chan []byte]struct{}), tickets: make(map[string]Ticket)}
}
func (h *Hub) IssueTicket(id string) (string, error) {
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return "", e
	}
	t := base64.RawURLEncoding.EncodeToString(raw)
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, v := range h.tickets {
		if time.Now().After(v.Expires) {
			delete(h.tickets, k)
		}
	}
	h.tickets[t] = Ticket{UserID: id, Expires: time.Now().Add(30 * time.Second)}
	return t, nil
}
func (h *Hub) ConsumeTicket(t string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.tickets[t]
	delete(h.tickets, t)
	return v.UserID, ok && time.Now().Before(v.Expires)
}
func (h *Hub) Subscribe(id string) (chan []byte, func()) {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	if h.sockets[id] == nil {
		h.sockets[id] = make(map[chan []byte]struct{})
	}
	h.sockets[id][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.sockets[id], ch)
			if len(h.sockets[id]) == 0 {
				delete(h.sockets, id)
			}
			close(ch)
			h.mu.Unlock()
		})
	}
	return ch, cleanup
}
func (h *Hub) Notify(ids []string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, id := range ids {
		for ch := range h.sockets[id] {
			select {
			case ch <- payload:
			default:
			}
		}
	}
}
