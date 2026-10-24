package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/global-messenger/backend/internal/auth"
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

func (a *App) hasAccountRole(r *http.Request, accountID string, allowed ...string) bool {
	if !auth.ValidUUID(accountID) {
		return false
	}
	var orgID string
	if e := a.db.QueryRowContext(r.Context(), "SELECT organization_id FROM business.official_accounts WHERE id=$1", accountID).Scan(&orgID); e != nil {
		return false
	}
	return a.hasOrgRole(r, orgID, allowed...)
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

func (a *App) addBusinessContact(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin", "agent") {
		fail(w, 403, "account forbidden")
		return
	}
	var in struct {
		UserID  string   `json:"user_id"`
		OptedIn bool     `json:"opted_in"`
		Tags    []string `json:"tags"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.UserID) {
		fail(w, 400, "invalid contact")
		return
	}
	tagsBytes, _ := json.Marshal(in.Tags)
	var id string
	e := a.db.QueryRowContext(r.Context(),
		`INSERT INTO business.contacts(official_account_id,user_id,opted_in,tags) VALUES ($1,$2,$3,$4)
		 ON CONFLICT(official_account_id,user_id) DO UPDATE SET opted_in=$3,tags=$4 RETURNING id`,
		accID, in.UserID, in.OptedIn, string(tagsBytes)).Scan(&id)
	if e != nil {
		fail(w, 500, "contact save failed")
		return
	}
	send(w, 201, map[string]any{"id": id, "user_id": in.UserID, "opted_in": in.OptedIn})
}

func (a *App) createSupportTicket(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin", "agent") {
		fail(w, 403, "account forbidden")
		return
	}
	var in struct {
		ContactID    string `json:"contact_id"`
		Priority     string `json:"priority"`
		InternalNote string `json:"internal_note"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.ContactID) {
		fail(w, 400, "invalid support ticket")
		return
	}
	if in.Priority == "" {
		in.Priority = "normal"
	}
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO business.support_tickets(official_account_id,contact_id,agent_id,priority,internal_note) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		accID, in.ContactID, claims(r).Subject, in.Priority, in.InternalNote).Scan(&id)
	if e != nil {
		fail(w, 500, "ticket creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "status": "open", "priority": in.Priority})
}

func (a *App) listSupportTickets(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin", "agent", "analyst") {
		fail(w, 403, "account forbidden")
		return
	}
	rows, e := a.db.QueryContext(r.Context(),
		"SELECT id,contact_id,COALESCE(agent_id,''),status,priority,COALESCE(internal_note,''),created_at FROM business.support_tickets WHERE official_account_id=$1 ORDER BY created_at DESC",
		accID)
	if e != nil {
		fail(w, 500, "tickets unavailable")
		return
	}
	defer rows.Close()
	out := make([]any, 0)
	for rows.Next() {
		var id, cid, aid, st, prio, note string
		var at any
		if rows.Scan(&id, &cid, &aid, &st, &prio, &note, &at) == nil {
			out = append(out, map[string]any{"id": id, "contact_id": cid, "agent_id": aid, "status": st, "priority": prio, "internal_note": note, "created_at": at})
		}
	}
	send(w, 200, map[string]any{"tickets": out})
}

func (a *App) createBusinessCampaign(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin") {
		fail(w, 403, "account forbidden")
		return
	}
	var in struct {
		Name            string         `json:"name"`
		MessageTemplate map[string]any `json:"message_template"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 {
		fail(w, 400, "invalid campaign")
		return
	}
	tplBytes, _ := json.Marshal(in.MessageTemplate)
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO business.campaigns(official_account_id,name,message_template,status) VALUES ($1,$2,$3,'scheduled') RETURNING id",
		accID, in.Name, string(tplBytes)).Scan(&id)
	if e != nil {
		fail(w, 500, "campaign creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "name": in.Name, "status": "scheduled"})
}

func (a *App) createRichMenu(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin") {
		fail(w, 403, "account forbidden")
		return
	}
	var in struct {
		Title      string         `json:"title"`
		Definition map[string]any `json:"definition"`
		Enabled    bool           `json:"enabled"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Title)) < 2 {
		fail(w, 400, "invalid rich menu")
		return
	}
	defBytes, _ := json.Marshal(in.Definition)
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO business.rich_menus(official_account_id,title,definition,enabled) VALUES ($1,$2,$3,$4) RETURNING id",
		accID, in.Title, string(defBytes), in.Enabled).Scan(&id)
	if e != nil {
		fail(w, 500, "rich menu creation failed")
		return
	}
	send(w, 201, map[string]any{"id": id, "title": in.Title, "enabled": in.Enabled})
}

func (a *App) createCoupon(w http.ResponseWriter, r *http.Request) {
	accID := r.PathValue("id")
	if !a.hasAccountRole(r, accID, "owner", "admin") {
		fail(w, 403, "account forbidden")
		return
	}
	var in struct {
		Code           string `json:"code"`
		MaxRedemptions int    `json:"max_redemptions"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Code)) < 3 {
		fail(w, 400, "invalid coupon")
		return
	}
	if in.MaxRedemptions <= 0 {
		in.MaxRedemptions = 100
	}
	var id string
	e := a.db.QueryRowContext(r.Context(),
		"INSERT INTO business.coupons(official_account_id,code,max_redemptions) VALUES ($1,$2,$3) RETURNING id",
		accID, strings.ToUpper(strings.TrimSpace(in.Code)), in.MaxRedemptions).Scan(&id)
	if e != nil {
		fail(w, 500, "coupon creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "code": strings.ToUpper(strings.TrimSpace(in.Code))})
}
