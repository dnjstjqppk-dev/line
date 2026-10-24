package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/example/global-messenger/backend/internal/auth"
)

func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]int64)
	queries := map[string]string{
		"users":         "SELECT count(*) FROM identity.users WHERE state='active'",
		"conversations": "SELECT count(*) FROM chat.conversations WHERE deleted_at IS NULL",
		"messages":      "SELECT count(*) FROM chat.messages WHERE deleted_at IS NULL",
		"organizations": "SELECT count(*) FROM business.organizations",
		"open_reports":  "SELECT count(*) FROM safety.reports WHERE state='open'",
		"calls":         "SELECT count(*) FROM calling.calls",
		"orders":        "SELECT count(*) FROM commerce.orders",
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

func (a *App) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	queries := map[string]string{
		"messenger_active_users_total":  "SELECT count(*) FROM identity.users WHERE state='active'",
		"messenger_conversations_total": "SELECT count(*) FROM chat.conversations WHERE deleted_at IS NULL",
		"messenger_messages_total":      "SELECT count(*) FROM chat.messages WHERE deleted_at IS NULL",
		"messenger_open_reports_total":  "SELECT count(*) FROM safety.reports WHERE state='open'",
	}
	for metric, q := range queries {
		var count int64
		_ = a.db.QueryRowContext(r.Context(), q).Scan(&count)
		_, _ = fmt.Fprintf(w, "%s %d\n", metric, count)
	}
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

// moderationAction enforces user sanctions or resolves reports and writes immutable audit logs (Phase 8: 092, 093).
func (a *App) moderationAction(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ReportID      string `json:"report_id"`
		SubjectUserID string `json:"subject_user_id"`
		Action        string `json:"action"` // warn, suspend, activate, dismiss
		Reason        string `json:"reason"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.SubjectUserID) || (in.Action != "warn" && in.Action != "suspend" && in.Action != "activate" && in.Action != "dismiss") {
		fail(w, 400, "invalid moderation action")
		return
	}
	var repArg any
	if in.ReportID != "" && auth.ValidUUID(in.ReportID) {
		repArg = in.ReportID
	}
	var actID string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO safety.moderation_actions(report_id,subject_user_id,actor_id,action,reason) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		repArg, in.SubjectUserID, claims(r).Subject, in.Action, in.Reason).Scan(&actID)
	if e != nil {
		fail(w, 500, "moderation action failed")
		return
	}
	if in.Action == "suspend" {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET state='suspended',updated_at=now() WHERE id=$1", in.SubjectUserID)
	} else if in.Action == "activate" {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE identity.users SET state='active',updated_at=now() WHERE id=$1", in.SubjectUserID)
	}
	if repArg != nil {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE safety.reports SET state='resolved' WHERE id=$1", repArg)
	}
	auditData, _ := json.Marshal(map[string]string{"action": in.Action, "reason": in.Reason})
	_, _ = a.db.ExecContext(r.Context(),
		"INSERT INTO ops.audit_logs(actor_user_id,action,target_type,target_id,data) VALUES ($1,$2,'user',$3,$4)",
		claims(r).Subject, "moderation."+in.Action, in.SubjectUserID, string(auditData))
	send(w, 201, map[string]string{"id": actID, "action": in.Action, "subject_user_id": in.SubjectUserID})
}

func (a *App) adminAuditLogs(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,COALESCE(actor_user_id,''),action,COALESCE(target_type,''),COALESCE(target_id,''),created_at FROM ops.audit_logs ORDER BY created_at DESC LIMIT 100")
	if e != nil {
		fail(w, 500, "audit logs unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id int64
		var actor, action, ttype, tid string
		var at any
		if rows.Scan(&id, &actor, &action, &ttype, &tid, &at) == nil {
			out = append(out, map[string]any{"id": id, "actor_user_id": actor, "action": action, "target_type": ttype, "target_id": tid, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"audit_logs": out})
}

func (a *App) upsertFeatureFlag(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key     string         `json:"key"`
		Enabled bool           `json:"enabled"`
		Config  map[string]any `json:"config"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Key)) < 2 {
		fail(w, 400, "invalid feature flag")
		return
	}
	cfgBytes, _ := json.Marshal(in.Config)
	_, e := a.db.ExecContext(r.Context(),
		`INSERT INTO ops.feature_flags(key,enabled,config,updated_at) VALUES ($1,$2,$3,now())
		 ON CONFLICT(key) DO UPDATE SET enabled=$2,config=$3,updated_at=now()`,
		in.Key, in.Enabled, string(cfgBytes))
	if e != nil {
		fail(w, 500, "feature flag update failed")
		return
	}
	send(w, 200, map[string]any{"key": in.Key, "enabled": in.Enabled})
}

func (a *App) listFeatureFlags(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT key,enabled,config,updated_at FROM ops.feature_flags ORDER BY key")
	if e != nil {
		fail(w, 500, "feature flags unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var k, cfg string
		var en bool
		var at any
		if rows.Scan(&k, &en, &cfg, &at) == nil {
			out = append(out, map[string]any{"key": k, "enabled": en, "config": cfg, "updated_at": at})
		}
	}
	send(w, 200, map[string]any{"feature_flags": out})
}

func (a *App) createPrivacyRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"` // export, delete, rectify
	}
	if decode(r, &in) != nil || (in.Kind != "export" && in.Kind != "delete" && in.Kind != "rectify") {
		fail(w, 400, "invalid privacy request kind")
		return
	}
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO ops.privacy_requests(user_id,kind,status) VALUES ($1,$2,'requested') RETURNING id",
		claims(r).Subject, in.Kind).Scan(&id)
	if e != nil {
		fail(w, 500, "privacy request failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "kind": in.Kind, "status": "requested"})
}
