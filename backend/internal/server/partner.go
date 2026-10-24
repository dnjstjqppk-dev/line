package server

import (
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
)

func (a *App) createOrganization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string `json:"name"`
		LegalName string `json:"legal_name"`
		Country   string `json:"country"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 160 || len(in.LegalName) > 160 || len(in.Country) > 8 {
		fail(w, 400, "invalid organization")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "database unavailable")
		return
	}
	defer tx.Rollback()
	var id string
	e = tx.QueryRowContext(r.Context(), "INSERT INTO business.organizations(owner_id,name,legal_name,country) VALUES($1,$2,$3,$4) RETURNING id", claims(r).Subject, in.Name, in.LegalName, in.Country).Scan(&id)
	if e != nil {
		fail(w, 500, "organization creation failed")
		return
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO business.memberships(organization_id,user_id,role) VALUES($1,$2,'owner')", id, claims(r).Subject)
	if e != nil || tx.Commit() != nil {
		fail(w, 500, "organization membership failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "name": in.Name})
}
func (a *App) organizations(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT o.id,o.name,o.verification_state,m.role
 FROM business.organizations o JOIN business.memberships m ON m.organization_id=o.id
 WHERE m.user_id=$1 ORDER BY o.created_at DESC`, claims(r).Subject)
	if e != nil {
		fail(w, 500, "organizations unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, state, role string
		if rows.Scan(&id, &name, &state, &role) == nil {
			result = append(result, map[string]string{"id": id, "name": name, "state": state, "my_role": role})
		}
	}
	send(w, 200, map[string]any{"organizations": result})
}
func (a *App) hasOrgRole(r *http.Request, id string, allowed ...string) bool {
	if !auth.ValidUUID(id) {
		return false
	}
	var role string
	if e := a.db.QueryRowContext(r.Context(), "SELECT role FROM business.memberships WHERE organization_id=$1 AND user_id=$2", id, claims(r).Subject).Scan(&role); e != nil {
		return false
	}
	for _, v := range allowed {
		if role == v {
			return true
		}
	}
	return false
}
func (a *App) createOfficialAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.hasOrgRole(r, id, "owner", "admin") {
		fail(w, 403, "organization forbidden")
		return
	}
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 120 || len(in.Description) > 1500 {
		fail(w, 400, "invalid account")
		return
	}
	var accountID string
	e := a.db.QueryRowContext(r.Context(), "INSERT INTO business.official_accounts(organization_id,name,description) VALUES($1,$2,$3) RETURNING id", id, in.Name, in.Description).Scan(&accountID)
	if e != nil {
		fail(w, 500, "account creation failed")
		return
	}
	send(w, 201, map[string]string{"id": accountID, "status": "pending", "name": in.Name})
}
func (a *App) officialAccounts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.hasOrgRole(r, id, "owner", "admin", "agent", "analyst", "billing") {
		fail(w, 403, "organization forbidden")
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,name,status FROM business.official_accounts WHERE organization_id=$1 ORDER BY created_at DESC", id)
	if e != nil {
		fail(w, 500, "accounts unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var aid, name, state string
		if rows.Scan(&aid, &name, &state) == nil {
			result = append(result, map[string]string{"id": aid, "name": name, "status": state})
		}
	}
	send(w, 200, map[string]any{"accounts": result})
}
