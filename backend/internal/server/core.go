package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
	"github.com/example/global-messenger/backend/internal/realtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	DSN, Secret, Port, Service string
	Origins                    []string
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{DSN: os.Getenv("DATABASE_URL"), Secret: os.Getenv("JWT_SECRET"), Port: os.Getenv("PORT"), Service: os.Getenv("SERVICE_NAME"), Origins: strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.Service == "" {
		cfg.Service = "gateway"
	}
	if cfg.DSN == "" {
		return cfg, errors.New("DATABASE_URL missing")
	}
	if len(cfg.Secret) < 32 {
		return cfg, errors.New("JWT_SECRET must be >=32 bytes")
	}
	return cfg, nil
}

type App struct {
	db  *sql.DB
	cfg Config
	hub *realtime.Hub
}

func New(cfg Config) (*App, error) {
	db, e := sql.Open("pgx", cfg.DSN)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, fmt.Errorf("postgres unavailable: %w", e)
	}
	return &App{db: db, cfg: cfg, hub: realtime.New()}, nil
}
func (a *App) Close() { _ = a.db.Close() }
func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	send(w, status, map[string]string{"error": msg})
}
func decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e == io.EOF {
		return nil
	} else if e == nil {
		return errors.New("multiple JSON objects")
	} else {
		return e
	}
}
func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			for _, allowed := range a.cfg.Origins {
				if strings.TrimSpace(allowed) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
					break
				}
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type key string

const userKey key = "claims"

func (a *App) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := strings.Fields(r.Header.Get("Authorization"))
		if len(h) != 2 || !strings.EqualFold(h[0], "Bearer") {
			fail(w, 401, "bearer token required")
			return
		}
		c, e := auth.Verify(h[1], []byte(a.cfg.Secret))
		if e != nil {
			fail(w, 401, "invalid or expired token")
			return
		}
		var state, role string
		if e = a.db.QueryRowContext(r.Context(), "SELECT state,role FROM identity.users WHERE id=$1", c.Subject).Scan(&state, &role); e != nil || state != "active" {
			fail(w, 401, "account unavailable")
			return
		}
		c.Role = role
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, c)))
	}
}
func claims(r *http.Request) auth.Claims { return r.Context().Value(userKey).(auth.Claims) }
func requireRoles(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			for _, role := range roles {
				if claims(r).Role == role {
					next(w, r)
					return
				}
			}
			fail(w, 403, "insufficient role")
		}
	}
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]string{"status": "ok", "service": a.cfg.Service})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if e := a.db.PingContext(r.Context()); e != nil {
			fail(w, 503, "database unavailable")
			return
		}
		send(w, 200, map[string]string{"status": "ready"})
	})
	active := func(names ...string) bool {
		if a.cfg.Service == "gateway" {
			return true
		}
		for _, n := range names {
			if a.cfg.Service == n {
				return true
			}
		}
		return false
	}
	if active("identity") {
		mux.HandleFunc("POST /api/v1/auth/register", a.register)
		mux.HandleFunc("POST /api/v1/auth/login", a.login)
		mux.HandleFunc("GET /api/v1/me", a.withAuth(a.me))
	}
	if active("social") {
		mux.HandleFunc("GET /api/v1/users", a.withAuth(a.users))
		mux.HandleFunc("GET /api/v1/friends", a.withAuth(a.friends))
		mux.HandleFunc("POST /api/v1/friends", a.withAuth(a.addFriend))
	}
	if active("chat") {
		mux.HandleFunc("GET /api/v1/conversations", a.withAuth(a.conversations))
		mux.HandleFunc("POST /api/v1/conversations", a.withAuth(a.createConversation))
		mux.HandleFunc("GET /api/v1/conversations/{id}/messages", a.withAuth(a.messages))
		mux.HandleFunc("POST /api/v1/conversations/{id}/messages", a.withAuth(a.sendMessage))
		mux.HandleFunc("POST /api/v1/conversations/{id}/read", a.withAuth(a.markRead))
	}
	if active("realtime") {
		mux.HandleFunc("POST /api/v1/ws-ticket", a.withAuth(a.wsTicket))
		mux.HandleFunc("GET /api/v1/ws", a.websocket)
	}
	if active("moderation") {
		mux.HandleFunc("POST /api/v1/reports", a.withAuth(a.report))
	}
	if active("admin") {
		mux.HandleFunc("GET /api/v1/admin/stats", a.withAuth(requireRoles("platform_admin", "support")(a.stats)))
		mux.HandleFunc("GET /api/v1/admin/users", a.withAuth(requireRoles("platform_admin", "support")(a.adminUsers)))
		mux.HandleFunc("GET /api/v1/admin/reports", a.withAuth(requireRoles("platform_admin", "moderator")(a.adminReports)))
	}
	if active("partner") {
		mux.HandleFunc("POST /api/v1/partner/organizations", a.withAuth(a.createOrganization))
		mux.HandleFunc("GET /api/v1/partner/organizations", a.withAuth(a.organizations))
		mux.HandleFunc("POST /api/v1/partner/organizations/{id}/accounts", a.withAuth(a.createOfficialAccount))
		mux.HandleFunc("GET /api/v1/partner/organizations/{id}/accounts", a.withAuth(a.officialAccounts))
	}
	if active("developer") {
		mux.HandleFunc("POST /api/v1/developer/organizations", a.withAuth(a.createDeveloperOrganization))
		mux.HandleFunc("GET /api/v1/developer/organizations", a.withAuth(a.developerOrganizations))
		mux.HandleFunc("POST /api/v1/developer/organizations/{id}/apps", a.withAuth(a.createDeveloperApp))
		mux.HandleFunc("GET /api/v1/developer/organizations/{id}/apps", a.withAuth(a.listDeveloperApps))
	}
	if active("creator") {
		mux.HandleFunc("POST /api/v1/creator/profile", a.withAuth(a.createCreator))
		mux.HandleFunc("GET /api/v1/creator/packs", a.withAuth(a.creatorPacks))
		mux.HandleFunc("POST /api/v1/creator/packs", a.withAuth(a.createStickerPack))
	}
	if active("ads") {
		mux.HandleFunc("GET /api/v1/ads/advertisers", a.withAuth(a.advertisers))
		mux.HandleFunc("POST /api/v1/ads/advertisers", a.withAuth(a.createAdvertiser))
		mux.HandleFunc("POST /api/v1/ads/campaigns", a.withAuth(a.createAdsCampaign))
		mux.HandleFunc("GET /api/v1/ads/campaigns", a.withAuth(a.adsCampaigns))
	}
	if active("media", "calling", "payments", "community", "analytics", "notifications") {
		mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
			send(w, 200, map[string]any{"status": "contract_only", "implemented": false, "service": a.cfg.Service})
		})
	}
	return a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mux.ServeHTTP(w, r)
		if time.Since(start) > 2*time.Second {
			log.Printf("slow request %s %s elapsed=%s", r.Method, r.URL.Path, time.Since(start))
		}
	}))
}
