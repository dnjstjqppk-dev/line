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
	"path/filepath"
	"strings"
	"time"

	"github.com/example/global-messenger/backend/internal/auth"
	internaldb "github.com/example/global-messenger/backend/internal/db"
	"github.com/example/global-messenger/backend/internal/realtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	DSN, Secret, Port, Service string
	Origins                    []string
	PushTokenKey               string
	MessageEncryptionKey       string
	FCMProjectID               string
	FCMAccessToken             string
	FCMServiceAccountJSON      string
	APNsKeyID                  string
	APNsTeamID                 string
	APNsPrivateKey             string
	APNsBundleID               string
	APNsProduction             bool
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		DSN:                  os.Getenv("DATABASE_URL"),
		Secret:               os.Getenv("JWT_SECRET"),
		Port:                 os.Getenv("PORT"),
		Service:              os.Getenv("SERVICE_NAME"),
		Origins:              strings.Split(os.Getenv("ALLOWED_ORIGINS"), ","),
		PushTokenKey:         os.Getenv("PUSH_TOKEN_ENCRYPTION_KEY"),
		MessageEncryptionKey: os.Getenv("MESSAGE_ENCRYPTION_KEY"),
		FCMProjectID:         os.Getenv("FCM_PROJECT_ID"),
		FCMAccessToken:       os.Getenv("FCM_ACCESS_TOKEN"),
		FCMServiceAccountJSON: os.Getenv("FCM_SERVICE_ACCOUNT_JSON"),
		APNsKeyID:             os.Getenv("APNS_KEY_ID"),
		APNsTeamID:            os.Getenv("APNS_TEAM_ID"),
		APNsPrivateKey:        os.Getenv("APNS_PRIVATE_KEY"),
		APNsBundleID:          os.Getenv("APNS_BUNDLE_ID"),
	}
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
	db          *sql.DB
	cfg         Config
	hub         *realtime.Hub
	limits      authLimiter
	relayCancel context.CancelFunc
	instanceID  string
	msgCipher   *messageCipher
}

func New(cfg Config) (*App, error) {
	driverName := "pgx"
	if strings.HasPrefix(cfg.DSN, "sqlite://") {
		internaldb.RegisterSQLiteCompat()
		driverName = "pgsqlite"
	}
	db, e := sql.Open(driverName, cfg.DSN)
	if e != nil {
		return nil, e
	}
	if driverName == "pgsqlite" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, fmt.Errorf("database unavailable: %w", e)
	}
	if driverName == "pgsqlite" {
		for _, candidate := range []string{
			os.Getenv("SCHEMA_SQL_PATH"),
			filepath.Join("database", "001_all_phases.sql"),
			filepath.Join("..", "database", "001_all_phases.sql"),
			filepath.Join("..", "..", "database", "001_all_phases.sql"),
			filepath.Join("..", "..", "..", "database", "001_all_phases.sql"),
		} {
			if candidate != "" {
				if _, err := os.Stat(candidate); err == nil {
					if err := internaldb.ApplySchemaFile(ctx, db, candidate); err != nil {
						db.Close()
						return nil, fmt.Errorf("apply schema %s: %w", candidate, err)
					}
					break
				}
			}
		}
	}
	_, _ = db.ExecContext(ctx, "INSERT INTO ops.schema_migrations(version) VALUES('001_all_phases') ON CONFLICT(version) DO NOTHING")
	instID, _ := randomSecret(8)
	if instID == "" {
		instID = "instance"
	}
	key := cfg.MessageEncryptionKey
	if key == "" {
		key = cfg.Secret
	}
	mc, _ := newMessageCipher(key)
	app := &App{db: db, cfg: cfg, hub: realtime.New(), instanceID: instID, msgCipher: mc}
	ctx2, cancel := context.WithCancel(context.Background())
	app.relayCancel = cancel
	go app.runOutboxRelay(ctx2)
	go app.runPushDelivery(ctx2)
	return app, nil
}

func (a *App) Close() {
	if a.relayCancel != nil {
		a.relayCancel()
	}
	_ = a.db.Close()
}

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
				if strings.TrimSpace(allowed) == origin || strings.TrimSpace(allowed) == "*" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
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
		if !a.sessionActive(r.Context(), c.Subject, c.SessionID) {
			fail(w, 401, "session unavailable")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, c)))
	}
}

func (a *App) withRateLimit(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.limits.allow(r.RemoteAddr, route, time.Now()) {
			fail(w, 429, "too many requests")
			return
		}
		next(w, r)
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

var domainServices = []string{
	"api-gateway", "identity-service", "user-service", "device-service", "social-graph-service",
	"chat-service", "message-service", "realtime-service", "presence-service", "media-service",
	"file-service", "call-signaling-service", "notification-service", "search-service", "community-service",
	"business-account-service", "customer-support-service", "bot-service", "developer-platform-service",
	"miniapp-service", "sticker-service", "payment-service", "billing-service", "ads-service",
	"creator-service", "moderation-service", "audit-service", "analytics-service",
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
		send(w, 200, map[string]string{"status": "ready", "service": a.cfg.Service})
	})
	mux.HandleFunc("GET /metrics", a.prometheusMetrics)
	mux.HandleFunc("GET /api/v1/services", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"active_service": a.cfg.Service, "services": domainServices, "count": len(domainServices)})
	})

	active := func(names ...string) bool {
		if a.cfg.Service == "gateway" || a.cfg.Service == "api-gateway" {
			return true
		}
		for _, n := range names {
			if a.cfg.Service == n || a.cfg.Service == n+"-service" {
				return true
			}
		}
		return false
	}

	if active("identity", "user", "device") {
		mux.HandleFunc("POST /api/v1/auth/register", a.withRateLimit("/api/v1/auth/register", a.register))
		mux.HandleFunc("POST /api/v1/auth/login", a.withRateLimit("/api/v1/auth/login", a.login))
		mux.HandleFunc("POST /api/v1/auth/refresh", a.withRateLimit("/api/v1/auth/refresh", a.refreshToken))
		mux.HandleFunc("POST /api/v1/auth/logout", a.withAuth(a.logout))
		mux.HandleFunc("PATCH /api/v1/auth/password", a.withAuth(a.changePassword))
		mux.HandleFunc("POST /api/v1/auth/recover", a.withRateLimit("/api/v1/auth/recover", a.recoverAccount))
		mux.HandleFunc("POST /api/v1/auth/recover/verify", a.withRateLimit("/api/v1/auth/recover/verify", a.verifyRecovery))
		mux.HandleFunc("POST /api/v1/auth/qr/challenge", a.withRateLimit("/api/v1/auth/qr/challenge", a.qrChallenge))
		mux.HandleFunc("POST /api/v1/auth/qr/approve", a.withAuth(a.qrApprove))
		mux.HandleFunc("POST /api/v1/auth/qr/poll", a.withRateLimit("/api/v1/auth/qr/poll", a.qrPoll))
		mux.HandleFunc("POST /api/v1/auth/verify/email/request", a.withRateLimit("/api/v1/auth/verify/email/request", a.withAuth(a.requestEmailVerification)))
		mux.HandleFunc("POST /api/v1/auth/verify/email/confirm", a.withRateLimit("/api/v1/auth/verify/email/confirm", a.withAuth(a.verifyEmail)))
		mux.HandleFunc("POST /api/v1/uploads/init", a.withAuth(a.initUpload))
		mux.HandleFunc("PUT /api/v1/uploads/{id}/chunks/{chunk}", a.withAuth(a.uploadChunk))
		mux.HandleFunc("POST /api/v1/uploads/{id}/complete", a.withAuth(a.completeUpload))
		mux.HandleFunc("GET /api/v1/push/credentials", a.withAuth(a.pushCredentialCheck))
		mux.HandleFunc("POST /api/v1/push/test", a.withAuth(a.pushTest))
		mux.HandleFunc("GET /api/v1/push/health", a.withAuth(a.pushHealth))
		mux.HandleFunc("GET /api/v1/me", a.withAuth(a.me))
		mux.HandleFunc("PATCH /api/v1/me", a.withAuth(a.updateMe))
		mux.HandleFunc("GET /api/v1/me/settings", a.withAuth(a.getSettings))
		mux.HandleFunc("PUT /api/v1/me/settings", a.withAuth(a.updateSettings))
		mux.HandleFunc("GET /api/v1/me/backups", a.withAuth(a.listBackups))
		mux.HandleFunc("POST /api/v1/me/backups", a.withAuth(a.createBackup))
		mux.HandleFunc("DELETE /api/v1/me", a.withAuth(a.deleteAccount))
		mux.HandleFunc("GET /api/v1/devices", a.withAuth(a.listDevices))
		mux.HandleFunc("DELETE /api/v1/devices/{id}", a.withAuth(a.revokeDevice))
		mux.HandleFunc("PUT /api/v1/devices/{id}/keys", a.withAuth(a.uploadDeviceKeys))
		mux.HandleFunc("GET /api/v1/users/{id}/keys", a.withAuth(a.getUserKeys))
	}
	if active("social", "social-graph") {
		mux.HandleFunc("GET /api/v1/users", a.withAuth(a.users))
		mux.HandleFunc("GET /api/v1/friends", a.withAuth(a.friends))
		mux.HandleFunc("POST /api/v1/friends", a.withAuth(a.addFriend))
		mux.HandleFunc("DELETE /api/v1/friends/{id}", a.withAuth(a.removeFriend))
		mux.HandleFunc("GET /api/v1/blocks", a.withAuth(a.blocks))
		mux.HandleFunc("POST /api/v1/blocks", a.withAuth(a.blockUser))
		mux.HandleFunc("DELETE /api/v1/blocks/{id}", a.withAuth(a.unblockUser))
		mux.HandleFunc("POST /api/v1/friends/qr", a.withAuth(a.createFriendQR))
		mux.HandleFunc("POST /api/v1/friends/qr/redeem", a.withAuth(a.redeemFriendQR))
		mux.HandleFunc("POST /api/v1/friends/sync-contacts", a.withAuth(a.syncContacts))
	}
	if active("chat", "message", "search") {
		mux.HandleFunc("GET /api/v1/conversations", a.withAuth(a.conversations))
		mux.HandleFunc("POST /api/v1/conversations", a.withAuth(a.createConversation))
		mux.HandleFunc("GET /api/v1/conversations/{id}/messages", a.withAuth(a.messages))
		mux.HandleFunc("GET /api/v1/conversations/{id}/sync", a.withAuth(a.syncMessages))
		mux.HandleFunc("POST /api/v1/conversations/{id}/messages", a.withAuth(a.sendMessage))
		mux.HandleFunc("PATCH /api/v1/conversations/{id}/messages/{msgId}", a.withAuth(a.editMessage))
		mux.HandleFunc("DELETE /api/v1/conversations/{id}/messages/{msgId}", a.withAuth(a.deleteMessage))
		mux.HandleFunc("POST /api/v1/conversations/{id}/messages/{msgId}/reactions", a.withAuth(a.reactMessage))
		mux.HandleFunc("POST /api/v1/conversations/{id}/pins", a.withAuth(a.pinMessage))
		mux.HandleFunc("GET /api/v1/conversations/{id}/pins", a.withAuth(a.listPins))
		mux.HandleFunc("POST /api/v1/conversations/{id}/read", a.withAuth(a.markRead))
		mux.HandleFunc("POST /api/v1/conversations/{id}/typing", a.withAuth(a.typing))
		mux.HandleFunc("GET /api/v1/conversations/{id}/polls", a.withAuth(a.listPolls))
		mux.HandleFunc("POST /api/v1/conversations/{id}/polls", a.withAuth(a.createPoll))
		mux.HandleFunc("POST /api/v1/polls/{pollId}/vote", a.withAuth(a.votePoll))
		mux.HandleFunc("POST /api/v1/polls/{pollId}/close", a.withAuth(a.closePoll))
		mux.HandleFunc("POST /api/v1/conversations/{id}/albums", a.withAuth(a.createAlbum))
		mux.HandleFunc("GET /api/v1/conversations/{id}/albums", a.withAuth(a.listAlbums))
		mux.HandleFunc("POST /api/v1/saved-messages", a.withAuth(a.saveMessage))
		mux.HandleFunc("GET /api/v1/saved-messages", a.withAuth(a.listSavedMessages))
		mux.HandleFunc("DELETE /api/v1/saved-messages/{id}", a.withAuth(a.deleteSavedMessage))
		mux.HandleFunc("GET /api/v1/search", a.withAuth(a.unifiedSearch))
	}
	if active("realtime", "presence", "notification") {
		mux.HandleFunc("POST /api/v1/ws-ticket", a.withAuth(a.wsTicket))
		mux.HandleFunc("GET /api/v1/ws", a.websocket)
		mux.HandleFunc("POST /api/v1/presence/heartbeat", a.withAuth(a.heartbeatPresence))
		mux.HandleFunc("GET /api/v1/presence", a.withAuth(a.getPresence))
		mux.HandleFunc("POST /api/v1/push-tokens", a.withAuth(a.registerPushToken))
	}
	if active("media", "file", "calling", "call-signaling") {
		mux.HandleFunc("POST /api/v1/attachments", a.withAuth(a.uploadAttachment))
		mux.HandleFunc("GET /api/v1/attachments/{id}", a.withAuth(a.getAttachment))
		mux.HandleFunc("GET /api/v1/attachments/{id}/content", a.withAuth(a.downloadAttachment))
		mux.HandleFunc("DELETE /api/v1/attachments/{id}", a.withAuth(a.deleteAttachment))
		mux.HandleFunc("POST /api/v1/calls", a.withAuth(a.startCall))
		mux.HandleFunc("GET /api/v1/calls", a.withAuth(a.listCalls))
		mux.HandleFunc("POST /api/v1/calls/{id}/signal", a.withAuth(a.callSignal))
		mux.HandleFunc("POST /api/v1/calls/{id}/state", a.withAuth(a.updateCallState))
		mux.HandleFunc("POST /api/v1/calls/{id}/quality", a.withAuth(a.recordCallQuality))
	}
	if active("sticker", "payment", "billing", "community") {
		mux.HandleFunc("GET /api/v1/store/stickers", a.withAuth(a.listStoreStickers))
		mux.HandleFunc("POST /api/v1/store/orders", a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			fail(w, http.StatusServiceUnavailable, "payments unavailable: verified provider integration required")
		}))
		mux.HandleFunc("GET /api/v1/store/orders", a.withAuth(a.listOrders))
		mux.HandleFunc("GET /api/v1/community/rooms", a.withAuth(a.listCommunityRooms))
		mux.HandleFunc("POST /api/v1/community/rooms", a.withAuth(a.createCommunityRoom))
		mux.HandleFunc("POST /api/v1/community/rooms/{id}/join", a.withAuth(a.joinCommunityRoom))
		mux.HandleFunc("GET /api/v1/community/rooms/{id}/posts", a.withAuth(a.listCommunityPosts))
		mux.HandleFunc("POST /api/v1/community/rooms/{id}/posts", a.withAuth(a.createCommunityPost))
	}
	if active("moderation", "audit") {
		mux.HandleFunc("POST /api/v1/reports", a.withAuth(a.report))
		mux.HandleFunc("POST /api/v1/privacy-requests", a.withAuth(a.createPrivacyRequest))
	}
	if active("admin", "analytics", "audit") {
		mux.HandleFunc("GET /api/v1/admin/stats", a.withAuth(requireRoles("platform_admin", "support")(a.stats)))
		mux.HandleFunc("GET /api/v1/admin/users", a.withAuth(requireRoles("platform_admin", "support")(a.adminUsers)))
		mux.HandleFunc("GET /api/v1/admin/reports", a.withAuth(requireRoles("platform_admin", "moderator")(a.adminReports)))
		mux.HandleFunc("POST /api/v1/admin/moderation/actions", a.withAuth(requireRoles("platform_admin", "moderator")(a.moderationAction)))
		mux.HandleFunc("GET /api/v1/admin/audit-logs", a.withAuth(requireRoles("platform_admin")(a.adminAuditLogs)))
		mux.HandleFunc("GET /api/v1/admin/feature-flags", a.withAuth(requireRoles("platform_admin")(a.listFeatureFlags)))
		mux.HandleFunc("PUT /api/v1/admin/feature-flags", a.withAuth(requireRoles("platform_admin")(a.upsertFeatureFlag)))
	}
	if active("partner", "business-account", "customer-support") {
		mux.HandleFunc("POST /api/v1/partner/organizations", a.withAuth(a.createOrganization))
		mux.HandleFunc("GET /api/v1/partner/organizations", a.withAuth(a.organizations))
		mux.HandleFunc("POST /api/v1/partner/organizations/{id}/accounts", a.withAuth(a.createOfficialAccount))
		mux.HandleFunc("GET /api/v1/partner/organizations/{id}/accounts", a.withAuth(a.officialAccounts))
		mux.HandleFunc("POST /api/v1/partner/accounts/{id}/contacts", a.withAuth(a.addBusinessContact))
		mux.HandleFunc("POST /api/v1/partner/accounts/{id}/tickets", a.withAuth(a.createSupportTicket))
		mux.HandleFunc("GET /api/v1/partner/accounts/{id}/tickets", a.withAuth(a.listSupportTickets))
		mux.HandleFunc("POST /api/v1/partner/accounts/{id}/campaigns", a.withAuth(a.createBusinessCampaign))
		mux.HandleFunc("POST /api/v1/partner/accounts/{id}/rich-menus", a.withAuth(a.createRichMenu))
		mux.HandleFunc("POST /api/v1/partner/accounts/{id}/coupons", a.withAuth(a.createCoupon))
	}
	if active("developer", "developer-platform", "bot", "miniapp") {
		mux.HandleFunc("POST /api/v1/developer/organizations", a.withAuth(a.createDeveloperOrganization))
		mux.HandleFunc("GET /api/v1/developer/organizations", a.withAuth(a.developerOrganizations))
		mux.HandleFunc("POST /api/v1/developer/organizations/{id}/apps", a.withAuth(a.createDeveloperApp))
		mux.HandleFunc("GET /api/v1/developer/organizations/{id}/apps", a.withAuth(a.listDeveloperApps))
		mux.HandleFunc("POST /api/v1/developer/apps/{id}/webhooks", a.withAuth(a.createWebhook))
		mux.HandleFunc("POST /api/v1/developer/apps/{id}/miniapps", a.withAuth(a.createMiniApp))
	}
	if active("creator") {
		mux.HandleFunc("POST /api/v1/creator/profile", a.withAuth(a.createCreator))
		mux.HandleFunc("GET /api/v1/creator/packs", a.withAuth(a.creatorPacks))
		mux.HandleFunc("POST /api/v1/creator/packs", a.withAuth(a.createStickerPack))
		mux.HandleFunc("POST /api/v1/creator/packs/{id}/stickers", a.withAuth(a.addStickerItem))
		mux.HandleFunc("GET /api/v1/creator/settlements", a.withAuth(a.creatorSettlements))
	}
	if active("ads") {
		mux.HandleFunc("GET /api/v1/ads/advertisers", a.withAuth(a.advertisers))
		mux.HandleFunc("POST /api/v1/ads/advertisers", a.withAuth(a.createAdvertiser))
		mux.HandleFunc("POST /api/v1/ads/campaigns", a.withAuth(a.createAdsCampaign))
		mux.HandleFunc("GET /api/v1/ads/campaigns", a.withAuth(a.adsCampaigns))
		mux.HandleFunc("POST /api/v1/ads/campaigns/{id}/groups", a.withAuth(a.createAdGroup))
	}
	return a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && !a.limits.allow(r.RemoteAddr, r.URL.Path, time.Now()) {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "authentication rate limit exceeded")
			return
		}
		mux.ServeHTTP(w, r)
		if time.Since(start) > 2*time.Second {
			log.Printf("slow request %s %s elapsed=%s", r.Method, r.URL.Path, time.Since(start))
		}
	}))
}
