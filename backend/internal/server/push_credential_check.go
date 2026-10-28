package server

import (
	"net/http"
)

// pushCredentialCheck validates that push notification credentials are properly configured.
func (a *App) pushCredentialCheck(w http.ResponseWriter, r *http.Request) {
	fcmOk := a.cfg.FCMProjectID != "" && (a.cfg.FCMAccessToken != "" || a.cfg.FCMServiceAccountJSON != "")
	apnsOk := a.cfg.APNsKeyID != "" && a.cfg.APNsPrivateKey != "" && a.cfg.APNsTeamID != "" && a.cfg.APNsBundleID != ""

	status := "ok"
	if !fcmOk && !apnsOk {
		status = "no_credentials"
	} else if !fcmOk {
		status = "fcm_missing"
	} else if !apnsOk {
		status = "apns_missing"
	}

	send(w, 200, map[string]any{
		"status": status,
		"fcm": map[string]any{
			"configured": fcmOk,
			"project_id": a.cfg.FCMProjectID,
			"has_token":  a.cfg.FCMAccessToken != "",
			"has_sa":     a.cfg.FCMServiceAccountJSON != "",
		},
		"apns": map[string]any{
			"configured": apnsOk,
			"key_id":     a.cfg.APNsKeyID,
			"team_id":    a.cfg.APNsTeamID,
			"bundle_id":  a.cfg.APNsBundleID,
			"production": a.cfg.APNsProduction,
		},
	})
}