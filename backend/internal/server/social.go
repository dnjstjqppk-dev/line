package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
)

func (a *App) users(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(q) < 2 || len(q) > 80 {
		fail(w, 400, "search must be 2-80 characters")
		return
	}
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,display_name,COALESCE(handle,''),COALESCE(bio,'') FROM identity.users WHERE state='active' AND id<>$1 AND (display_name ILIKE '%' || $2 || '%' OR handle ILIKE '%' || $2 || '%') LIMIT 30",
		claims(r).Subject, q)
	if e != nil {
		fail(w, 500, "search failed")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, handle, bio string
		if rows.Scan(&id, &name, &handle, &bio) == nil {
			result = append(result, map[string]string{"id": id, "display_name": name, "handle": handle, "bio": bio})
		}
	}
	send(w, 200, map[string]any{"users": result})
}

func (a *App) friends(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT u.id,u.display_name,COALESCE(u.handle,''),COALESCE(u.bio,'') FROM social.friendships f JOIN identity.users u ON u.id=f.friend_id WHERE f.user_id=$1 AND u.state='active' ORDER BY u.display_name",
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "friend list failed")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, handle, bio string
		if rows.Scan(&id, &name, &handle, &bio) == nil {
			result = append(result, map[string]string{"id": id, "display_name": name, "handle": handle, "bio": bio})
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
	send(w, 201, map[string]string{"status": "added", "friend_id": in.FriendID})
}

func (a *App) removeFriend(w http.ResponseWriter, r *http.Request) {
	friendID := r.PathValue("id")
	if !auth.ValidUUID(friendID) {
		fail(w, 400, "invalid friend id")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "DELETE FROM social.friendships WHERE (user_id=$1 AND friend_id=$2) OR (user_id=$2 AND friend_id=$1)", claims(r).Subject, friendID)
	if e != nil {
		fail(w, 500, "friend removal failed")
		return
	}
	send(w, 200, map[string]string{"status": "removed", "friend_id": friendID})
}

func (a *App) blocks(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT u.id,u.display_name,COALESCE(b.reason,''),b.created_at FROM social.blocks b JOIN identity.users u ON u.id=b.blocked_id WHERE b.user_id=$1 ORDER BY b.created_at DESC",
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "blocks unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, name, reason string
		var created any
		if rows.Scan(&id, &name, &reason, &created) == nil {
			out = append(out, map[string]any{"id": id, "display_name": name, "reason": reason, "created_at": created})
		}
	}
	send(w, 200, map[string]any{"blocks": out})
}

func (a *App) blockUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BlockedID string `json:"blocked_id"`
		Reason    string `json:"reason"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.BlockedID) || in.BlockedID == claims(r).Subject {
		fail(w, 400, "invalid blocked_id")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(r.Context(), "DELETE FROM social.friendships WHERE (user_id=$1 AND friend_id=$2) OR (user_id=$2 AND friend_id=$1)", claims(r).Subject, in.BlockedID)
	_, e = tx.ExecContext(r.Context(), "INSERT INTO social.blocks(user_id,blocked_id,reason) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING", claims(r).Subject, in.BlockedID, in.Reason)
	if e != nil || tx.Commit() != nil {
		fail(w, 500, "block failed")
		return
	}
	send(w, 201, map[string]string{"status": "blocked", "blocked_id": in.BlockedID})
}

func (a *App) unblockUser(w http.ResponseWriter, r *http.Request) {
	blockedID := r.PathValue("id")
	if !auth.ValidUUID(blockedID) {
		fail(w, 400, "invalid blocked id")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "DELETE FROM social.blocks WHERE user_id=$1 AND blocked_id=$2", claims(r).Subject, blockedID)
	if e != nil {
		fail(w, 500, "unblock failed")
		return
	}
	send(w, 200, map[string]string{"status": "unblocked", "blocked_id": blockedID})
}

func (a *App) createFriendQR(w http.ResponseWriter, r *http.Request) {
	token, e := randomSecret(24)
	if e != nil {
		fail(w, 500, "token generation failed")
		return
	}
	exp := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	var id string
	e = a.db.QueryRowContext(r.Context(),
		"INSERT INTO social.invite_links(creator_id,token_hash,target,expires_at) VALUES ($1,$2,'friend',$3) RETURNING id",
		claims(r).Subject, sha256hex(token), exp).Scan(&id)
	if e != nil {
		fail(w, 500, "qr creation failed")
		return
	}
	send(w, 201, map[string]any{"id": id, "qr_token": token, "expires_at": exp})
}

func (a *App) redeemFriendQR(w http.ResponseWriter, r *http.Request) {
	var in struct {
		QRToken string `json:"qr_token"`
	}
	if decode(r, &in) != nil || len(in.QRToken) < 16 {
		fail(w, 400, "invalid qr_token")
		return
	}
	var creatorID string
	var revoked any
	e := a.db.QueryRowContext(r.Context(),
		"SELECT creator_id,revoked_at FROM social.invite_links WHERE token_hash=$1 AND target='friend'",
		sha256hex(in.QRToken)).Scan(&creatorID, &revoked)
	if e != nil || revoked != nil || creatorID == claims(r).Subject {
		fail(w, 400, "invalid or expired friend qr")
		return
	}
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO social.friendships(user_id,friend_id) VALUES ($1,$2),($2,$1) ON CONFLICT DO NOTHING", claims(r).Subject, creatorID)
	if e != nil {
		fail(w, 500, "friend qr redeem failed")
		return
	}
	send(w, 201, map[string]string{"status": "added", "friend_id": creatorID})
}

func (a *App) syncContacts(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phones []string `json:"phones"`
		Emails []string `json:"emails"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "invalid contact sync payload")
		return
	}
	matched := make([]any, 0)
	seen := map[string]bool{claims(r).Subject: true}
	for _, p := range in.Phones {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var id, name string
		if a.db.QueryRowContext(r.Context(), "SELECT id,display_name FROM identity.users WHERE phone_e164=$1 AND state='active'", p).Scan(&id, &name) == nil && !seen[id] {
			seen[id] = true
			matched = append(matched, map[string]string{"id": id, "display_name": name, "matched_by": "phone"})
		}
	}
	for _, em := range in.Emails {
		em = strings.ToLower(strings.TrimSpace(em))
		if em == "" {
			continue
		}
		var id, name string
		if a.db.QueryRowContext(r.Context(), "SELECT id,display_name FROM identity.users WHERE email=$1 AND state='active'", em).Scan(&id, &name) == nil && !seen[id] {
			seen[id] = true
			matched = append(matched, map[string]string{"id": id, "display_name": name, "matched_by": "email"})
		}
	}
	send(w, 200, map[string]any{"matched_users": matched})
}
