package server

import (
	"net/http"

	"github.com/example/global-messenger/backend/internal/auth"
)

// pushTest sends a test push notification to verify credentials.
func (a *App) pushTest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceID string `json:"device_id"`
		Title    string `json:"title"`
		Body     string `json:"body"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.DeviceID) {
		fail(w, 400, "invalid device id")
		return
	}
	if in.Title == "" {
		in.Title = "Test Notification"
	}
	if in.Body == "" {
		in.Body = "This is a test push notification"
	}

	// Queue test push
	_, err := a.db.ExecContext(r.Context(),
		"INSERT INTO realtime.push_queue(user_id, device_id, title, body, payload) VALUES ($1, $2, $3, $4, $5)",
		claims(r).Subject, in.DeviceID, in.Title, in.Body, "{}")
	if err != nil {
		fail(w, 500, "failed to queue test push")
		return
	}

	send(w, 200, map[string]string{
		"status":  "test_push_queued",
		"message": "Test push notification has been queued for delivery",
	})
}