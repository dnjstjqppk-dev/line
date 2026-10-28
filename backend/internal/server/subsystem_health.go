package server

import (
	"net/http"
	"time"
)

// callHealth checks the health of the calling subsystem.
func (a *App) callHealth(w http.ResponseWriter, r *http.Request) {
	var activeCalls int
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM calling.calls WHERE state IN ('ringing', 'connected')").Scan(&activeCalls)

	var totalCalls int
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM calling.calls").Scan(&totalCalls)

	send(w, 200, map[string]any{
		"status":       "ok",
		"active_calls": activeCalls,
		"total_calls":  totalCalls,
	})
}

// mediaHealth checks the health of the media subsystem.
func (a *App) mediaHealth(w http.ResponseWriter, r *http.Request) {
	var totalAttachments int
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.attachments WHERE status='ready'").Scan(&totalAttachments)

	var totalBytes int64
	a.db.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(size_bytes), 0) FROM chat.attachments WHERE status='ready'").Scan(&totalBytes)

	var pendingUploads int
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.upload_sessions WHERE status IN ('pending', 'uploading')").Scan(&pendingUploads)

	send(w, 200, map[string]any{
		"status":           "ok",
		"total_attachments": totalAttachments,
		"total_bytes":      totalBytes,
		"pending_uploads":  pendingUploads,
	})
}

// dbStats returns database statistics.
func (a *App) dbStats(w http.ResponseWriter, r *http.Request) {
	var userCount, messageCount, conversationCount int
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM identity.users WHERE state='active'").Scan(&userCount)
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.messages WHERE deleted_at IS NULL").Scan(&messageCount)
	a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM chat.conversations WHERE deleted_at IS NULL").Scan(&conversationCount)

	send(w, 200, map[string]any{
		"users":          userCount,
		"messages":       messageCount,
		"conversations":  conversationCount,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})
}