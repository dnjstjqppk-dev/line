package server

import (
	"database/sql"
	"encoding/json"
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	ClientID       string    `json:"client_id"`
	Sequence       int64     `json:"sequence"`
	Kind           string    `json:"kind"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

func (a *App) isMember(r *http.Request, id string) bool {
	if !auth.ValidUUID(id) {
		return false
	}
	var count int
	err := a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL", id, claims(r).Subject).Scan(&count)
	return err == nil && count > 0
}
func (a *App) conversations(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT c.id,c.kind,COALESCE(c.title,''),c.last_sequence,m.last_read_sequence,c.updated_at
 FROM chat.conversation_members m JOIN chat.conversations c ON c.id=m.conversation_id
 WHERE m.user_id=$1 AND m.left_at IS NULL AND c.deleted_at IS NULL ORDER BY c.updated_at DESC LIMIT 200`, claims(r).Subject)
	if e != nil {
		fail(w, 500, "conversation list failed")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, kind, title string
		var seq, read int64
		var updated time.Time
		if rows.Scan(&id, &kind, &title, &seq, &read, &updated) == nil {
			out = append(out, map[string]any{"id": id, "kind": kind, "title": title, "last_sequence": seq, "last_read_sequence": read, "unread_count": max(int64(0), seq-read), "updated_at": updated})
		}
	}
	send(w, 200, map[string]any{"conversations": out})
}
func (a *App) createConversation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind      string   `json:"kind"`
		Title     string   `json:"title"`
		MemberIDs []string `json:"member_ids"`
	}
	if decode(r, &in) != nil || (in.Kind != "direct" && in.Kind != "group" && in.Kind != "self") {
		fail(w, 400, "invalid conversation")
		return
	}
	mine := claims(r).Subject
	seen := map[string]bool{mine: true}
	members := []string{mine}
	for _, id := range in.MemberIDs {
		if !auth.ValidUUID(id) {
			fail(w, 400, "invalid member id")
			return
		}
		if !seen[id] {
			seen[id] = true
			members = append(members, id)
		}
	}
	if in.Kind == "direct" && len(members) != 2 || in.Kind == "self" && len(members) != 1 || in.Kind == "group" && (len(members) < 2 || len(members) > 200) || len(in.Title) > 120 {
		fail(w, 400, "invalid members or title")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db unavailable")
		return
	}
	defer tx.Rollback()
	for _, id := range members {
		var state string
		if e = tx.QueryRowContext(r.Context(), "SELECT state FROM identity.users WHERE id=$1", id).Scan(&state); e != nil || state != "active" {
			fail(w, 400, "inactive or missing member")
			return
		}
	}
	if in.Kind == "direct" {
		var count int
		e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM social.friendships WHERE user_id=$1 AND friend_id=$2", mine, members[1]).Scan(&count)
		if e != nil || count == 0 {
			fail(w, 403, "only friends can start direct conversations")
			return
		}
	}
	if in.Kind == "direct" {
		var existing string
		err := tx.QueryRowContext(r.Context(), `SELECT c.id FROM chat.conversations c WHERE c.kind='direct' AND c.deleted_at IS NULL
  AND (SELECT count(*) FROM chat.conversation_members cm WHERE cm.conversation_id=c.id AND cm.left_at IS NULL)=2
  AND EXISTS(SELECT 1 FROM chat.conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=$1 AND cm.left_at IS NULL)
  AND EXISTS(SELECT 1 FROM chat.conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=$2 AND cm.left_at IS NULL) LIMIT 1`, mine, members[1]).Scan(&existing)
		if err == nil {
			send(w, 200, map[string]string{"id": existing, "status": "existing"})
			return
		}
	}
	var id string
	e = tx.QueryRowContext(r.Context(), "INSERT INTO chat.conversations(kind,title,created_by) VALUES ($1,$2,$3) RETURNING id", in.Kind, in.Title, mine).Scan(&id)
	if e != nil {
		fail(w, 500, "creation failed")
		return
	}
	for _, member := range members {
		role := "member"
		if member == mine {
			role = "owner"
		}
		_, e = tx.ExecContext(r.Context(), "INSERT INTO chat.conversation_members(conversation_id,user_id,role) VALUES ($1,$2,$3)", id, member, role)
		if e != nil {
			fail(w, 500, "members failed")
			return
		}
	}
	if tx.Commit() != nil {
		fail(w, 500, "commit failed")
		return
	}
	send(w, 201, map[string]any{"id": id, "kind": in.Kind, "members": members})
}
func (a *App) messages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	limit := auth.ParsePositiveInt(r.URL.Query().Get("limit"), 50, 100)
	before := int64(9223372036854775807)
	if raw := r.URL.Query().Get("before"); raw != "" {
		n, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || n < 1 {
			fail(w, 400, "invalid cursor")
			return
		}
		before = n
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT id,conversation_id,sender_id,client_id,sequence,kind,body,created_at
 FROM chat.messages WHERE conversation_id=$1 AND sequence<$2 AND deleted_at IS NULL ORDER BY sequence DESC LIMIT $3`, id, before, limit)
	if e != nil {
		fail(w, 500, "message fetch failed")
		return
	}
	defer rows.Close()
	msgs := make([]Message, 0)
	for rows.Next() {
		var m Message
		if rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientID, &m.Sequence, &m.Kind, &m.Body, &m.CreatedAt) == nil {
			msgs = append(msgs, m)
		}
	}
	// Reverse into chronological sequence for UI.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	send(w, 200, map[string]any{"messages": msgs})
}
func (a *App) sendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		ClientID string `json:"client_id"`
		Kind     string `json:"kind"`
		Body     string `json:"body"`
	}
	if decode(r, &in) != nil || len(in.ClientID) < 8 || len(in.ClientID) > 128 || len(in.Body) > 20000 {
		fail(w, 400, "invalid message")
		return
	}
	if in.Kind == "" {
		in.Kind = "text"
	}
	if in.Kind != "text" || strings.TrimSpace(in.Body) == "" {
		fail(w, 400, "only nonempty text messages implemented in MVP")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db unavailable")
		return
	}
	defer tx.Rollback()
	// Serialize sequence allocation and duplicate checks for a conversation.
	var previous int64
	e = tx.QueryRowContext(r.Context(), "SELECT last_sequence FROM chat.conversations WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&previous)
	if e != nil {
		fail(w, 404, "conversation unavailable")
		return
	}
	var old Message
	e = tx.QueryRowContext(r.Context(), `SELECT id,conversation_id,sender_id,client_id,sequence,kind,body,created_at FROM chat.messages WHERE conversation_id=$1 AND sender_id=$2 AND client_id=$3`, id, claims(r).Subject, in.ClientID).Scan(&old.ID, &old.ConversationID, &old.SenderID, &old.ClientID, &old.Sequence, &old.Kind, &old.Body, &old.CreatedAt)
	if e == nil {
		send(w, 200, map[string]any{"message": old, "duplicate": true})
		return
	}
	if e != sql.ErrNoRows {
		fail(w, 500, "message lookup failed")
		return
	}
	m := Message{ConversationID: id, SenderID: claims(r).Subject, ClientID: in.ClientID, Sequence: previous + 1, Kind: in.Kind, Body: in.Body}
	e = tx.QueryRowContext(r.Context(), `INSERT INTO chat.messages(conversation_id,sender_id,client_id,sequence,kind,body)
 VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at`, m.ConversationID, m.SenderID, m.ClientID, m.Sequence, m.Kind, m.Body).Scan(&m.ID, &m.CreatedAt)
	if e != nil {
		fail(w, 500, "message persistence failed")
		return
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE chat.conversations SET last_sequence=$2,updated_at=now() WHERE id=$1", id, m.Sequence)
	if e != nil {
		fail(w, 500, "sequence update failed")
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": "message.created", "message": m})
	_, e = tx.ExecContext(r.Context(), "INSERT INTO realtime.delivery_outbox(conversation_id,message_id,event_type,payload) VALUES($1,$2,'message.created',$3)", id, m.ID, string(payload))
	if e != nil {
		fail(w, 500, "event persistence failed")
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "commit failed")
		return
	}
	// Local fanout. A durable outbox publisher + NATS subscriber is required before scaling beyond one pod.
	rows, e := a.db.QueryContext(r.Context(), "SELECT user_id FROM chat.conversation_members WHERE conversation_id=$1 AND left_at IS NULL", id)
	if e == nil {
		var ids []string
		for rows.Next() {
			var uid string
			if rows.Scan(&uid) == nil {
				ids = append(ids, uid)
			}
		}
		rows.Close()
		a.hub.Notify(ids, payload)
	}
	send(w, 201, map[string]any{"message": m, "duplicate": false})
}
func (a *App) markRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		Sequence int64 `json:"sequence"`
	}
	if decode(r, &in) != nil || in.Sequence < 0 {
		fail(w, 400, "invalid sequence")
		return
	}
	result, e := a.db.ExecContext(r.Context(), `UPDATE chat.conversation_members SET last_read_sequence=GREATEST(last_read_sequence,LEAST($3,(SELECT last_sequence FROM chat.conversations WHERE id=$1))) WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL`, id, claims(r).Subject, in.Sequence)
	if e != nil {
		fail(w, 500, "read update failed")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		fail(w, 404, "conversation unavailable")
		return
	}
	send(w, 200, map[string]any{"status": "read", "up_to": in.Sequence})
}
