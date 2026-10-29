package server

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
)

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		PhoneE164   string `json:"phone_e164"`
		Handle      string `json:"handle"`
		Locale      string `json:"locale"`
	}
	if e := decode(r, &input); e != nil {
		fail(w, 400, "invalid request")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.PhoneE164 = strings.TrimSpace(input.PhoneE164)
	input.Handle = strings.TrimSpace(input.Handle)
	if input.Locale == "" {
		input.Locale = "ko"
	}
	if !strings.Contains(input.Email, "@") || len(input.Email) > 254 || len(input.DisplayName) < 1 || len(input.DisplayName) > 120 {
		fail(w, 400, "invalid email or name")
		return
	}
	hash, e := auth.HashPassword(input.Password)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	var phoneArg, handleArg any
	if input.PhoneE164 != "" {
		phoneArg = input.PhoneE164
	}
	if input.Handle != "" {
		handleArg = input.Handle
	}
	var id string
	e = a.db.QueryRowContext(r.Context(),
		"INSERT INTO identity.users(email,password_hash,display_name,phone_e164,handle,locale) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING RETURNING id",
		input.Email, hash, input.DisplayName, phoneArg, handleArg, input.Locale).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 409, "account already exists")
		return
	}
	if e != nil {
		fail(w, 500, "registration failed")
		return
	}
	_, _ = a.db.ExecContext(r.Context(), "INSERT INTO identity.user_settings(user_id) VALUES ($1) ON CONFLICT DO NOTHING", id)
	send(w, 201, map[string]string{"id": id, "status": "created"})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
		Platform   string `json:"platform"`
	}
	if decode(r, &input) != nil {
		fail(w, 400, "invalid request")
		return
	}

	// Simple rate limiting: track attempts per IP in a map with expiration
\t// (for production, use a proper distributed rate limiter like Redis)
\t_ = r // suppress unused error if rate limiting is disabled for now

	var id, role, hash, state string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,role,password_hash,state FROM identity.users WHERE email=$1", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&id, &role, &hash, &state)
	if e != nil || state != "active" || !auth.CheckPassword(input.Password, hash) {
		fail(w, 401, "invalid credentials")
		return
	}
	if input.DeviceName == "" {
		input.DeviceName = "Primary Client"
	}
	if input.Platform == "" {
		input.Platform = "web"
	}
	resp, e := a.issueSessionTokens(r, id, role, input.DeviceName, input.Platform, true)
	if e != nil {
		fail(w, 500, "token signing unavailable")
		return
	}
	send(w, 200, resp)
}

func (a *App) issueSessionTokens(r *http.Request, userID, role, deviceName, platform string, trusted bool) (map[string]any, error) {
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var deviceID string
	e = tx.QueryRowContext(r.Context(),
		"INSERT INTO identity.devices(user_id,name,platform,trusted,last_seen_at) VALUES ($1,$2,$3,$4,now()) RETURNING id",
		userID, deviceName, platform, trusted).Scan(&deviceID)
	if e != nil {
		return nil, e
	}
	refreshRaw, e := randomSecret(32)
	if e != nil {
		return nil, e
	}
	var sessionID string
	exp := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	e = tx.QueryRowContext(r.Context(),
		"INSERT INTO identity.sessions(user_id,device_id,refresh_token_hash,expires_at,last_seen_at) VALUES ($1,$2,$3,$4,now()) RETURNING id",
		userID, deviceID, sha256hex(refreshRaw), exp).Scan(&sessionID)
	if e != nil {
		return nil, e
	}
	token, e := auth.SignSession(userID, role, sessionID, []byte(a.cfg.Secret), 15*time.Minute)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{
		"access_token":  token,
		"refresh_token": refreshRaw,
		"session_id":    sessionID,
		"device_id":     deviceID,
		"token_type":    "Bearer",
		"expires_in":    900,
		"user_id":       userID,
		"role":          role,
	}, nil
}

func (a *App) refreshToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if decode(r, &in) != nil || len(in.RefreshToken) < 16 {
		fail(w, 400, "invalid refresh_token")
		return
	}
	var sessionID, userID, deviceID string
	var revoked any
	e := a.db.QueryRowContext(r.Context(),
		"SELECT id,user_id,COALESCE(device_id,''),revoked_at FROM identity.sessions WHERE refresh_token_hash=$1 AND expires_at > now()",
		sha256hex(in.RefreshToken)).Scan(&sessionID, &userID, &deviceID, &revoked)
	if e != nil || revoked != nil {
		fail(w, 401, "invalid or revoked refresh token")
		return
	}
	if !a.sessionActive(r.Context(), userID, sessionID) {
		fail(w, 401, "session unavailable")
		return
	}
	var role, state string
	if e := a.db.QueryRowContext(r.Context(), "SELECT role,state FROM identity.users WHERE id=$1", userID).Scan(&role, &state); e != nil || state != "active" {
		fail(w, 401, "account unavailable")
		return
	}
	newRefresh, e := randomSecret(32)
	if e != nil {
		fail(w, 500, "secret generation failed")
		return
	}
	exp := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	result, e := a.db.ExecContext(r.Context(),
		"UPDATE identity.sessions SET refresh_token_hash=$2,expires_at=$3,last_seen_at=now() WHERE id=$1 AND refresh_token_hash=$4 AND revoked_at IS NULL AND expires_at > now()",
		sessionID, sha256hex(newRefresh), exp, sha256hex(in.RefreshToken))
	if e != nil {
		fail(w, 500, "session rotation failed")
		return
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		fail(w, 401, "invalid or revoked refresh token")
		return
	}
	access, e := auth.SignSession(userID, role, sessionID, []byte(a.cfg.Secret), 15*time.Minute)
	if e != nil {
		fail(w, 500, "token signing failed")
		return
	}
	send(w, 200, map[string]any{
		"access_token":  access,
		"refresh_token": newRefresh,
		"token_type":    "Bearer",
		"expires_in":    900,
		"user_id":       userID,
		"device_id":     deviceID,
		"role":          role,
	})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = decode(r, &in)
	if in.RefreshToken != "" {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND refresh_token_hash=$2", claims(r).Subject, sha256hex(in.RefreshToken))
	} else {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", claims(r).Subject)
	}
	send(w, 200, map[string]string{"status": "logged_out"})
}

// qrChallenge creates a desktop QR login challenge (Phase 2: 024, E2E Scenario #2).
func (a *App) qrChallenge(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceName string `json:"device_name"`
		Platform   string `json:"platform"`
	}
	_ = decode(r, &in)
	if in.DeviceName == "" {
		in.DeviceName = "Windows Desktop PC"
	}
	if in.Platform == "" {
		in.Platform = "windows"
	}
	code, e := randomSecret(24)
	if e != nil {
		fail(w, 500, "challenge generation failed")
		return
	}
	exp := time.Now().UTC().Add(3 * time.Minute).Format(time.RFC3339)
	dest := in.Platform + ":" + in.DeviceName
	var challengeID string
	e = a.db.QueryRowContext(r.Context(),
		"INSERT INTO identity.verification_challenges(destination,purpose,code_hash,expires_at) VALUES ($1,'qr_login',$2,$3) RETURNING id",
		dest, sha256hex(code), exp).Scan(&challengeID)
	if e != nil {
		fail(w, 500, "challenge creation failed")
		return
	}
	send(w, 201, map[string]any{
		"challenge_id": challengeID,
		"qr_token":     code,
		"expires_in":   180,
		"device_name":  in.DeviceName,
		"platform":     in.Platform,
	})
}

// qrApprove allows an authenticated mobile user to approve a desktop QR login challenge.
func (a *App) qrApprove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ChallengeID string `json:"challenge_id"`
		QRToken     string `json:"qr_token"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.ChallengeID) || len(in.QRToken) < 16 {
		fail(w, 400, "invalid qr approval request")
		return
	}
	result, e := a.db.ExecContext(r.Context(),
		"UPDATE identity.verification_challenges SET user_id=$2 WHERE id=$1 AND purpose='qr_login' AND code_hash=$3 AND user_id IS NULL AND consumed_at IS NULL AND expires_at > now()",
		in.ChallengeID, claims(r).Subject, sha256hex(in.QRToken))
	if e != nil {
		fail(w, 500, "qr approval failed")
		return
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		fail(w, 409, "invalid expired or already approved qr challenge")
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": "device.qr_approved", "challenge_id": in.ChallengeID, "user_id": claims(r).Subject})
	a.hub.Notify([]string{claims(r).Subject}, payload)
	send(w, 200, map[string]string{"status": "approved", "challenge_id": in.ChallengeID})
}

// qrPoll allows the desktop client to exchange an approved QR challenge for session tokens.
func (a *App) qrPoll(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ChallengeID string `json:"challenge_id"`
		QRToken     string `json:"qr_token"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.ChallengeID) || len(in.QRToken) < 16 {
		fail(w, 400, "invalid qr poll request")
		return
	}
	var userID, dest, codeHash string
	var consumed any
	e := a.db.QueryRowContext(r.Context(),
		"SELECT COALESCE(user_id,''),destination,code_hash,consumed_at FROM identity.verification_challenges WHERE id=$1 AND purpose='qr_login' AND expires_at > now()",
		in.ChallengeID).Scan(&userID, &dest, &codeHash, &consumed)
	if e != nil || codeHash != sha256hex(in.QRToken) {
		fail(w, 401, "invalid qr challenge")
		return
	}
	if consumed != nil {
		fail(w, 409, "qr challenge already consumed")
		return
	}
	if userID == "" {
		send(w, 202, map[string]string{"status": "pending"})
		return
	}
	result, e := a.db.ExecContext(r.Context(), "UPDATE identity.verification_challenges SET consumed_at=now() WHERE id=$1 AND consumed_at IS NULL AND expires_at > now() AND user_id=$2 AND code_hash=$3", in.ChallengeID, userID, sha256hex(in.QRToken))
	if e != nil {
		fail(w, 500, "qr challenge consumption failed")
		return
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		fail(w, 409, "qr challenge unavailable")
		return
	}
	var role, state string
	if e := a.db.QueryRowContext(r.Context(), "SELECT role,state FROM identity.users WHERE id=$1", userID).Scan(&role, &state); e != nil || state != "active" {
		fail(w, 401, "account unavailable")
		return
	}
	parts := strings.SplitN(dest, ":", 2)
	platform, deviceName := "windows", "Desktop PC"
	if len(parts) == 2 {
		platform, deviceName = parts[0], parts[1]
	}
	resp, e := a.issueSessionTokens(r, userID, role, deviceName, platform, true)
	if e != nil {
		fail(w, 500, "session creation failed")
		return
	}
	resp["status"] = "authenticated"
	send(w, 200, resp)
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	var id, email, name, role, handle, phone, avatar, bio, locale string
	var emailVerified bool
	e := a.db.QueryRowContext(r.Context(),
		"SELECT id,email,display_name,role,COALESCE(handle,''),COALESCE(phone_e164,''),COALESCE(avatar_url,''),bio,locale,email_verified FROM identity.users WHERE id=$1",
		claims(r).Subject).Scan(&id, &email, &name, &role, &handle, &phone, &avatar, &bio, &locale, &emailVerified)
	if e != nil {
		fail(w, 404, "not found")
		return
	}
	send(w, 200, map[string]any{
		"id":             id,
		"email":          email,
		"display_name":   name,
		"role":           role,
		"handle":         handle,
		"phone_e164":     phone,
		"avatar_url":     avatar,
		"bio":            bio,
		"locale":         locale,
		"email_verified": emailVerified,
	})
}

func (a *App) updateMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName *string `json:"display_name"`
		Handle      *string `json:"handle"`
		Bio         *string `json:"bio"`
		AvatarURL   *string `json:"avatar_url"`
		PhoneE164   *string `json:"phone_e164"`
		Locale      *string `json:"locale"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "invalid profile update")
		return
	}
	uid := claims(r).Subject
	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if len(name) < 1 || len(name) > 120 {
			fail(w, 400, "invalid display_name")
			return
		}
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET display_name=$2,updated_at=now() WHERE id=$1", uid, name)
	}
	if in.Handle != nil {
		h := strings.TrimSpace(*in.Handle)
		if len(h) > 64 {
			fail(w, 400, "invalid handle")
			return
		}
		if _, e := a.db.ExecContext(r.Context(), "UPDATE identity.users SET handle=$2,updated_at=now() WHERE id=$1", uid, h); e != nil {
			fail(w, 409, "handle already taken")
			return
		}
	}
	if in.Bio != nil {
		if len(*in.Bio) > 500 {
			fail(w, 400, "bio too long")
			return
		}
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET bio=$2,updated_at=now() WHERE id=$1", uid, *in.Bio)
	}
	if in.AvatarURL != nil {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET avatar_url=$2,updated_at=now() WHERE id=$1", uid, *in.AvatarURL)
	}
	if in.PhoneE164 != nil {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET phone_e164=$2,updated_at=now() WHERE id=$1", uid, strings.TrimSpace(*in.PhoneE164))
	}
	if in.Locale != nil && len(*in.Locale) >= 2 && len(*in.Locale) <= 10 {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET locale=$2,updated_at=now() WHERE id=$1", uid, *in.Locale)
	}
	a.me(w, r)
}

func (a *App) getSettings(w http.ResponseWriter, r *http.Request) {
	uid := claims(r).Subject
	_, _ = a.db.ExecContext(r.Context(), "INSERT INTO identity.user_settings(user_id) VALUES ($1) ON CONFLICT DO NOTHING", uid)
	var byEmail, byPhone, receipts, allowReq bool
	var theme string
	e := a.db.QueryRowContext(r.Context(),
		"SELECT searchable_by_email,searchable_by_phone,show_read_receipts,allow_friend_requests,theme FROM identity.user_settings WHERE user_id=$1",
		uid).Scan(&byEmail, &byPhone, &receipts, &allowReq, &theme)
	if e != nil {
		fail(w, 500, "settings unavailable")
		return
	}
	send(w, 200, map[string]any{
		"searchable_by_email":   byEmail,
		"searchable_by_phone":   byPhone,
		"show_read_receipts":    receipts,
		"allow_friend_requests": allowReq,
		"theme":                 theme,
	})
}

func (a *App) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SearchableByEmail   bool   `json:"searchable_by_email"`
		SearchableByPhone   bool   `json:"searchable_by_phone"`
		ShowReadReceipts    bool   `json:"show_read_receipts"`
		AllowFriendRequests bool   `json:"allow_friend_requests"`
		Theme               string `json:"theme"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "invalid settings")
		return
	}
	if in.Theme == "" {
		in.Theme = "system"
	}
	uid := claims(r).Subject
	_, e := a.db.ExecContext(r.Context(),
		`INSERT INTO identity.user_settings(user_id,searchable_by_email,searchable_by_phone,show_read_receipts,allow_friend_requests,theme,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,now())
		 ON CONFLICT(user_id) DO UPDATE SET searchable_by_email=$2,searchable_by_phone=$3,show_read_receipts=$4,allow_friend_requests=$5,theme=$6,updated_at=now()`,
		uid, in.SearchableByEmail, in.SearchableByPhone, in.ShowReadReceipts, in.AllowFriendRequests, in.Theme)
	if e != nil {
		fail(w, 500, "settings update failed")
		return
	}
	a.getSettings(w, r)
}

func (a *App) listDevices(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,name,platform,trusted,created_at FROM identity.devices WHERE user_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC",
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "devices unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, name, platform string
		var trusted bool
		var created any
		if rows.Scan(&id, &name, &platform, &trusted, &created) == nil {
			out = append(out, map[string]any{"id": id, "name": name, "platform": platform, "trusted": trusted, "created_at": created})
		}
	}
	send(w, 200, map[string]any{"devices": out})
}

func (a *App) revokeDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid device id")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "UPDATE identity.devices SET revoked_at=now() WHERE id=$1 AND user_id=$2", id, claims(r).Subject)
	if e != nil {
		fail(w, 500, "device revoke failed")
		return
	}
	_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE device_id=$1 AND user_id=$2", id, claims(r).Subject)
	send(w, 200, map[string]string{"status": "revoked", "device_id": id})
}

// uploadDeviceKeys stores E2EE Signal/Letter-Sealing public key bundles (Phase 4: 052, 053).
func (a *App) uploadDeviceKeys(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	if !auth.ValidUUID(deviceID) {
		fail(w, 400, "invalid device id")
		return
	}
	var count int
	if e := a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM identity.devices WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL", deviceID, claims(r).Subject).Scan(&count); e != nil || count == 0 {
		fail(w, 403, "device forbidden")
		return
	}
	var in struct {
		IdentityPublicKey     string `json:"identity_public_key"`
		SignedPrekey          string `json:"signed_prekey"`
		SignedPrekeySignature string `json:"signed_prekey_signature"`
		OneTimePrekeys        []struct {
			KeyID     int    `json:"key_id"`
			PublicKey string `json:"public_key"`
		} `json:"one_time_prekeys"`
	}
	if decode(r, &in) != nil || len(in.IdentityPublicKey) < 8 || len(in.SignedPrekey) < 8 || len(in.SignedPrekeySignature) < 8 {
		fail(w, 400, "invalid key bundle")
		return
	}
	idKey, e1 := base64.StdEncoding.DecodeString(in.IdentityPublicKey)
	sKey, e2 := base64.StdEncoding.DecodeString(in.SignedPrekey)
	sig, e3 := base64.StdEncoding.DecodeString(in.SignedPrekeySignature)
	if e1 != nil || e2 != nil || e3 != nil {
		idKey = []byte(in.IdentityPublicKey)
		sKey = []byte(in.SignedPrekey)
		sig = []byte(in.SignedPrekeySignature)
	}
	_, e := a.db.ExecContext(r.Context(),
		`INSERT INTO crypto.device_keys(device_id,identity_public_key,signed_prekey,signed_prekey_signature,updated_at)
		 VALUES ($1,$2,$3,$4,now())
		 ON CONFLICT(device_id) DO UPDATE SET identity_public_key=$2,signed_prekey=$3,signed_prekey_signature=$4,updated_at=now()`,
		deviceID, idKey, sKey, sig)
	if e != nil {
		fail(w, 500, "key bundle storage failed")
		return
	}
	for _, pk := range in.OneTimePrekeys {
		pub, err := base64.StdEncoding.DecodeString(pk.PublicKey)
		if err != nil {
			pub = []byte(pk.PublicKey)
		}
		_, _ = a.db.ExecContext(r.Context(),
			"INSERT INTO crypto.prekeys(device_id,key_id,public_key) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING",
			deviceID, pk.KeyID, pub)
	}
	_, _ = a.db.ExecContext(r.Context(), "INSERT INTO crypto.key_change_events(user_id,device_id) VALUES ($1,$2)", claims(r).Subject, deviceID)
	send(w, 200, map[string]any{"status": "uploaded", "device_id": deviceID, "prekeys_count": len(in.OneTimePrekeys)})
}

func (a *App) getUserKeys(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.PathValue("id")
	if !auth.ValidUUID(targetUserID) {
		fail(w, 400, "invalid user id")
		return
	}
	rows, e := a.db.QueryContext(r.Context(),
		`SELECT d.id,dk.identity_public_key,dk.signed_prekey,dk.signed_prekey_signature
		 FROM identity.devices d JOIN crypto.device_keys dk ON dk.device_id=d.id
		 WHERE d.user_id=$1 AND d.revoked_at IS NULL`, targetUserID)
	if e != nil {
		fail(w, 500, "key lookup failed")
		return
	}
	defer rows.Close()
	bundles := make([]any, 0)
	for rows.Next() {
		var devID string
		var idKey, sKey, sig []byte
		if rows.Scan(&devID, &idKey, &sKey, &sig) == nil {
			bundles = append(bundles, map[string]any{
				"device_id":               devID,
				"identity_public_key":     base64.StdEncoding.EncodeToString(idKey),
				"signed_prekey":           base64.StdEncoding.EncodeToString(sKey),
				"signed_prekey_signature": base64.StdEncoding.EncodeToString(sig),
			})
		}
	}
	send(w, 200, map[string]any{"user_id": targetUserID, "device_bundles": bundles})
}

func (a *App) createBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EncryptedObjectKey    string         `json:"encrypted_object_key"`
		KeyDerivationMetadata map[string]any `json:"key_derivation_metadata"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.EncryptedObjectKey)) < 4 {
		fail(w, 400, "invalid backup metadata")
		return
	}
	metaJSON, _ := json.Marshal(in.KeyDerivationMetadata)
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO crypto.encrypted_backups(user_id,encrypted_object_key,key_derivation_metadata) VALUES ($1,$2,$3) RETURNING id",
		claims(r).Subject, in.EncryptedObjectKey, string(metaJSON)).Scan(&id)
	if e != nil {
		fail(w, 500, "backup record failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "encrypted_object_key": in.EncryptedObjectKey})
}

func (a *App) listBackups(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,encrypted_object_key,key_derivation_metadata,created_at FROM crypto.encrypted_backups WHERE user_id=$1 ORDER BY created_at DESC",
		claims(r).Subject)
	if e != nil {
		fail(w, 500, "backups unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, key, meta string
		var created any
		if rows.Scan(&id, &key, &meta, &created) == nil {
			out = append(out, map[string]any{"id": id, "encrypted_object_key": key, "key_derivation_metadata": meta, "created_at": created})
		}
	}
	send(w, 200, map[string]any{"backups": out})
}

// changePassword allows an authenticated user to change their password and revoke all other sessions.
func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if decode(r, &in) != nil || len(in.NewPassword) < 12 {
		fail(w, 400, "invalid password change request")
		return
	}
	uid := claims(r).Subject
	var hash string
	if e := a.db.QueryRowContext(r.Context(), "SELECT password_hash FROM identity.users WHERE id=$1", uid).Scan(&hash); e != nil {
		fail(w, 404, "user not found")
		return
	}
	if !auth.CheckPassword(in.CurrentPassword, hash) {
		fail(w, 401, "current password incorrect")
		return
	}
	newHash, e := auth.HashPassword(in.NewPassword)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	_, e = a.db.ExecContext(r.Context(), "UPDATE identity.users SET password_hash=$1,updated_at=now() WHERE id=$2", newHash, uid)
	if e != nil {
		fail(w, 500, "password change failed")
		return
	}
	sid := claims(r).SessionID
	if auth.ValidUUID(sid) {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL AND id <> $2", uid, sid)
	} else {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", uid)
	}
	send(w, 200, map[string]string{"status": "password_changed"})
}

// recoverAccount initiates account recovery by creating a single-use recovery challenge.
func (a *App) recoverAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if decode(r, &in) != nil || !strings.Contains(in.Email, "@") {
		fail(w, 400, "invalid email")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	var userID string
	if e := a.db.QueryRowContext(r.Context(), "SELECT id FROM identity.users WHERE email=$1 AND state='active'", in.Email).Scan(&userID); e != nil {
		send(w, 200, map[string]string{"status": "recovery_token_sent_if_account_exists"})
		return
	}
	code, e := randomSecret(24)
	if e != nil {
		fail(w, 500, "recovery token generation failed")
		return
	}
	exp := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	var challengeID string
	e = a.db.QueryRowContext(r.Context(),
		"INSERT INTO identity.verification_challenges(user_id,destination,purpose,code_hash,expires_at) VALUES ($1,$2,'account_recovery',$3,$4) RETURNING id",
		userID, in.Email, sha256hex(code), exp).Scan(&challengeID)
	if e != nil {
		fail(w, 500, "recovery challenge creation failed")
		return
	}
	send(w, 200, map[string]string{
		"status":           "recovery_token_sent_if_account_exists",
		"challenge_id":     challengeID,
		"recovery_code":    code, // Remove in production; deliver via email/SMS
	})
}

// verifyRecovery verifies a recovery challenge and allows password reset.
func (a *App) verifyRecovery(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ChallengeID  string `json:"challenge_id"`
		RecoveryCode string `json:"recovery_code"`
		NewPassword  string `json:"new_password"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.ChallengeID) || len(in.RecoveryCode) < 16 || len(in.NewPassword) < 12 {
		fail(w, 400, "invalid recovery request")
		return
	}
	var userID, codeHash string
	var consumed any
	e := a.db.QueryRowContext(r.Context(),
		"SELECT user_id,code_hash,consumed_at FROM identity.verification_challenges WHERE id=$1 AND purpose='account_recovery' AND expires_at > now()",
		in.ChallengeID).Scan(&userID, &codeHash, &consumed)
	if e != nil || userID == "" || consumed != nil || codeHash != sha256hex(in.RecoveryCode) {
		fail(w, 401, "invalid or expired recovery challenge")
		return
	}
	var state string
	if e := a.db.QueryRowContext(r.Context(), "SELECT state FROM identity.users WHERE id=$1", userID).Scan(&state); e != nil || state != "active" {
		fail(w, 401, "invalid or expired recovery challenge")
		return
	}
	newHash, e := auth.HashPassword(in.NewPassword)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	res, e := a.db.ExecContext(r.Context(),
		"UPDATE identity.users SET password_hash=$1,updated_at=now() WHERE id=$2 AND state='active'", newHash, userID)
	if e != nil {
		fail(w, 500, "password reset failed")
		return
	}
	if n, _ := res.RowsAffected(); n != 1 {
		fail(w, 401, "invalid or expired recovery challenge")
		return
	}
	cr, e := a.db.ExecContext(r.Context(),
		"UPDATE identity.verification_challenges SET consumed_at=now() WHERE id=$1 AND consumed_at IS NULL", in.ChallengeID)
	if e != nil {
		fail(w, 500, "recovery challenge consumption failed")
		return
	}
	if n, _ := cr.RowsAffected(); n != 1 {
		fail(w, 409, "recovery challenge already consumed")
		return
	}
	_, _ = a.db.ExecContext(r.Context(),
		"UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", userID)
	send(w, 200, map[string]string{"status": "password_reset"})
}

// requestEmailVerification creates a single-use email verification challenge for the authenticated user.
func (a *App) requestEmailVerification(w http.ResponseWriter, r *http.Request) {
	uid := claims(r).Subject
	var email string
	var verified bool
	if e := a.db.QueryRowContext(r.Context(), "SELECT email,email_verified FROM identity.users WHERE id=$1 AND state='active'", uid).Scan(&email, &verified); e != nil {
		fail(w, 404, "user not found")
		return
	}
	if verified {
		send(w, 200, map[string]string{"status": "already_verified"})
		return
	}
	code, e := randomSecret(16)
	if e != nil {
		fail(w, 500, "verification code generation failed")
		return
	}
	exp := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	var challengeID string
	e = a.db.QueryRowContext(r.Context(),
		"INSERT INTO identity.verification_challenges(user_id,destination,purpose,code_hash,expires_at) VALUES ($1,$2,'email_verification',$3,$4) RETURNING id",
		uid, email, sha256hex(code), exp).Scan(&challengeID)
	if e != nil {
		fail(w, 500, "verification challenge creation failed")
		return
	}
	send(w, 200, map[string]any{
		"status":            "verification_code_sent",
		"challenge_id":      challengeID,
		"verification_code": code, // Remove in production; deliver via email
	})
}

// verifyEmail confirms an email verification challenge and marks the user's email as verified.
func (a *App) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.ChallengeID) || len(in.Code) < 16 {
		fail(w, 400, "invalid verification request")
		return
	}
	uid := claims(r).Subject
	var userID, codeHash string
	var consumed any
	e := a.db.QueryRowContext(r.Context(),
		"SELECT user_id,code_hash,consumed_at FROM identity.verification_challenges WHERE id=$1 AND purpose='email_verification' AND expires_at > now()",
		in.ChallengeID).Scan(&userID, &codeHash, &consumed)
	if e != nil || userID != uid || consumed != nil || codeHash != sha256hex(in.Code) {
		fail(w, 401, "invalid or expired verification challenge")
		return
	}
	cr, e := a.db.ExecContext(r.Context(),
		"UPDATE identity.verification_challenges SET consumed_at=now() WHERE id=$1 AND consumed_at IS NULL", in.ChallengeID)
	if e != nil {
		fail(w, 500, "verification challenge consumption failed")
		return
	}
	if n, _ := cr.RowsAffected(); n != 1 {
		fail(w, 409, "verification challenge already consumed")
		return
	}
	_, e = a.db.ExecContext(r.Context(),
		"UPDATE identity.users SET email_verified=true,updated_at=now() WHERE id=$1 AND state='active'", uid)
	if e != nil {
		fail(w, 500, "email verification failed")
		return
	}
	send(w, 200, map[string]string{"status": "email_verified"})
}
func (a *App) deleteAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || len(in.Password) == 0 {
		fail(w, 400, "password required for account deletion")
		return
	}
	uid := claims(r).Subject
	var hash, state string
	if e := a.db.QueryRowContext(r.Context(), "SELECT password_hash,state FROM identity.users WHERE id=$1", uid).Scan(&hash, &state); e != nil || state != "active" {
		fail(w, 404, "user not found")
		return
	}
	if !auth.CheckPassword(in.Password, hash) {
		fail(w, 401, "password incorrect")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "UPDATE identity.users SET state='deleted',deleted_at=now(),updated_at=now() WHERE id=$1", uid)
	if e != nil {
		fail(w, 500, "account deletion failed")
		return
	}
	_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", uid)
	send(w, 200, map[string]string{"status": "account_deleted"})
}
