package server

import (
	"net/http"
)

// pushHealth checks if push notification credentials are configured.
func (a *App) pushHealth(w http.ResponseWriter, r *http.Request) {
	fcmConfigured := a.cfg.FCMProjectID != "" && (a.cfg.FCMAccessToken != "" || a.cfg.FCMServiceAccountJSON != "")
	apnsConfigured := a.cfg.APNsKeyID != "" && a.cfg.APNsPrivateKey != "" && a.cfg.APNsTeamID != "" && a.cfg.APNsBundleID != ""

	status := "ok"
	if !fcmConfigured && !apnsConfigured {
		status = "no_credentials"
	}

	send(w, 200, map[string]any{
		"status": status,
		"fcm": map[string]any{
			"configured": fcmConfigured,
			"project_id": a.cfg.FCMProjectID,
		},
		"apns": map[string]any{
			"configured": apnsConfigured,
			"key_id":     a.cfg.APNsKeyID,
			"bundle_id":  a.cfg.APNsBundleID,
			"production": a.cfg.APNsProduction,
		},
	})
}