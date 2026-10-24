package server

import (
	"database/sql"
	"errors"
	"github.com/example/global-messenger/backend/internal/auth"
	"net/http"
	"strings"
	"time"
)

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if e := decode(r, &input); e != nil {
		fail(w, 400, "invalid request")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !strings.Contains(input.Email, "@") || len(input.Email) > 254 || len(input.DisplayName) < 1 || len(input.DisplayName) > 120 {
		fail(w, 400, "invalid email or name")
		return
	}
	hash, e := auth.HashPassword(input.Password)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	var id string
	e = a.db.QueryRowContext(r.Context(), "INSERT INTO identity.users(email,password_hash,display_name) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING RETURNING id", input.Email, hash, input.DisplayName).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 409, "account already exists")
		return
	}
	if e != nil {
		fail(w, 500, "registration failed")
		return
	}
	send(w, 201, map[string]string{"id": id, "status": "created"})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(r, &input) != nil {
		fail(w, 400, "invalid request")
		return
	}
	var id, role, hash, state string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,role,password_hash,state FROM identity.users WHERE email=$1", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&id, &role, &hash, &state)
	if e != nil || state != "active" || !auth.CheckPassword(input.Password, hash) {
		fail(w, 401, "invalid credentials")
		return
	}
	token, e := auth.Sign(id, role, []byte(a.cfg.Secret), 15*time.Minute)
	if e != nil {
		fail(w, 500, "token signing unavailable")
		return
	}
	send(w, 200, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 900, "user_id": id, "role": role})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	var id, email, name, role string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,email,display_name,role FROM identity.users WHERE id=$1", claims(r).Subject).Scan(&id, &email, &name, &role)
	if e != nil {
		fail(w, 404, "not found")
		return
	}
	send(w, 200, map[string]string{"id": id, "email": email, "display_name": name, "role": role})
}
