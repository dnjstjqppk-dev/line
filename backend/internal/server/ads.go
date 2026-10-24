package server

import (
	"database/sql"
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
)

func (a *App) advertisers(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT a.id,a.name,a.status,a.organization_id FROM ads.advertisers a
 JOIN business.memberships m ON m.organization_id=a.organization_id WHERE m.user_id=$1 ORDER BY a.name`, claims(r).Subject)
	if e != nil {
		fail(w, 500, "advertisers unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, status, org string
		if rows.Scan(&id, &name, &status, &org) == nil {
			result = append(result, map[string]string{"id": id, "name": name, "status": status, "organization_id": org})
		}
	}
	send(w, 200, map[string]any{"advertisers": result})
}
func (a *App) createAdvertiser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OrganizationID string `json:"organization_id"`
		Name           string `json:"name"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.OrganizationID) || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 120 {
		fail(w, 400, "invalid advertiser")
		return
	}
	if !a.hasOrgRole(r, in.OrganizationID, "owner", "admin") {
		fail(w, 403, "organization forbidden")
		return
	}
	var id string
	e := a.db.QueryRowContext(r.Context(), "INSERT INTO ads.advertisers(organization_id,name) VALUES($1,$2) RETURNING id", in.OrganizationID, in.Name).Scan(&id)
	if e != nil {
		fail(w, 500, "advertiser creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "name": in.Name, "status": "pending"})
}
func (a *App) adsCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT c.id,c.name,c.status,c.daily_budget_minor,c.currency FROM ads.campaigns c
 JOIN ads.advertisers a ON a.id=c.advertiser_id JOIN business.memberships m ON m.organization_id=a.organization_id
 WHERE m.user_id=$1 ORDER BY c.name`, claims(r).Subject)
	if e != nil {
		fail(w, 500, "campaigns unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, name, status, currency string
		var budget int64
		if rows.Scan(&id, &name, &status, &budget, &currency) == nil {
			result = append(result, map[string]any{"id": id, "name": name, "status": status, "daily_budget_minor": budget, "currency": currency})
		}
	}
	send(w, 200, map[string]any{"campaigns": result})
}
func (a *App) createAdsCampaign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AdvertiserID     string `json:"advertiser_id"`
		Name             string `json:"name"`
		Objective        string `json:"objective"`
		DailyBudgetMinor int64  `json:"daily_budget_minor"`
		Currency         string `json:"currency"`
	}
	if decode(r, &in) != nil || !auth.ValidUUID(in.AdvertiserID) || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 160 || len(in.Objective) < 2 || len(in.Objective) > 100 || in.DailyBudgetMinor < 0 {
		fail(w, 400, "invalid campaign")
		return
	}
	if in.Currency == "" {
		in.Currency = "KRW"
	}
	if len(in.Currency) != 3 {
		fail(w, 400, "invalid currency")
		return
	}
	var role string
	e := a.db.QueryRowContext(r.Context(), `SELECT m.role FROM ads.advertisers a JOIN business.memberships m ON m.organization_id=a.organization_id
 WHERE a.id=$1 AND m.user_id=$2`, in.AdvertiserID, claims(r).Subject).Scan(&role)
	if e == sql.ErrNoRows || (role != "owner" && role != "admin") {
		fail(w, 403, "advertiser forbidden")
		return
	}
	if e != nil {
		fail(w, 500, "advertiser unavailable")
		return
	}
	var id string
	e = a.db.QueryRowContext(r.Context(), "INSERT INTO ads.campaigns(advertiser_id,name,objective,daily_budget_minor,currency) VALUES($1,$2,$3,$4,$5) RETURNING id", in.AdvertiserID, in.Name, in.Objective, in.DailyBudgetMinor, in.Currency).Scan(&id)
	if e != nil {
		fail(w, 500, "campaign creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "status": "draft"})
}
