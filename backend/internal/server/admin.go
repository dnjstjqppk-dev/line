package server

import (
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
)

func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]int64)
	queries := map[string]string{
		"users":         "SELECT count(*) FROM identity.users WHERE state='active'",
		"conversations": "SELECT count(*) FROM chat.conversations WHERE deleted_at IS NULL",
		"messages":      "SELECT count(*) FROM chat.messages WHERE deleted_at IS NULL",
		"organizations": "SELECT count(*) FROM business.organizations",
		"open_reports":  "SELECT count(*) FROM safety.reports WHERE state='open'",
	}
	for k, q := range queries {
		var count int64
		if e := a.db.QueryRowContext(r.Context(), q).Scan(&count); e != nil {
			fail(w, 500, "metrics unavailable")
			return
		}
		result[k] = count
	}
	send(w, 200, map[string]any{"metrics": result})
}
func (a *App) adminUsers(w http.ResponseWriter, r *http.Request) {
	limit := auth.ParsePositiveInt(r.URL.Query().Get("limit"), 50, 100)
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,email,display_name,role,state,created_at FROM identity.users ORDER BY created_at DESC LIMIT $1", limit)
	if e != nil {
		fail(w, 500, "users unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, email, name, role, state string
		var created any
		if rows.Scan(&id, &email, &name, &role, &state, &created) == nil {
			result = append(result, map[string]any{"id": id, "email": email, "display_name": name, "role": role, "state": state, "created_at": created})
		}
	}
	send(w, 200, map[string]any{"users": result})
}
func (a *App) report(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SubjectUserID string `json:"subject_user_id"`
		Category      string `json:"category"`
		Details       string `json:"details"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.SubjectUserID) || len(in.Category) < 3 || len(in.Category) > 80 || len(in.Details) > 2000 {
		fail(w, 400, "invalid report")
		return
	}
	var id string
	e := a.db.QueryRowContext(r.Context(), "INSERT INTO safety.reports(reporter_id,subject_user_id,category,details) VALUES($1,$2,$3,$4) RETURNING id", claims(r).Subject, in.SubjectUserID, strings.TrimSpace(in.Category), in.Details).Scan(&id)
	if e != nil {
		fail(w, 500, "report not created")
		return
	}
	send(w, 201, map[string]string{"id": id})
}
func (a *App) adminReports(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,category,COALESCE(details,''),state,created_at FROM safety.reports ORDER BY created_at DESC LIMIT 100")
	if e != nil {
		fail(w, 500, "reports unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, category, details, state string
		var at any
		if rows.Scan(&id, &category, &details, &state, &at) == nil {
			result = append(result, map[string]any{"id": id, "category": category, "details": details, "state": state, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"reports": result})
}
