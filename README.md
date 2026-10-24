# Global Messenger — Vue · Go · Flutter · PostgreSQL · Protobuf

**Status: substantial developer foundation / NOT a commercially complete LINE clone.** Independent service design; not affiliated with LINE. See `docs/SECURITY_AND_GAPS.md` before operating.

## Repository

- `backend/` Go REST domain handlers; WebSocket; PostgreSQL persistence; in-memory dev fanout; JWT/password utilities.
- `database/001_all_phases.sql` wide-domain PostgreSQL schema for identity, social, chat, realtime, calling, crypto, community, business, developer, creator, commerce, ads, safety and operations.
- `proto/messenger/v1/*.proto` 15 Protobuf domain contracts (targets, not compiled service implementations).
- `apps/user-web/` actual Vue registration/login, user search/friend add, chat list, message send/history, WebSocket events.
- `apps/admin-console/` actual admin metrics, users and reports (requires platform_admin role).
- `apps/partner-console/` actual organization and official-account create/list with server RBAC.
- `apps/developer-console/` developer organization and app registration, one-time client secret issuance.
- `apps/creator-console/` creator profile and sticker pack draft CRUD.
- `apps/ads-console/` advertiser and ad campaign draft CRUD.
- `mobile/flutter_app/` Flutter Dart authentication, contacts, chat + events; native runners must be generated.
- `desktop/user-desktop/` unverified Tauri 2/Rust shell; not an installed PC program.
- `docs/` ERD, API and gap descriptions.

## Quick start (requires Docker, internet access for dependencies, Node 22+, npm)

```bash
cp .env.example .env
# Replace JWT_SECRET with output of: openssl rand -hex 32
# IMPORTANT: Do not keep the default example secret for real use.
docker compose up --build -d
python3 scripts/smoke_test.py
npm install
npm run dev:user        # http://localhost:5173
npm run dev:admin       # http://localhost:5174
npm run dev:partner     # http://localhost:5175
npm run dev:developer   # http://localhost:5176
npm run dev:creator     # http://localhost:5177
npm run dev:ads         # http://localhost:5178
```

**Note:** because `.env.example` contains a placeholder, replace it with `JWT_SECRET=$(openssl rand -hex 32)` before running the API. An existing database volume is NOT automatically remigrated when schema SQL changes. To reset the **development** database only: `docker compose down -v` (deletes all development data), then `docker compose up --build -d`.

Admin role bootstrap for local isolated dev only: register a normal account, then from psql with privileged DB credentials:

```sql
UPDATE identity.users SET role = 'platform_admin' WHERE email = 'my-local-admin@example.test';
```

Never expose this approach as a public self-service admin grant.

## Go tests

```bash
cd backend
go mod tidy
GOPROXY=https://proxy.golang.org go test ./...
```

An offline stdlib-only `go test ./internal/auth ./internal/realtime` was executed successfully when packaging. Full compilation was blocked by missing `pgx`/`gorilla/websocket` dependencies and disabled network access. No production E2E PASS is asserted.

## Microservice process composition

Go `SERVICE_NAME` selects the domain endpoints. For example, run chat separately with `SERVICE_NAME=chat PORT=8082`, identity with `SERVICE_NAME=identity PORT=8081`, and realtime with `SERVICE_NAME=realtime PORT=8083`. Dev `SERVICE_NAME=gateway` hosts routes together. **The production gateway proxy, independent DB ownership and NATS fanout are planned, not complete.** This is a service-oriented modular monolith development baseline with independently selectable process entrypoints, not a complete 25-service deployment.

## User privacy

**Chat messages in the MVP are plaintext in PostgreSQL**. Neither Letter Sealing nor any production E2EE is implemented. Do not use real private conversations. Do not enable payment, video calling, claims of data confidentiality or public launch without the open security work.
