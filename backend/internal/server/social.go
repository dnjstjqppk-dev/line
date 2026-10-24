package server

import (
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
)

func (a *App) users(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(q) < 2 || len(q) > 80 {
		fail(w, 400, "search must be 2-80 characters")
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,display_name,COALESCE(handle,'') FROM identity.users WHERE state='active' AND id<>$1 AND (display_name ILIKE '%' || $2 || '%' OR handle ILIKE '%' || $2 || '%') LIMIT 30", claims(r).Subject, q)
	if e != nil {
		fail(w, 500, "search failed")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, handle string
		if rows.Scan(&id, &name, &handle) == nil {
			result = append(result, map[string]string{"id": id, "display_name": name, "handle": handle})
		}
	}
	send(w, 200, map[string]any{"users": result})
}
func (a *App) friends(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT u.id,u.display_name FROM social.friendships f JOIN identity.users u ON u.id=f.friend_id WHERE f.user_id=$1 AND u.state='active' ORDER BY u.display_name", claims(r).Subject)
	if e != nil {
		fail(w, 500, "friend list failed")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name string
		if rows.Scan(&id, &name) == nil {
			result = append(result, map[string]string{"id": id, "display_name": name})
		}
	}
	send(w, 200, map[string]any{"friends": result})
}
func (a *App) addFriend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FriendID string `json:"friend_id"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.FriendID) || in.FriendID == claims(r).Subject {
		fail(w, 400, "invalid friend_id")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	var count int
	e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM identity.users WHERE id=$1 AND state='active'", in.FriendID).Scan(&count)
	if e != nil || count == 0 {
		fail(w, 404, "user not found")
		return
	}
	e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM social.blocks WHERE (user_id=$1 AND blocked_id=$2) OR (user_id=$2 AND blocked_id=$1)", claims(r).Subject, in.FriendID).Scan(&count)
	if e != nil || count > 0 {
		fail(w, 403, "friendship unavailable")
		return
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO social.friendships(user_id,friend_id) VALUES ($1,$2),($2,$1) ON CONFLICT DO NOTHING", claims(r).Subject, in.FriendID)
	if e != nil || tx.Commit() != nil {
		fail(w, 500, "friendship creation failed")
		return
	}
	send(w, 201, map[string]string{"status": "added"})
}
