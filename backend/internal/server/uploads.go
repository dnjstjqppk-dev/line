package server

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
)

func (a *App) initUpload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OriginalName   string `json:"original_name"`
		MimeType       string `json:"mime_type"`
		TotalSize      int64  `json:"total_size"`
		ChunkSize      int64  `json:"chunk_size"`
		ChecksumSHA256 string `json:"checksum_sha256"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.OriginalName)) < 1 || len(in.MimeType) < 3 || in.TotalSize < 0 || in.TotalSize > 50*1024*1024 {
		fail(w, 400, "invalid upload metadata")
		return
	}
	if in.ChunkSize <= 0 {
		in.ChunkSize = 65536
	}
	if in.ChunkSize > 1024*1024 {
		in.ChunkSize = 1024 * 1024
	}
	var id string
	e := a.db.QueryRowContext(r.Context(),
		`INSERT INTO chat.upload_sessions(owner_id,original_name,mime_type,total_size,chunk_size,checksum_sha256,status,expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,'pending',$7) RETURNING id`,
		claims(r).Subject, in.OriginalName, in.MimeType, in.TotalSize, in.ChunkSize, in.ChecksumSHA256,
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339)).Scan(&id)
	if e != nil {
		fail(w, 500, "upload session creation failed")
		return
	}
	send(w, 201, map[string]any{
		"upload_id":  id,
		"chunk_size": in.ChunkSize,
		"total_size": in.TotalSize,
		"status":     "pending",
	})
}
func (a *App) uploadChunk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	chunkNumStr := r.PathValue("chunk")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid upload id")
		return
	}
	var chunkNum int
	for _, c := range chunkNumStr {
		if c < '0' || c > '9' {
			fail(w, 400, "invalid chunk number")
			return
		}
		chunkNum = chunkNum*10 + int(c-'0')
	}
	var in struct {
		ContentBase64 string `json:"content_base64"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "invalid chunk")
		return
	}
	content, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
	if err != nil || len(content) == 0 {
		fail(w, 400, "invalid chunk bytes")
		return
	}
	var owner, status string
	var totalSize, received, chunkSize int64
	if err := a.db.QueryRowContext(r.Context(),
		"SELECT owner_id,status,total_size,received_bytes,chunk_size FROM chat.upload_sessions WHERE id=$1", id).
		Scan(&owner, &status, &totalSize, &received, &chunkSize); err != nil || owner != claims(r).Subject {
		fail(w, 404, "upload session unavailable")
		return
	}
	if status == "completed" || status == "failed" || status == "expired" {
		fail(w, 409, "upload session not writable")
		return
	}
	if int64(len(content)) > chunkSize {
		fail(w, 400, "chunk too large")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "storage unavailable")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(),
		"INSERT INTO chat.upload_chunks(session_id,chunk_number,content,size_bytes) VALUES($1,$2,$3,$4) ON CONFLICT(session_id,chunk_number) DO UPDATE SET content=$3,size_bytes=$4,received_at=now()",
		id, chunkNum, content, len(content))
	if err != nil {
		fail(w, 500, "chunk storage failed")
		return
	}
	var newReceived int64
	if err := tx.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(size_bytes),0) FROM chat.upload_chunks WHERE session_id=$1", id).Scan(&newReceived); err != nil {
		fail(w, 500, "chunk accounting failed")
		return
	}
	_, err = tx.ExecContext(r.Context(),
		"UPDATE chat.upload_sessions SET received_bytes=$2,status='uploading',updated_at=now() WHERE id=$1", id, newReceived)
	if err != nil {
		fail(w, 500, "session update failed")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "commit failed")
		return
	}
	send(w, 200, map[string]any{
		"upload_id":      id,
		"chunk_number":   chunkNum,
		"received_bytes": newReceived,
		"total_size":     totalSize,
	})
}
func (a *App) completeUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid upload id")
		return
	}
	var in struct {
		MessageID string `json:"message_id"`
	}
	_ = decode(r, &in)

	var owner, status, originalName, mimeType, checksum string
	var totalSize, received int64
	if err := a.db.QueryRowContext(r.Context(),
		"SELECT owner_id,status,original_name,mime_type,total_size,received_bytes,COALESCE(checksum_sha256,'') FROM chat.upload_sessions WHERE id=$1", id).
		Scan(&owner, &status, &originalName, &mimeType, &totalSize, &received, &checksum); err != nil || owner != claims(r).Subject {
		fail(w, 404, "upload session unavailable")
		return
	}
	if status == "completed" {
		fail(w, 409, "upload already completed")
		return
	}
	if received != totalSize {
		fail(w, 400, "incomplete upload")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT content FROM chat.upload_chunks WHERE session_id=$1 ORDER BY chunk_number", id)
	if err != nil {
		fail(w, 500, "chunk read failed")
		return
	}
	defer rows.Close()
	var content []byte
	for rows.Next() {
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			fail(w, 500, "chunk read failed")
			return
		}
		content = append(content, chunk...)
	}
	if int64(len(content)) != totalSize {
		fail(w, 400, "size mismatch")
		return
	}
	computed := sha256hex(string(content))
	if checksum != "" && !strings.EqualFold(checksum, computed) {
		fail(w, 400, "checksum mismatch")
		return
	}
	rnd, e := randomSecret(16)
	if e != nil {
		fail(w, 500, "object key generation failed")
		return
	}
	objectKey := "media/" + claims(r).Subject + "/" + rnd
	var msgArg any
	if in.MessageID != "" && auth.ValidUUID(in.MessageID) {
		msgArg = in.MessageID
	}
	thumbnail := ""
	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		thumbnail = makeThumbnail(content)
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "storage unavailable")
		return
	}
	defer tx.Rollback()
	var attID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO chat.attachments(message_id,owner_id,object_key,mime_type,original_name,size_bytes,checksum_sha256,encryption_mode,status,thumbnail_base64)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'none','ready',$8) RETURNING id`,
		msgArg, claims(r).Subject, objectKey, mimeType, originalName, totalSize, computed, thumbnail).Scan(&attID)
	if err != nil {
		fail(w, 500, "attachment creation failed")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO chat.attachment_blobs(attachment_id,content) VALUES($1,$2)", attID, content); err != nil {
		fail(w, 500, "byte storage failed")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE chat.upload_sessions SET status='completed',updated_at=now() WHERE id=$1", id); err != nil {
		fail(w, 500, "session update failed")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "commit failed")
		return
	}
	send(w, 201, map[string]any{
		"id":              attID,
		"upload_id":       id,
		"object_key":      objectKey,
		"original_name":   originalName,
		"mime_type":       mimeType,
		"size_bytes":      totalSize,
		"checksum_sha256": computed,
		"status":          "ready",
	})
}