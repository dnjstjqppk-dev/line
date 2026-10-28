package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
)

type Message struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	SenderID       string         `json:"sender_id"`
	ClientID       string         `json:"client_id"`
	Sequence       int64          `json:"sequence"`
	Kind           string         `json:"kind"`
	Body           string         `json:"body"`
	ReplyTo        string         `json:"reply_to,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	EditedAt       *time.Time     `json:"edited_at,omitempty"`
	Reactions      map[string]int `json:"reactions,omitempty"`
	MyReactions    []string       `json:"my_reactions,omitempty"`
}

func (a *App) isMember(r *http.Request, id string) bool {
	if !auth.ValidUUID(id) {
		return false
	}
	var count int
	err := a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL", id, claims(r).Subject).Scan(&count)
	return err == nil && count > 0
}

func (a *App) conversationMemberIDsCtx(ctx context.Context, conversationID string) []string {
	rows, e := a.db.QueryContext(ctx, "SELECT user_id FROM chat.conversation_members WHERE conversation_id=$1 AND left_at IS NULL", conversationID)
	if e != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var uid string
		if rows.Scan(&uid) == nil {
			ids = append(ids, uid)
		}
	}
	return ids
}

func (a *App) conversationMemberIDs(r *http.Request, conversationID string) []string {
	return a.conversationMemberIDsCtx(r.Context(), conversationID)
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
			out = append(out, map[string]any{
				"id":                 id,
				"kind":               kind,
				"title":              title,
				"last_sequence":      seq,
				"last_read_sequence": read,
				"unread_count":       max(int64(0), seq-read),
				"updated_at":         updated,
			})
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
	rows, e := a.db.QueryContext(r.Context(), `SELECT id,conversation_id,sender_id,client_id,sequence,kind,body,COALESCE(reply_to,''),COALESCE(metadata,'{}'),created_at
 FROM chat.messages WHERE conversation_id=$1 AND sequence<$2 AND deleted_at IS NULL ORDER BY sequence DESC LIMIT $3`, id, before, limit)
	if e != nil {
		fail(w, 500, "message fetch failed")
		return
	}
	defer rows.Close()
	msgs := make([]Message, 0)
	for rows.Next() {
		var m Message
		var metaStr string
		if rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientID, &m.Sequence, &m.Kind, &m.Body, &m.ReplyTo, &metaStr, &m.CreatedAt) == nil {
			m.Body = a.msgCipher.decrypt(m.Body)

			if metaStr != "" && metaStr != "{}" {
				_ = json.Unmarshal([]byte(metaStr), &m.Metadata)
			}
			msgs = append(msgs, m)
		}
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	a.populateReactions(r.Context(), msgs, claims(r).Subject)
	send(w, 200, map[string]any{"messages": msgs})
}

// syncMessages provides offline catch-up synchronization after network reconnect (Phase 3: 050, E2E Scenario #11).
func (a *App) syncMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	after := int64(0)
	if raw := r.URL.Query().Get("after_sequence"); raw != "" {
		n, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || n < 0 {
			fail(w, 400, "invalid after_sequence")
			return
		}
		after = n
	}
	limit := auth.ParsePositiveInt(r.URL.Query().Get("limit"), 100, 500)
	rows, e := a.db.QueryContext(r.Context(), `SELECT id,conversation_id,sender_id,client_id,sequence,kind,body,COALESCE(reply_to,''),COALESCE(metadata,'{}'),created_at
 FROM chat.messages WHERE conversation_id=$1 AND sequence>$2 AND deleted_at IS NULL ORDER BY sequence ASC LIMIT $3`, id, after, limit)
	if e != nil {
		fail(w, 500, "message sync failed")
		return
	}
	defer rows.Close()
	msgs := make([]Message, 0)
	for rows.Next() {
		var m Message
		var metaStr string
			m.Body = a.msgCipher.decrypt(m.Body)

		if rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientID, &m.Sequence, &m.Kind, &m.Body, &m.ReplyTo, &metaStr, &m.CreatedAt) == nil {
			if metaStr != "" && metaStr != "{}" {
				_ = json.Unmarshal([]byte(metaStr), &m.Metadata)
			}
			msgs = append(msgs, m)
		}
	}
	a.populateReactions(r.Context(), msgs, claims(r).Subject)
	var lastSeq int64
	_ = a.db.QueryRowContext(r.Context(), "SELECT last_sequence FROM chat.conversations WHERE id=$1", id).Scan(&lastSeq)
	send(w, 200, map[string]any{"messages": msgs, "last_sequence": lastSeq})
}

var allowedMessageKinds = map[string]bool{
	"text": true, "image": true, "video": true, "audio": true, "file": true,
	"sticker": true, "location": true, "contact": true, "system": true, "poll": true,
}

func (a *App) sendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		ClientID   string         `json:"client_id"`
		Kind       string         `json:"kind"`
		Body       string         `json:"body"`
		ReplyTo    string         `json:"reply_to"`
		Metadata   map[string]any `json:"metadata"`
		Ciphertext string         `json:"ciphertext"`
	}
	if decode(r, &in) != nil || len(in.ClientID) < 8 || len(in.ClientID) > 128 || len(in.Body) > 20000 {
		fail(w, 400, "invalid message")
		return
	}
	if in.Kind == "" {
		in.Kind = "text"
	}
	if !allowedMessageKinds[in.Kind] {
		fail(w, 400, "unsupported message kind")
		return
	}
	if in.Kind == "text" && strings.TrimSpace(in.Body) == "" && in.Ciphertext == "" {
		fail(w, 400, "empty text message")
		return
	}
	if in.ReplyTo != "" && !auth.ValidUUID(in.ReplyTo) {
		fail(w, 400, "invalid reply_to")
		return
	}
	metaBytes := []byte("{}")
	if len(in.Metadata) > 0 {
		if b, err := json.Marshal(in.Metadata); err == nil {
			metaBytes = b
		}
	}
	var cipherBytes []byte
	if in.Ciphertext != "" {
		if b, err := base64.StdEncoding.DecodeString(in.Ciphertext); err == nil {
			cipherBytes = b
		} else {
			cipherBytes = []byte(in.Ciphertext)
		}
	}

	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db unavailable")
		return
	}
	defer tx.Rollback()

	var previous int64
	e = tx.QueryRowContext(r.Context(), "SELECT last_sequence FROM chat.conversations WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&previous)
	if e != nil {
		fail(w, 404, "conversation unavailable")
		return
	}
	var old Message
	var oldMeta string
	e = tx.QueryRowContext(r.Context(),
		`SELECT id,conversation_id,sender_id,client_id,sequence,kind,body,COALESCE(reply_to,''),COALESCE(metadata,'{}'),created_at
		 FROM chat.messages WHERE conversation_id=$1 AND sender_id=$2 AND client_id=$3`,
		id, claims(r).Subject, in.ClientID).Scan(&old.ID, &old.ConversationID, &old.SenderID, &old.ClientID, &old.Sequence, &old.Kind, &old.Body, &old.ReplyTo, &oldMeta, &old.CreatedAt)
	if e == nil {
		if oldMeta != "" && oldMeta != "{}" {
			_ = json.Unmarshal([]byte(oldMeta), &old.Metadata)
		}
		send(w, 200, map[string]any{"message": old, "duplicate": true})
		return
	}
	if e != sql.ErrNoRows {
		fail(w, 500, "message lookup failed")
		return
	}
	var replyArg any
	if in.ReplyTo != "" {
		replyArg = in.ReplyTo
	}
	m := Message{
		ConversationID: id,
		SenderID:       claims(r).Subject,
		ClientID:       in.ClientID,
		Sequence:       previous + 1,
		Kind:           in.Kind,
		Body:           in.Body,
		ReplyTo:        in.ReplyTo,
		Metadata:       in.Metadata,
	}
	encryptedBody := a.msgCipher.encrypt(m.Body)
	e = tx.QueryRowContext(r.Context(),
		`INSERT INTO chat.messages(conversation_id,sender_id,client_id,sequence,kind,body,reply_to,metadata,ciphertext)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`,
		m.ConversationID, m.SenderID, m.ClientID, m.Sequence, m.Kind, encryptedBody, replyArg, string(metaBytes), cipherBytes).Scan(&m.ID, &m.CreatedAt)
	if e != nil {
		fail(w, 500, "message persistence failed")
		return
	}
	if attachmentID, exists := in.Metadata["attachment_id"]; exists {
		attachment, ok := attachmentID.(string)
		if !ok || !auth.ValidUUID(attachment) {
			fail(w, 400, "invalid attachment id")
			return
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE chat.attachments SET message_id=$1
		 WHERE id=$2 AND owner_id=$3 AND message_id IS NULL AND status='ready'
		 AND EXISTS(SELECT 1 FROM chat.attachment_blobs WHERE attachment_id=$2)`, m.ID, attachment, claims(r).Subject)
		if err != nil {
			fail(w, 500, "attachment association failed")
			return
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			fail(w, 403, "attachment unavailable")
			return
		}
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
	ids := a.conversationMemberIDs(r, id)
	a.hub.Notify(ids, payload)
	// Queue push notifications for members other than sender.
	for _, uid := range ids {
		if uid == claims(r).Subject {
			continue
		}
		_, _ = a.db.ExecContext(r.Context(),
			"INSERT INTO realtime.push_queue(user_id,title,body,payload) VALUES($1,$2,$3,$4)",
			uid, "New message", m.Body, string(payload))
	}
	send(w, 201, map[string]any{"message": m, "duplicate": false})
}

func (a *App) editMessage(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	msgID := r.PathValue("msgId")
	if !a.isMember(r, convID) || !auth.ValidUUID(msgID) {
		fail(w, 403, "unavailable")
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Body) == "" || len(in.Body) > 20000 {
		fail(w, 400, "invalid body")
		return
	}
	res, e := a.db.ExecContext(r.Context(),
		"UPDATE chat.messages SET body=$3,edited_at=now() WHERE id=$1 AND conversation_id=$2 AND sender_id=$4 AND deleted_at IS NULL",
		msgID, convID, in.Body, claims(r).Subject)
	if e != nil {
		fail(w, 500, "edit failed")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "message not found or not editable")
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": "message.updated", "conversation_id": convID, "message_id": msgID, "body": in.Body})
	a.hub.Notify(a.conversationMemberIDs(r, convID), payload)
	send(w, 200, map[string]string{"status": "edited", "id": msgID})
}

func (a *App) deleteMessage(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	msgID := r.PathValue("msgId")
	if !a.isMember(r, convID) || !auth.ValidUUID(msgID) {
		fail(w, 403, "unavailable")
		return
	}
	res, e := a.db.ExecContext(r.Context(),
		"UPDATE chat.messages SET deleted_at=now(),body='' WHERE id=$1 AND conversation_id=$2 AND sender_id=$3 AND deleted_at IS NULL",
		msgID, convID, claims(r).Subject)
	if e != nil {
		fail(w, 500, "recall failed")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "message not found")
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": "message.deleted", "conversation_id": convID, "message_id": msgID})
	a.hub.Notify(a.conversationMemberIDs(r, convID), payload)
	send(w, 200, map[string]string{"status": "recalled", "id": msgID})
}

func (a *App) populateReactions(ctx context.Context, msgs []Message, viewerID string) {
	if len(msgs) == 0 {
		return
	}
	msgMap := make(map[string]*Message, len(msgs))
	placeholders := make([]string, len(msgs))
	args := make([]any, len(msgs))
	for i := range msgs {
		msgMap[msgs[i].ID] = &msgs[i]
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = msgs[i].ID
	}
	q := fmt.Sprintf("SELECT message_id, user_id, reaction FROM chat.message_reactions WHERE message_id IN (%s)", strings.Join(placeholders, ","))
	rows, err := a.db.QueryContext(ctx, q, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var mid, uid, rct string
		if rows.Scan(&mid, &uid, &rct) == nil {
			if m, ok := msgMap[mid]; ok {
				if m.Reactions == nil {
					m.Reactions = make(map[string]int)
				}
				m.Reactions[rct]++
				if uid == viewerID {
					m.MyReactions = append(m.MyReactions, rct)
				}
			}
		}
	}
}

func (a *App) reactMessage(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	msgID := r.PathValue("msgId")
	if !a.isMember(r, convID) || !auth.ValidUUID(msgID) {
		fail(w, 403, "unavailable")
		return
	}
	var in struct {
		Reaction string `json:"reaction"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Reaction) == "" || len(in.Reaction) > 32 {
		fail(w, 400, "invalid reaction")
		return
	}
	viewerID := claims(r).Subject

	var msgConvID string
	err := a.db.QueryRowContext(r.Context(), "SELECT conversation_id FROM chat.messages WHERE id=$1 AND deleted_at IS NULL", msgID).Scan(&msgConvID)
	if err == sql.ErrNoRows || msgConvID != convID {
		fail(w, 404, "message unavailable")
		return
	}
	if err != nil {
		fail(w, 500, "message lookup failed")
		return
	}

	var count int
	_ = a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.message_reactions WHERE message_id=$1 AND user_id=$2 AND reaction=$3", msgID, viewerID, in.Reaction).Scan(&count)

	status := "reacted"
	httpStatus := 201
	if count > 0 {
		_, e := a.db.ExecContext(r.Context(), "DELETE FROM chat.message_reactions WHERE message_id=$1 AND user_id=$2 AND reaction=$3", msgID, viewerID, in.Reaction)
		if e != nil {
			fail(w, 500, "reaction removal failed")
			return
		}
		status = "removed"
		httpStatus = 200
	} else {
		_, e := a.db.ExecContext(r.Context(), "INSERT INTO chat.message_reactions(message_id,user_id,reaction) VALUES ($1,$2,$3)", msgID, viewerID, in.Reaction)
		if e != nil {
			fail(w, 500, "reaction failed")
			return
		}
	}

	counts := make(map[string]int)
	rows, qErr := a.db.QueryContext(r.Context(), "SELECT reaction, count(*) FROM chat.message_reactions WHERE message_id=$1 GROUP BY reaction", msgID)
	if qErr == nil {
		defer rows.Close()
		for rows.Next() {
			var rct string
			var cnt int
			if rows.Scan(&rct, &cnt) == nil {
				counts[rct] = cnt
			}
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"type":            "message.reaction",
		"conversation_id": convID,
		"message_id":      msgID,
		"user_id":         viewerID,
		"reaction":        in.Reaction,
		"reactions":       counts,
		"status":          status,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), payload)
	send(w, httpStatus, map[string]any{
		"status":    status,
		"reaction":  in.Reaction,
		"reactions": counts,
	})
}

func (a *App) pinMessage(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		MessageID string `json:"message_id"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.MessageID) {
		fail(w, 400, "invalid message_id")
		return
	}
	_, e := a.db.ExecContext(r.Context(),
		"INSERT INTO chat.pinned_messages(conversation_id,message_id,pinned_by) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING",
		convID, in.MessageID, claims(r).Subject)
	if e != nil {
		fail(w, 500, "pin failed")
		return
	}
	send(w, 201, map[string]string{"status": "pinned", "message_id": in.MessageID})
}

func (a *App) listPins(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT p.message_id,p.pinned_by,p.pinned_at,m.body FROM chat.pinned_messages p JOIN chat.messages m ON m.id=p.message_id WHERE p.conversation_id=$1 ORDER BY p.pinned_at DESC",
		convID)
	if e != nil {
		fail(w, 500, "pins unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var mid, by, body string
		var at any
		if rows.Scan(&mid, &by, &at, &body) == nil {
			out = append(out, map[string]any{"message_id": mid, "pinned_by": by, "pinned_at": at, "body": body})
		}
	}
	send(w, 200, map[string]any{"pins": out})
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
	result, e := a.db.ExecContext(r.Context(),
		`UPDATE chat.conversation_members SET last_read_sequence=GREATEST(last_read_sequence,LEAST($3,(SELECT last_sequence FROM chat.conversations WHERE id=$1))) WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL`,
		id, claims(r).Subject, in.Sequence)
	if e != nil {
		fail(w, 500, "read update failed")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		fail(w, 404, "conversation unavailable")
		return
	}
	// Broadcast read receipt synchronization event across all conversation participants & multi-devices (E2E Scenario #6 & #7).
	payload, _ := json.Marshal(map[string]any{
		"type":            "conversation.read",
		"conversation_id": id,
		"user_id":         claims(r).Subject,
		"sequence":        in.Sequence,
	})
	a.hub.Notify(a.conversationMemberIDs(r, id), payload)
	send(w, 200, map[string]any{"status": "read", "up_to": in.Sequence})
}

func (a *App) typing(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.isMember(r, id) {
		fail(w, 403, "conversation unavailable")
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"type":            "conversation.typing",
		"conversation_id": id,
		"user_id":         claims(r).Subject,
	})
	a.hub.Notify(a.conversationMemberIDs(r, id), payload)
	send(w, 200, map[string]string{"status": "broadcasted"})
}

func (a *App) getPollView(ctx context.Context, pollID string, viewerID string) (map[string]any, error) {
	var p struct {
		ID             string
		ConversationID string
		Question       string
		Multiple       bool
		CreatedBy      string
		CreatedAt      time.Time
		ClosedAt       *time.Time
	}
	err := a.db.QueryRowContext(ctx,
		"SELECT id, conversation_id, question, multiple, created_by, created_at, closed_at FROM chat.polls WHERE id=$1",
		pollID).Scan(&p.ID, &p.ConversationID, &p.Question, &p.Multiple, &p.CreatedBy, &p.CreatedAt, &p.ClosedAt)
	if err != nil {
		return nil, err
	}

	optRows, err := a.db.QueryContext(ctx,
		"SELECT id, label FROM chat.poll_options WHERE poll_id=$1 ORDER BY id", pollID)
	if err != nil {
		return nil, err
	}
	defer optRows.Close()

	type optItem struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Text  string `json:"text"`
		Votes int    `json:"votes"`
		Voted bool   `json:"voted"`
	}
	var options []optItem
	optMap := make(map[string]*optItem)
	for optRows.Next() {
		var o optItem
		if optRows.Scan(&o.ID, &o.Label) == nil {
			o.Text = o.Label
			options = append(options, o)
		}
	}
	for i := range options {
		optMap[options[i].ID] = &options[i]
	}

	voterSet := make(map[string]bool)
	voteRows, err := a.db.QueryContext(ctx,
		`SELECT pv.option_id, pv.user_id FROM chat.poll_votes pv
		 JOIN chat.poll_options po ON po.id = pv.option_id
		 WHERE po.poll_id = $1`, pollID)
	if err == nil {
		defer voteRows.Close()
		for voteRows.Next() {
			var oid, uid string
			if voteRows.Scan(&oid, &uid) == nil {
				voterSet[uid] = true
				if item, ok := optMap[oid]; ok {
					item.Votes++
					if uid == viewerID {
						item.Voted = true
					}
				}
			}
		}
	}

	return map[string]any{
		"id":              p.ID,
		"conversation_id": p.ConversationID,
		"question":        p.Question,
		"multiple":        p.Multiple,
		"closed":          p.ClosedAt != nil,
		"created_by":      p.CreatedBy,
		"created_at":      p.CreatedAt,
		"total_voters":    len(voterSet),
		"options":         options,
	}, nil
}

func (a *App) createPoll(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
		Multiple bool     `json:"multiple"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Question)) < 2 || len(in.Options) < 2 || len(in.Options) > 20 {
		fail(w, 400, "invalid poll")
		return
	}
	viewerID := claims(r).Subject
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db error")
		return
	}
	defer tx.Rollback()
	var pollID string
	e = tx.QueryRowContext(r.Context(),
		"INSERT INTO chat.polls(conversation_id,question,multiple,created_by) VALUES ($1,$2,$3,$4) RETURNING id",
		convID, strings.TrimSpace(in.Question), in.Multiple, viewerID).Scan(&pollID)
	if e != nil {
		fail(w, 500, "poll creation failed")
		return
	}
	for _, rawLabel := range in.Options {
		label := strings.TrimSpace(rawLabel)
		if label == "" {
			continue
		}
		var optID string
		if e := tx.QueryRowContext(r.Context(), "INSERT INTO chat.poll_options(poll_id,label) VALUES ($1,$2) RETURNING id", pollID, label).Scan(&optID); e != nil {
			fail(w, 500, "option creation failed")
			return
		}
	}

	var lastSeq int64
	_ = tx.QueryRowContext(r.Context(), "SELECT COALESCE(MAX(sequence),0)+1 FROM chat.messages WHERE conversation_id=$1", convID).Scan(&lastSeq)
	var msgID string
	metaJSON, _ := json.Marshal(map[string]any{"poll_id": pollID})
	_ = tx.QueryRowContext(r.Context(),
		`INSERT INTO chat.messages(conversation_id,sender_id,client_id,sequence,kind,body,metadata)
		 VALUES ($1,$2,$3,$4,'poll',$5,$6) RETURNING id`,
		convID, viewerID, "poll-"+pollID, lastSeq, in.Question, string(metaJSON)).Scan(&msgID)

	if tx.Commit() != nil {
		fail(w, 500, "commit failed")
		return
	}

	view, err := a.getPollView(r.Context(), pollID, viewerID)
	if err != nil {
		fail(w, 500, "poll view failed")
		return
	}

	updatePayload, _ := json.Marshal(map[string]any{
		"type":            "poll.updated",
		"conversation_id": convID,
		"poll_id":         pollID,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), updatePayload)

	send(w, 201, view)
}

func (a *App) listPolls(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	viewerID := claims(r).Subject
	rows, err := a.db.QueryContext(r.Context(),
		"SELECT id FROM chat.polls WHERE conversation_id=$1 ORDER BY created_at DESC", convID)
	if err != nil {
		fail(w, 500, "polls unavailable")
		return
	}
	defer rows.Close()

	var pollIDs []string
	for rows.Next() {
		var pid string
		if rows.Scan(&pid) == nil {
			pollIDs = append(pollIDs, pid)
		}
	}

	out := make([]map[string]any, 0, len(pollIDs))
	for _, pid := range pollIDs {
		pv, err := a.getPollView(r.Context(), pid, viewerID)
		if err == nil {
			out = append(out, pv)
		}
	}
	send(w, 200, map[string]any{"polls": out})
}

func (a *App) votePoll(w http.ResponseWriter, r *http.Request) {
	pollID := r.PathValue("pollId")
	if !auth.ValidUUID(pollID) {
		fail(w, 400, "invalid poll_id")
		return
	}
	var in struct {
		OptionID string `json:"option_id"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.OptionID) {
		fail(w, 400, "invalid option_id")
		return
	}
	viewerID := claims(r).Subject

	var convID string
	var multiple bool
	var closedAt *time.Time
	err := a.db.QueryRowContext(r.Context(),
		"SELECT conversation_id, multiple, closed_at FROM chat.polls WHERE id=$1", pollID).Scan(&convID, &multiple, &closedAt)
	if err == sql.ErrNoRows {
		fail(w, 404, "poll not found")
		return
	}
	if err != nil {
		fail(w, 500, "poll lookup failed")
		return
	}
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	if closedAt != nil {
		fail(w, 400, "poll is closed")
		return
	}

	var optPollID string
	err = a.db.QueryRowContext(r.Context(), "SELECT poll_id FROM chat.poll_options WHERE id=$1", in.OptionID).Scan(&optPollID)
	if err != nil || optPollID != pollID {
		fail(w, 404, "option not found")
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "db error")
		return
	}
	defer tx.Rollback()

	var existingVoteCount int
	_ = tx.QueryRowContext(r.Context(),
		"SELECT count(*) FROM chat.poll_votes WHERE option_id=$1 AND user_id=$2", in.OptionID, viewerID).Scan(&existingVoteCount)

	if existingVoteCount > 0 {
		_, err = tx.ExecContext(r.Context(),
			"DELETE FROM chat.poll_votes WHERE option_id=$1 AND user_id=$2", in.OptionID, viewerID)
		if err != nil {
			fail(w, 500, "vote removal failed")
			return
		}
	} else {
		if !multiple {
			_, err = tx.ExecContext(r.Context(),
				`DELETE FROM chat.poll_votes WHERE user_id=$1 AND option_id IN
				 (SELECT id FROM chat.poll_options WHERE poll_id=$2)`, viewerID, pollID)
			if err != nil {
				fail(w, 500, "vote reset failed")
				return
			}
		}
		_, err = tx.ExecContext(r.Context(),
			"INSERT INTO chat.poll_votes(option_id, user_id) VALUES ($1,$2)", in.OptionID, viewerID)
		if err != nil {
			fail(w, 500, "vote failed")
			return
		}
	}

	if tx.Commit() != nil {
		fail(w, 500, "commit failed")
		return
	}

	view, err := a.getPollView(r.Context(), pollID, viewerID)
	if err != nil {
		fail(w, 500, "poll view failed")
		return
	}

	updatePayload, _ := json.Marshal(map[string]any{
		"type":            "poll.updated",
		"conversation_id": convID,
		"poll_id":         pollID,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), updatePayload)

	send(w, 200, map[string]any{
		"status": "voted",
		"poll":   view,
	})
}

func (a *App) closePoll(w http.ResponseWriter, r *http.Request) {
	pollID := r.PathValue("pollId")
	if !auth.ValidUUID(pollID) {
		fail(w, 400, "invalid poll_id")
		return
	}
	viewerID := claims(r).Subject

	var convID, createdBy string
	var closedAt *time.Time
	err := a.db.QueryRowContext(r.Context(),
		"SELECT conversation_id, created_by, closed_at FROM chat.polls WHERE id=$1", pollID).Scan(&convID, &createdBy, &closedAt)
	if err == sql.ErrNoRows {
		fail(w, 404, "poll not found")
		return
	}
	if err != nil {
		fail(w, 500, "poll lookup failed")
		return
	}
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	if createdBy != viewerID {
		fail(w, 403, "only creator can close poll")
		return
	}

	if closedAt == nil {
		_, err = a.db.ExecContext(r.Context(), "UPDATE chat.polls SET closed_at=now() WHERE id=$1", pollID)
		if err != nil {
			fail(w, 500, "poll close failed")
			return
		}
	}

	view, err := a.getPollView(r.Context(), pollID, viewerID)
	if err != nil {
		fail(w, 500, "poll view failed")
		return
	}

	updatePayload, _ := json.Marshal(map[string]any{
		"type":            "poll.updated",
		"conversation_id": convID,
		"poll_id":         pollID,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), updatePayload)

	send(w, 200, map[string]any{
		"status": "closed",
		"poll":   view,
	})
}

func (a *App) createAlbum(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Title)) < 1 || len(in.Title) > 120 {
		fail(w, 400, "invalid album title")
		return
	}
	var albumID string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO chat.albums(conversation_id,title,created_by) VALUES ($1,$2,$3) RETURNING id",
		convID, in.Title, claims(r).Subject).Scan(&albumID)
	if e != nil {
		fail(w, 500, "album creation failed")
		return
	}
	send(w, 201, map[string]string{"id": albumID, "title": in.Title})
}

func (a *App) listAlbums(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !a.isMember(r, convID) {
		fail(w, 403, "conversation unavailable")
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,title,created_by,created_at FROM chat.albums WHERE conversation_id=$1 ORDER BY created_at DESC", convID)
	if e != nil {
		fail(w, 500, "albums unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, title, by string
		var at any
		if rows.Scan(&id, &title, &by, &at) == nil {
			out = append(out, map[string]any{"id": id, "title": title, "created_by": by, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"albums": out})
}

func (a *App) saveMessage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MessageID string `json:"message_id"`
		// Legacy web clients send display fields; stored message data is authoritative.
		Body string `json:"body"`
		Kind string `json:"kind"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.MessageID) {
		fail(w, 400, "invalid message_id")
		return
	}
	var conversationID string
	e := a.db.QueryRowContext(r.Context(), "SELECT conversation_id FROM chat.messages WHERE id=$1 AND deleted_at IS NULL", in.MessageID).Scan(&conversationID)
	if e == sql.ErrNoRows {
		fail(w, 404, "message unavailable")
		return
	}
	if e != nil {
		fail(w, 500, "message lookup failed")
		return
	}
	if !a.isMember(r, conversationID) {
		fail(w, 404, "message unavailable")
		return
	}
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO chat.saved_messages(user_id,message_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", claims(r).Subject, in.MessageID)
	if e != nil {
		fail(w, 500, "save message failed")
		return
	}
	a.hub.Notify([]string{claims(r).Subject}, []byte(`{"type":"saved-messages.updated"}`))
	send(w, 201, map[string]string{"status": "saved", "message_id": in.MessageID})
}

func (a *App) deleteSavedMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid id")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "DELETE FROM chat.saved_messages WHERE user_id=$1 AND message_id=$2", claims(r).Subject, id)
	if e != nil {
		fail(w, 500, "delete saved message failed")
		return
	}
	a.hub.Notify([]string{claims(r).Subject}, []byte(`{"type":"saved-messages.updated"}`))
	send(w, 200, map[string]string{"status": "deleted", "message_id": id})
}

func (a *App) listSavedMessages(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		`SELECT m.id,m.conversation_id,m.body,m.kind,s.saved_at FROM chat.saved_messages s JOIN chat.messages m ON m.id=s.message_id
         WHERE s.user_id=$1 AND m.deleted_at IS NULL
         AND EXISTS(SELECT 1 FROM chat.conversation_members cm WHERE cm.conversation_id=m.conversation_id AND cm.user_id=$1 AND cm.left_at IS NULL)
         ORDER BY s.saved_at DESC`,
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "saved messages unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, cid, body, kind string
		var at any
		if rows.Scan(&id, &cid, &body, &kind, &at) == nil {
			out = append(out, map[string]any{"id": id, "message_id": id, "conversation_id": cid, "body": a.msgCipher.decrypt(body), "kind": kind, "created_at": at, "saved_at": at})
		}
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "saved messages unavailable")
		return
	}
	send(w, 200, map[string]any{"saved_messages": out})
}

func (a *App) unifiedSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 || len(q) > 100 {
		fail(w, 400, "query must be 2-100 chars")
		return
	}
	uid := claims(r).Subject
	msgRows, _ := a.db.QueryContext(r.Context(),
		`SELECT m.id,m.conversation_id,m.body,m.sequence FROM chat.messages m
		 JOIN chat.conversation_members cm ON cm.conversation_id=m.conversation_id
		 WHERE cm.user_id=$1 AND cm.left_at IS NULL AND m.deleted_at IS NULL
		 ORDER BY m.created_at DESC LIMIT 500`, uid)
	msgs := make([]any, 0)
	if msgRows != nil {
		for msgRows.Next() {
			var id, cid, body string
			var seq int64
			if msgRows.Scan(&id, &cid, &body, &seq) == nil {
				decrypted := a.msgCipher.decrypt(body)
				if strings.Contains(strings.ToLower(decrypted), strings.ToLower(q)) {
					msgs = append(msgs, map[string]any{"id": id, "conversation_id": cid, "body": decrypted, "sequence": seq})
				}
			}
		}
		if err := msgRows.Err(); err != nil {
			msgRows.Close()
		}
		msgRows.Close()
	}
	send(w, 200, map[string]any{"messages": msgs})
}
// runOutboxRelay periodically drains durable delivery outbox rows and notifies local sockets.
func (a *App) runOutboxRelay(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.drainOutbox(ctx)
		}
	}
}

// drainOutbox delivers pending outbox events to connected sockets and records per-instance delivery.
func (a *App) drainOutbox(ctx context.Context) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT o.id, o.conversation_id, o.payload
		 FROM realtime.delivery_outbox o
		 LEFT JOIN realtime.outbox_deliveries d ON d.outbox_id=o.id AND d.instance_id=$1
		 WHERE d.id IS NULL
		 ORDER BY o.id LIMIT 100`, a.instanceID)
	if err != nil {
		return
	}
	type pending struct {
		id      int64
		convID  string
		payload []byte
	}
	var batch []pending
	for rows.Next() {
		var p pending
		var payload string
		if err := rows.Scan(&p.id, &p.convID, &payload); err == nil {
			p.payload = []byte(payload)
			batch = append(batch, p)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return
	}
	rows.Close()
	for _, p := range batch {
		ids := a.conversationMemberIDsCtx(ctx, p.convID)
		a.hub.Notify(ids, p.payload)
		_, _ = a.db.ExecContext(ctx,
			"INSERT INTO realtime.outbox_deliveries(outbox_id,instance_id) VALUES($1,$2) ON CONFLICT(outbox_id,instance_id) DO NOTHING",
			p.id, a.instanceID)
	}
}
