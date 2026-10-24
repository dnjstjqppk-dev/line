package server

import (
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
)

func (a *App) createDeveloperOrganization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 160 {
		fail(w, 400, "invalid organization name")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "database unavailable")
		return
	}
	defer tx.Rollback()
	var id string
	e = tx.QueryRowContext(r.Context(), "INSERT INTO developer.organizations(owner_id,name) VALUES($1,$2) RETURNING id", claims(r).Subject, in.Name).Scan(&id)
	if e != nil {
		fail(w, 500, "organization creation failed")
		return
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO developer.memberships(organization_id,user_id,role) VALUES($1,$2,'owner')", id, claims(r).Subject)
	if e != nil || tx.Commit() != nil {
		fail(w, 500, "membership creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "name": in.Name})
}
func (a *App) developerOrganizations(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT o.id,o.name,m.role FROM developer.organizations o JOIN developer.memberships m ON m.organization_id=o.id WHERE m.user_id=$1 ORDER BY o.created_at DESC", claims(r).Subject)
	if e != nil {
		fail(w, 500, "organizations unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, name, role string
		if rows.Scan(&id, &name, &role) == nil {
			out = append(out, map[string]string{"id": id, "name": name, "role": role})
		}
	}
	send(w, 200, map[string]any{"organizations": out})
}

// App secrets are shown once and only the SHA-256 hash is persisted. OAuth/token
// issuance is not implemented; this endpoint does not enable credential use yet.
func (a *App) createDeveloperApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid organization")
		return
	}
	var role string
	if e := a.db.QueryRowContext(r.Context(), "SELECT role FROM developer.memberships WHERE organization_id=$1 AND user_id=$2", id, claims(r).Subject).Scan(&role); e != nil || (role != "owner" && role != "admin") {
		fail(w, 403, "organization forbidden")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 140 {
		fail(w, 400, "invalid app name")
		return
	}
	clientID, e := randomSecret(24)
	if e != nil {
		fail(w, 500, "secret generation failed")
		return
	}
	secret, e := randomSecret(32)
	if e != nil {
		fail(w, 500, "secret generation failed")
		return
	}
	var appID string
	e = a.db.QueryRowContext(r.Context(), "INSERT INTO developer.apps(organization_id,name,client_id,client_secret_hash) VALUES($1,$2,$3,$4) RETURNING id", id, in.Name, clientID, sha256hex(secret)).Scan(&appID)
	if e != nil {
		fail(w, 500, "app creation failed")
		return
	}
	send(w, 201, map[string]string{"id": appID, "name": in.Name, "client_id": clientID, "client_secret_once": secret, "status": "development"})
}
func (a *App) listDeveloperApps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !auth.ValidUUID(id) {
		fail(w, 400, "invalid organization")
		return
	}
	var role string
	if a.db.QueryRowContext(r.Context(), "SELECT role FROM developer.memberships WHERE organization_id=$1 AND user_id=$2", id, claims(r).Subject).Scan(&role) != nil {
		fail(w, 403, "organization forbidden")
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,name,client_id,status FROM developer.apps WHERE organization_id=$1 ORDER BY created_at DESC", id)
	if e != nil {
		fail(w, 500, "apps unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var appID, name, client, state string
		if rows.Scan(&appID, &name, &client, &state) == nil {
			result = append(result, map[string]string{"id": appID, "name": name, "client_id": client, "status": state})
		}
	}
	send(w, 200, map[string]any{"apps": result})
}
