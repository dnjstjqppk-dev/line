package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"

	"github.com/example/global-messenger/backend/internal/auth"
)

// uploadAttachment stores media/file metadata and checksums (Phase 3: 044-045, E2E Scenario #9).
func (a *App) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MessageID      string `json:"message_id"`
		OriginalName   string `json:"original_name"`
		MimeType       string `json:"mime_type"`
		SizeBytes      int64  `json:"size_bytes"`
		ChecksumSHA256 string `json:"checksum_sha256"`
		EncryptionMode string `json:"encryption_mode"`
		ContentBase64  string `json:"content_base64"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.OriginalName)) < 1 || len(in.OriginalName) > 255 || len(in.MimeType) < 3 || in.SizeBytes < 0 {
		fail(w, 400, "invalid attachment metadata")
		return
	}
	if in.EncryptionMode != "" && in.EncryptionMode != "none" {
		fail(w, 400, "unsupported encryption mode")
		return
	}
	in.EncryptionMode = "none"
	content, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
	if err != nil || len(content) == 0 || len(content) > 512*1024 || int64(len(content)) != in.SizeBytes {
		fail(w, 400, "invalid bytes or size; maximum 512 KiB")
		return
	}
	checksum := sha256hex(string(content))
	if in.ChecksumSHA256 != "" && !strings.EqualFold(in.ChecksumSHA256, checksum) {
		fail(w, 400, "checksum mismatch")
		return
	}
	in.ChecksumSHA256 = checksum
	rnd, e := randomSecret(16)
	if e != nil {
		fail(w, 500, "object key generation failed")
		return
	}
	objectKey := "media/" + claims(r).Subject + "/" + rnd
	var msgArg any
	if in.MessageID != "" {
		var conversationID, senderID string
		if !auth.ValidUUID(in.MessageID) {
			fail(w, 400, "invalid message id")
			return
		}
		if err := a.db.QueryRowContext(r.Context(), "SELECT conversation_id,sender_id FROM chat.messages WHERE id=$1 AND deleted_at IS NULL", in.MessageID).Scan(&conversationID, &senderID); err != nil || senderID != claims(r).Subject || !a.isMember(r, conversationID) {
			fail(w, 403, "message forbidden")
			return
		}
		msgArg = in.MessageID
	}
	thumbnail := ""
	if strings.HasPrefix(strings.ToLower(in.MimeType), "image/") {
		thumbnail = makeThumbnail(content)
	}

	var id string
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "storage unavailable")
		return
	}
	defer tx.Rollback()
	e = tx.QueryRowContext(r.Context(),
		`INSERT INTO chat.attachments(message_id,owner_id,object_key,mime_type,original_name,size_bytes,checksum_sha256,encryption_mode,status,thumbnail_base64)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'ready',$9) RETURNING id`,
		msgArg, claims(r).Subject, objectKey, in.MimeType, in.OriginalName, in.SizeBytes, in.ChecksumSHA256, in.EncryptionMode, thumbnail).Scan(&id)
	if e != nil {
		fail(w, 500, "attachment creation failed")
		return
	}
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO chat.attachment_blobs(attachment_id,content) VALUES($1,$2)", id, content); e != nil {
		fail(w, 500, "byte storage failed")
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "storage commit failed")
		return
	}
	send(w, 201, map[string]any{
		"id":              id,
		"object_key":      objectKey,
		"original_name":   in.OriginalName,
		"mime_type":       in.MimeType,
		"size_bytes":      in.SizeBytes,
		"checksum_sha256": in.ChecksumSHA256,
		"status":          "ready",
	})
}

func (a *App) getAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid attachment id")
		return
	}
	var ownerID, objKey, mime, origName, checksum, encMode, status string
	var size int64
	e := a.db.QueryRowContext(r.Context(),
		"SELECT owner_id,object_key,mime_type,original_name,size_bytes,COALESCE(checksum_sha256,''),encryption_mode,status FROM chat.attachments WHERE id=$1 AND status='ready'",
		id).Scan(&ownerID, &objKey, &mime, &origName, &size, &checksum, &encMode, &status)
	if e != nil {
		fail(w, 404, "attachment not found")
		return
	}
	if ownerID != claims(r).Subject {
		var conversationID string
		if err := a.db.QueryRowContext(r.Context(), "SELECT m.conversation_id FROM chat.messages m JOIN chat.attachments a ON a.message_id=m.id WHERE a.id=$1 AND m.deleted_at IS NULL", id).Scan(&conversationID); err != nil || !a.isMember(r, conversationID) {
			fail(w, 403, "attachment forbidden")
			return
		}
	}
	send(w, 200, map[string]any{
		"id":              id,
		"owner_id":        ownerID,
		"object_key":      objKey,
		"mime_type":       mime,
		"original_name":   origName,
		"size_bytes":      size,
		"checksum_sha256": checksum,
		"encryption_mode": encMode,
		"status":          status,
	})
}

// startCall initiates a 1:1 or Group Voice/Video WebRTC call session (Phase 4: 055-060, E2E Scenario #10).
func (a *App) startCall(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ConversationID string `json:"conversation_id"`
		Kind           string `json:"kind"`
		SDPOffer       string `json:"sdp_offer"`
	}
	if decode(r, &in) != nil || !a.isMember(r, in.ConversationID) || (in.Kind != "voice" && in.Kind != "video") {
		fail(w, 400, "invalid call request")
		return
	}
	members := a.conversationMemberIDs(r, in.ConversationID)
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db error")
		return
	}
	defer tx.Rollback()
	var callID string
	e = tx.QueryRowContext(r.Context(),
		"INSERT INTO calling.calls(conversation_id,initiated_by,kind,state) VALUES ($1,$2,$3,'ringing') RETURNING id",
		in.ConversationID, claims(r).Subject, in.Kind).Scan(&callID)
	if e != nil {
		fail(w, 500, "call creation failed")
		return
	}
	for _, uid := range members {
		st := "invited"
		if uid == claims(r).Subject {
			st = "joined"
		}
		_, _ = tx.ExecContext(r.Context(),
			"INSERT INTO calling.participants(call_id,user_id,state,joined_at) VALUES ($1,$2,$3,now())",
			callID, uid, st)
	}
	if tx.Commit() != nil {
		fail(w, 500, "call commit failed")
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"type":            "call.ringing",
		"call_id":         callID,
		"conversation_id": in.ConversationID,
		"initiated_by":    claims(r).Subject,
		"kind":            in.Kind,
		"sdp_offer":       in.SDPOffer,
	})
	a.hub.Notify(members, payload)
	send(w, 201, map[string]any{
		"id":              callID,
		"conversation_id": in.ConversationID,
		"kind":            in.Kind,
		"state":           "ringing",
	})
}

func (a *App) callSignal(w http.ResponseWriter, r *http.Request) {
	callID := r.PathValue("id")
	if !auth.ValidUUID(callID) {
		fail(w, 400, "invalid call id")
		return
	}
	var convID string
	if e := a.db.QueryRowContext(r.Context(), "SELECT conversation_id FROM calling.calls WHERE id=$1", callID).Scan(&convID); e != nil || !a.isMember(r, convID) {
		fail(w, 403, "call forbidden")
		return
	}
	var in struct {
		SignalType string `json:"signal_type"` // offer, answer, ice-candidate, screen-share
		SDP        string `json:"sdp"`
		Candidate  string `json:"candidate"`
	}
	if decode(r, &in) != nil || in.SignalType == "" {
		fail(w, 400, "invalid signal payload")
		return
	}
	// Persist signal for late joiners / reconnection catch-up.
	_, _ = a.db.ExecContext(r.Context(),
		"INSERT INTO calling.call_signals(call_id,from_user_id,signal_type,sdp,candidate) VALUES($1,$2,$3,$4,$5)",
		callID, claims(r).Subject, in.SignalType, in.SDP, in.Candidate)

	payload, _ := json.Marshal(map[string]any{
		"type":            "call.signal",
		"call_id":         callID,
		"conversation_id": convID,
		"from_user_id":    claims(r).Subject,
		"signal_type":     in.SignalType,
		"sdp":             in.SDP,
		"candidate":       in.Candidate,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), payload)
	send(w, 200, map[string]string{"status": "signaled", "call_id": callID})
}

func (a *App) updateCallState(w http.ResponseWriter, r *http.Request) {
	callID := r.PathValue("id")
	if !auth.ValidUUID(callID) {
		fail(w, 400, "invalid call id")
		return
	}
	var convID string
	if e := a.db.QueryRowContext(r.Context(), "SELECT conversation_id FROM calling.calls WHERE id=$1", callID).Scan(&convID); e != nil || !a.isMember(r, convID) {
		fail(w, 403, "call forbidden")
		return
	}
	var in struct {
		State     string `json:"state"` // connected, ended
		EndReason string `json:"end_reason"`
	}
	if decode(r, &in) != nil || (in.State != "connected" && in.State != "ended") {
		fail(w, 400, "invalid call state")
		return
	}
	if in.State == "connected" {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE calling.calls SET state='connected',connected_at=now() WHERE id=$1", callID)
		_, _ = a.db.ExecContext(r.Context(), "UPDATE calling.participants SET state='joined',joined_at=now() WHERE call_id=$1 AND user_id=$2", callID, claims(r).Subject)
	} else {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE calling.calls SET state='ended',ended_at=now(),end_reason=$2 WHERE id=$1", callID, in.EndReason)
		_, _ = a.db.ExecContext(r.Context(), "UPDATE calling.participants SET state='left',left_at=now() WHERE call_id=$1 AND user_id=$2", callID, claims(r).Subject)
	}
	payload, _ := json.Marshal(map[string]any{
		"type":            "call.state",
		"call_id":         callID,
		"conversation_id": convID,
		"user_id":         claims(r).Subject,
		"state":           in.State,
		"end_reason":      in.EndReason,
	})
	a.hub.Notify(a.conversationMemberIDs(r, convID), payload)
	send(w, 200, map[string]string{"status": in.State, "call_id": callID})
}

func (a *App) recordCallQuality(w http.ResponseWriter, r *http.Request) {
	callID := r.PathValue("id")
	if !auth.ValidUUID(callID) {
		fail(w, 400, "invalid call id")
		return
	}
	var conversationID string
	if err := a.db.QueryRowContext(r.Context(), "SELECT conversation_id FROM calling.calls WHERE id=$1", callID).Scan(&conversationID); err != nil || !a.isMember(r, conversationID) {
		fail(w, 403, "call forbidden")
		return
	}
	var in struct {
		RTTMs           int     `json:"rtt_ms"`
		JitterMs        int     `json:"jitter_ms"`
		PacketLossRatio float64 `json:"packet_loss_ratio"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "invalid quality metrics")
		return
	}
	_, e := a.db.ExecContext(r.Context(),
		"INSERT INTO calling.quality_samples(call_id,user_id,rtt_ms,jitter_ms,packet_loss_ratio) VALUES ($1,$2,$3,$4,$5)",
		callID, claims(r).Subject, in.RTTMs, in.JitterMs, in.PacketLossRatio)
	if e != nil {
		fail(w, 500, "quality recording failed")
		return
	}
	send(w, 201, map[string]string{"status": "recorded"})
}

func (a *App) listCalls(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		`SELECT c.id,c.conversation_id,c.initiated_by,c.kind,c.state,c.started_at
		 FROM calling.calls c JOIN calling.participants p ON p.call_id=c.id
		 WHERE p.user_id=$1 ORDER BY c.started_at DESC LIMIT 50`, claims(r).Subject)
	if e != nil {
		fail(w, 500, "calls unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, cid, by, kind, state string
		var started any
		if rows.Scan(&id, &cid, &by, &kind, &state, &started) == nil {
			out = append(out, map[string]any{"id": id, "conversation_id": cid, "initiated_by": by, "kind": kind, "state": state, "started_at": started})
		}
	}
	send(w, 200, map[string]any{"calls": out})
}

func (a *App) heartbeatPresence(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceID string `json:"device_id"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.DeviceID) {
		fail(w, 400, "invalid device_id")
		return
	}
	if !a.ownsActiveDevice(r, in.DeviceID) {
		fail(w, 403, "device forbidden")
		return
	}
	_, e := a.db.ExecContext(r.Context(),
		`INSERT INTO realtime.presence(user_id,device_id,last_heartbeat_at) VALUES ($1,$2,now())
		 ON CONFLICT(user_id,device_id) DO UPDATE SET last_heartbeat_at=now()`,
		claims(r).Subject, in.DeviceID)
	if e != nil {
		fail(w, 500, "presence update failed")
		return
	}
	send(w, 200, map[string]string{"status": "online"})
}

func (a *App) getPresence(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if !auth.ValidUUID(uid) {
		fail(w, 400, "invalid user_id")
		return
	}
	var lastSeen any
	e := a.db.QueryRowContext(r.Context(), "SELECT max(last_heartbeat_at) FROM realtime.presence WHERE user_id=$1", uid).Scan(&lastSeen)
	online := e == nil && lastSeen != nil
	send(w, 200, map[string]any{"user_id": uid, "online": online, "last_heartbeat_at": lastSeen})
}

func (a *App) registerPushToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceID string `json:"device_id"`
		Provider string `json:"provider"` // fcm, apns, webpush
		Token    string `json:"token"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.DeviceID) || len(in.Provider) < 2 || len(in.Token) < 8 {
		fail(w, 400, "invalid push token")
		return
	}
	if !a.ownsActiveDevice(r, in.DeviceID) {
		fail(w, 403, "device forbidden")
		return
	}
	if (in.Provider != "fcm" && in.Provider != "apns" && in.Provider != "webpush") || len(in.Token) > 8192 {
		fail(w, 400, "invalid provider or token")
		return
	}
	encrypted, err := sealPushToken(a.cfg.PushTokenKey, in.DeviceID, in.Provider, in.Token)
	if err != nil {
		fail(w, 503, "push token encryption not configured")
		return
	}
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO realtime.push_tokens(device_id,provider,token_ciphertext) VALUES ($1,$2,$3) RETURNING id",
		in.DeviceID, in.Provider, encrypted).Scan(&id)
	if e != nil {
		fail(w, 500, "push token save failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "provider": in.Provider})
}

func (a *App) ownsActiveDevice(r *http.Request, deviceID string) bool {
	var id string
	return a.db.QueryRowContext(r.Context(), "SELECT id FROM identity.devices WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL", deviceID, claims(r).Subject).Scan(&id) == nil
}

func (a *App) listStoreStickers(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,title,COALESCE(description,''),price_minor,currency,status FROM creator.sticker_packs ORDER BY created_at DESC LIMIT 100")
	if e != nil {
		fail(w, 500, "sticker store unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, title, desc, curr, st string
		var price int64
		if rows.Scan(&id, &title, &desc, &price, &curr, &st) == nil {
			out = append(out, map[string]any{"id": id, "title": title, "description": desc, "price_minor": price, "currency": curr, "status": st})
		}
	}
	send(w, 200, map[string]any{"packs": out})
}

func (a *App) createOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductType    string `json:"product_type"`
		ResourceID     string `json:"resource_id"`
		Name           string `json:"name"`
		PriceMinor     int64  `json:"price_minor"`
		Currency       string `json:"currency"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if decode(r, &in) != nil || len(in.IdempotencyKey) < 8 || in.PriceMinor < 0 {
		fail(w, 400, "invalid order")
		return
	}
	if in.Currency == "" {
		in.Currency = "KRW"
	}
	if in.ProductType == "" {
		in.ProductType = "sticker_pack"
	}
	if in.Name == "" {
		in.Name = "Digital Item"
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "db error")
		return
	}
	defer tx.Rollback()

	var existingID string
	if tx.QueryRowContext(r.Context(), "SELECT id FROM commerce.orders WHERE idempotency_key=$1", in.IdempotencyKey).Scan(&existingID) == nil {
		send(w, 200, map[string]any{"order_id": existingID, "status": "completed", "idempotent": true})
		return
	}
	var prodID string
	var resArg any
	if auth.ValidUUID(in.ResourceID) {
		resArg = in.ResourceID
	}
	if e := tx.QueryRowContext(r.Context(),
		"INSERT INTO commerce.products(type,resource_id,name,currency,price_minor,active) VALUES ($1,$2,$3,$4,$5,true) RETURNING id",
		in.ProductType, resArg, in.Name, in.Currency, in.PriceMinor).Scan(&prodID); e != nil {
		fail(w, 500, "product creation failed")
		return
	}
	var orderID string
	if e := tx.QueryRowContext(r.Context(),
		"INSERT INTO commerce.orders(buyer_id,state,currency,total_minor,idempotency_key) VALUES ($1,'completed',$2,$3,$4) RETURNING id",
		claims(r).Subject, in.Currency, in.PriceMinor, in.IdempotencyKey).Scan(&orderID); e != nil {
		fail(w, 500, "order creation failed")
		return
	}
	_, _ = tx.ExecContext(r.Context(),
		"INSERT INTO commerce.order_items(order_id,product_id,quantity,unit_price_minor) VALUES ($1,$2,1,$3)",
		orderID, prodID, in.PriceMinor)
	var payID string
	_ = tx.QueryRowContext(r.Context(),
		"INSERT INTO commerce.payments(order_id,provider,provider_transaction_id,state,amount_minor,currency) VALUES ($1,'internal_ledger',$2,'succeeded',$3,$4) RETURNING id",
		orderID, "tx-"+in.IdempotencyKey, in.PriceMinor, in.Currency).Scan(&payID)
	_, _ = tx.ExecContext(r.Context(),
		"INSERT INTO commerce.ledger_entries(event_id,account_code,direction,amount_minor,currency) VALUES ($1,'CASH_CLEARING','debit',$2,$3),($1,'PLATFORM_REVENUE','credit',$2,$3)",
		orderID, in.PriceMinor, in.Currency)
	if tx.Commit() != nil {
		fail(w, 500, "order commit failed")
		return
	}
	send(w, 201, map[string]any{"order_id": orderID, "payment_id": payID, "status": "completed", "total_minor": in.PriceMinor, "currency": in.Currency})
}

func (a *App) listOrders(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,state,currency,total_minor,created_at FROM commerce.orders WHERE buyer_id=$1 ORDER BY created_at DESC",
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "orders unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, st, curr string
		var total int64
		var at any
		if rows.Scan(&id, &st, &curr, &total, &at) == nil {
			out = append(out, map[string]any{"id": id, "state": st, "currency": curr, "total_minor": total, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"orders": out})
}

func (a *App) listCommunityRooms(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,owner_id,name,COALESCE(description,''),visibility,created_at FROM community.rooms ORDER BY created_at DESC LIMIT 100")
	if e != nil {
		fail(w, 500, "community rooms unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, owner, name, desc, vis string
		var at any
		if rows.Scan(&id, &owner, &name, &desc, &vis, &at) == nil {
			out = append(out, map[string]any{"id": id, "owner_id": owner, "name": name, "description": desc, "visibility": vis, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"rooms": out})
}

func (a *App) createCommunityRoom(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Nickname    string `json:"nickname"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 120 {
		fail(w, 400, "invalid community room")
		return
	}
	if in.Nickname == "" {
		in.Nickname = "Host"
	}
	var roomID string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO community.rooms(owner_id,name,description,visibility) VALUES ($1,$2,$3,'public') RETURNING id",
		claims(r).Subject, in.Name, in.Description).Scan(&roomID)
	if e != nil {
		fail(w, 500, "room creation failed")
		return
	}
	_, _ = a.db.ExecContext(r.Context(),
		"INSERT INTO community.members(room_id,user_id,nickname,role) VALUES ($1,$2,$3,'owner') ON CONFLICT DO NOTHING",
		roomID, claims(r).Subject, in.Nickname)
	send(w, 201, map[string]string{"id": roomID, "name": in.Name})
}

func (a *App) joinCommunityRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if !auth.ValidUUID(roomID) {
		fail(w, 400, "invalid room id")
		return
	}
	var in struct {
		Nickname string `json:"nickname"`
	}
	_ = decode(r, &in)
	if in.Nickname == "" {
		in.Nickname = "Member"
	}
	_, e := a.db.ExecContext(r.Context(),
		"INSERT INTO community.members(room_id,user_id,nickname,role) VALUES ($1,$2,$3,'member') ON CONFLICT DO NOTHING",
		roomID, claims(r).Subject, in.Nickname)
	if e != nil {
		fail(w, 500, "join room failed")
		return
	}
	send(w, 200, map[string]string{"status": "joined", "room_id": roomID})
}

func (a *App) createCommunityPost(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if !auth.ValidUUID(roomID) {
		fail(w, 400, "invalid room id")
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Body)) < 1 || len(in.Body) > 10000 {
		fail(w, 400, "invalid post body")
		return
	}
	var postID string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO community.posts(room_id,author_id,body) VALUES ($1,$2,$3) RETURNING id",
		roomID, claims(r).Subject, in.Body).Scan(&postID)
	if e != nil {
		fail(w, 500, "post creation failed")
		return
	}
	send(w, 201, map[string]string{"id": postID, "room_id": roomID, "body": in.Body})
}

func (a *App) listCommunityPosts(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if !auth.ValidUUID(roomID) {
		fail(w, 400, "invalid room id")
		return
	}
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,author_id,body,created_at FROM community.posts WHERE room_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 100",
		roomID)
	if e != nil {
		fail(w, 500, "posts unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, author, body string
		var at any
		if rows.Scan(&id, &author, &body, &at) == nil {
			out = append(out, map[string]any{"id": id, "author_id": author, "body": body, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"posts": out})
}
// makeThumbnail creates a small 64x64 PNG thumbnail for image content.
func makeThumbnail(content []byte) string {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return ""
	}
	src := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			sx := src.Min.X + x*src.Dx()/64
			sy := src.Min.Y + y*src.Dy()/64
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
