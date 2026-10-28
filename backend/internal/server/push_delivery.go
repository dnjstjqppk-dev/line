package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"encoding/pem"
)

type fcmServiceAccount struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	TokenURI     string `json:"token_uri"`
}

type fcmConfig struct {
	ProjectID       string
	AccessToken     string
	ServiceAccount  *fcmServiceAccount
	Enabled         bool
}

func (a *App) fcmConfig() fcmConfig {
	cfg := fcmConfig{
		ProjectID: a.cfg.FCMProjectID,
		Enabled:   false,
	}
	if a.cfg.FCMServiceAccountJSON != "" {
		var sa fcmServiceAccount
		if err := json.Unmarshal([]byte(a.cfg.FCMServiceAccountJSON), &sa); err == nil {
			cfg.ServiceAccount = &sa
			cfg.ProjectID = sa.ProjectID
			cfg.Enabled = true
		}
	}
	if a.cfg.FCMAccessToken != "" {
		cfg.AccessToken = a.cfg.FCMAccessToken
		cfg.Enabled = true
	}
	return cfg
}

func (a *App) runPushDelivery(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.drainPushQueue(ctx)
		}
	}
}

func (a *App) drainPushQueue(ctx context.Context) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, user_id, device_id, title, body, payload FROM realtime.push_queue
		 WHERE delivered_at IS NULL AND failed_at IS NULL AND attempts < 5
		 ORDER BY id LIMIT 50`)
	if err != nil {
		return
	}
	type item struct {
		id       int64
		userID   string
		deviceID sql.NullString
		title    sql.NullString
		body     sql.NullString
		payload  string
	}
	var batch []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.userID, &it.deviceID, &it.title, &it.body, &it.payload); err == nil {
			batch = append(batch, it)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return
	}
	rows.Close()

	fcm := a.fcmConfig()

	for _, it := range batch {
		delivered := false
		tokenRows, err := a.db.QueryContext(ctx,
			`SELECT pt.token_ciphertext, pt.provider, d.id
			 FROM realtime.push_tokens pt
			 JOIN identity.devices d ON d.id = pt.device_id
			 WHERE d.user_id = $1 AND pt.revoked_at IS NULL AND d.revoked_at IS NULL`,
			it.userID)
		if err == nil {
			type pushToken struct {
				token    string
				provider string
				deviceID string
			}
			var tokens []pushToken
			for tokenRows.Next() {
				var tok string
				var prov, devID string
				if err := tokenRows.Scan(&tok, &prov, &devID); err == nil {
					tokens = append(tokens, pushToken{token: tok, provider: prov, deviceID: devID})
				}
			}
			if err := tokenRows.Err(); err != nil {
				tokenRows.Close()
			}
			tokenRows.Close()

			if len(tokens) == 0 {
				delivered = true
			}
			for _, pt := range tokens {
				if pt.provider == "fcm" {
					if fcm.Enabled {
						token := fcm.AccessToken
						if token == "" {
							token = "simulated"
						}
						ok := a.sendFCM(ctx, fcm.ProjectID, token, pt.token, it.title.String, it.body.String, it.payload)
						if ok {
							delivered = true
						}
					} else {
						delivered = true
					}
				} else if pt.provider == "apns" {
					if a.cfg.APNsKeyID != "" && a.cfg.APNsPrivateKey != "" {
						ok := a.sendAPNs(ctx, a.cfg, pt.token, it.title.String, it.body.String, it.payload)
						if ok {
							delivered = true
						}
					} else {
						delivered = true
					}
				} else {
					delivered = true
				}
			}
		} else {
			delivered = true
		}

		if delivered {
			_, _ = a.db.ExecContext(ctx,
				"UPDATE realtime.push_queue SET delivered_at=now(), attempts=attempts+1 WHERE id=$1", it.id)
		} else {
			_, _ = a.db.ExecContext(ctx,
				"UPDATE realtime.push_queue SET attempts=attempts+1, failed_at=CASE WHEN attempts+1>=5 THEN now() ELSE failed_at END WHERE id=$1", it.id)
		}
	}
}

func (a *App) sendFCM(ctx context.Context, projectID, accessToken, deviceToken, title, body, payload string) bool {
	if projectID == "" {
		return true // simulate success when not configured
	}
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)

	msg := map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]any{
				"title": title,
				"body":  body,
			},
			"data": map[string]any{
				"payload": payload,
			},
		},
	}
	bodyBytes, err := json.Marshal(msg)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
// sendAPNs sends a push notification via APNs HTTP/2 API.
func (a *App) sendAPNs(ctx context.Context, cfg Config, deviceToken, title, body, payload string) bool {
	if cfg.APNsKeyID == "" || cfg.APNsPrivateKey == "" || cfg.APNsTeamID == "" || cfg.APNsBundleID == "" {
		return true // simulate success when not configured
	}
	host := "api.push.apple.com"
	if !cfg.APNsProduction {
		host = "api.sandbox.push.apple.com"
	}
	url := fmt.Sprintf("https://%s/3/device/%s", host, deviceToken)

	apnsPayload := map[string]any{
		"aps": map[string]any{
			"alert": map[string]any{
				"title": title,
				"body":  body,
			},
			"sound": "default",
		},
		"payload": payload,
	}
	bodyBytes, err := json.Marshal(apnsPayload)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.Header.Set("apns-topic", cfg.APNsBundleID)
	// Generate and set JWT token
	jwt, err := generateAPNsJWT(cfg.APNsKeyID, cfg.APNsTeamID, cfg.APNsPrivateKey)
	if err == nil && jwt != "" {
		req.Header.Set("authorization", "bearer "+jwt)
	}

	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
// generateAPNsJWT creates an ES256 JWT for APNs authentication.
func generateAPNsJWT(keyID, teamID, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}
	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return "", fmt.Errorf("failed to parse private key: %v / %v", err, err2)
		}
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("not an EC private key")
		}
		privKey = ecKey
	}

	now := time.Now()
	header := map[string]string{"alg": "ES256", "kid": keyID}
	claims := map[string]any{"iss": teamID, "iat": now.Unix()}

	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := encodedHeader + "." + encodedClaims

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return "", err
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + encodedSig, nil
}