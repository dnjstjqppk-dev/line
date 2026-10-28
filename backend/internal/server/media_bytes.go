package server

import (
	"github.com/example/global-messenger/backend/internal/auth"
	"mime"
	"net/http"
)

func (a *App) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid attachment id")
		return
	}
	var data []byte
	var name string
	err := a.db.QueryRowContext(r.Context(), `SELECT b.content,a.original_name FROM chat.attachments a
 JOIN chat.attachment_blobs b ON b.attachment_id=a.id
 WHERE a.id=$1 AND a.status='ready' AND
 (a.message_id IS NULL OR EXISTS(SELECT 1 FROM chat.messages m WHERE m.id=a.message_id AND m.deleted_at IS NULL))
 AND (a.owner_id=$2 OR EXISTS(SELECT 1 FROM chat.messages m JOIN chat.conversation_members cm ON cm.conversation_id=m.conversation_id
 WHERE m.id=a.message_id AND m.deleted_at IS NULL AND cm.user_id=$2 AND cm.left_at IS NULL))`, id, claims(r).Subject).Scan(&data, &name)
	if err != nil {
		fail(w, 404, "attachment unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (a *App) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid attachment id")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "storage unavailable")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), "UPDATE chat.attachments SET status='deleted' WHERE id=$1 AND owner_id=$2 AND status='ready'", id, claims(r).Subject)
	if err != nil {
		fail(w, 500, "delete failed")
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		fail(w, 404, "attachment unavailable")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM chat.attachment_blobs WHERE attachment_id=$1", id); err != nil {
		fail(w, 500, "delete failed")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "delete failed")
		return
	}
	send(w, 200, map[string]string{"status": "deleted"})
}
