package server

import (
	"database/sql"
	"net/http"
	"strings"
)

func (a *App) createCreator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"display_name"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Name) > 120 {
		fail(w, 400, "invalid display name")
		return
	}
	var id string
	e := a.db.QueryRowContext(r.Context(), "INSERT INTO creator.creators(user_id,display_name) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET display_name=excluded.display_name RETURNING id", claims(r).Subject, in.Name).Scan(&id)
	if e != nil {
		fail(w, 500, "profile unavailable")
		return
	}
	send(w, 200, map[string]string{"id": id, "display_name": in.Name})
}
func (a *App) creatorPacks(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT p.id,p.title,p.status,p.price_minor,p.currency FROM creator.sticker_packs p JOIN creator.creators c ON c.id=p.creator_id WHERE c.user_id=$1 ORDER BY p.created_at DESC", claims(r).Subject)
	if e != nil {
		fail(w, 500, "packs unavailable")
		return
	}
	defer rows.Close()
	result := make([]any, 0)
	for rows.Next() {
		var id, title, status, currency string
		var amount int64
		if rows.Scan(&id, &title, &status, &amount, &currency) == nil {
			result = append(result, map[string]any{"id": id, "title": title, "status": status, "price_minor": amount, "currency": currency})
		}
	}
	send(w, 200, map[string]any{"packs": result})
}
func (a *App) createStickerPack(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title      string `json:"title"`
		PriceMinor int64  `json:"price_minor"`
		Currency   string `json:"currency"`
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Title)) < 2 || len(in.Title) > 160 || in.PriceMinor < 0 {
		fail(w, 400, "invalid pack")
		return
	}
	if in.Currency == "" {
		in.Currency = "KRW"
	}
	if len(in.Currency) != 3 {
		fail(w, 400, "currency must be ISO 4217 code")
		return
	}
	var creatorID string
	e := a.db.QueryRowContext(r.Context(), "SELECT id FROM creator.creators WHERE user_id=$1", claims(r).Subject).Scan(&creatorID)
	if e == sql.ErrNoRows {
		fail(w, 400, "create creator profile first")
		return
	}
	if e != nil {
		fail(w, 500, "profile lookup failed")
		return
	}
	var id string
	e = a.db.QueryRowContext(r.Context(), "INSERT INTO creator.sticker_packs(creator_id,title,price_minor,currency) VALUES($1,$2,$3,$4) RETURNING id", creatorID, in.Title, in.PriceMinor, in.Currency).Scan(&id)
	if e != nil {
		fail(w, 500, "pack creation failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "title": in.Title, "status": "draft"})
}
