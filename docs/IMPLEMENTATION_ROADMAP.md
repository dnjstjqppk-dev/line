# Implementation roadmap and handoff

Updated: 2026-10-28. Work locally. Do not commit, push, or deploy; the user will
hand changes to Gemini for Git operations. Preserve pre-existing changes.

## Architecture decision

Use Go + PostgreSQL as the authoritative API and durable store. Cloudflare serves
assets and optionally proxies every API and WebSocket request to that backend.
Set the Worker variable `API_BACKEND_ORIGIN` to an HTTPS origin with no path,
credentials, query, or fragment. Without it, the existing in-memory demo remains
active. Backend errors must never fall back to demo accounts/data.

This is an opt-in integration foundation, not a production cutover. No backend
origin has been provisioned or configured. The Go and Worker APIs still differ
for reactions, polls, and other UI features. Audit those contracts before enabling
the proxy for users. Existing demo data is not migrated automatically. Configure
the Go `ALLOWED_ORIGINS` with the public website origin for WebSocket handshakes.

## Schedule by acceptance gate

No calendar estimates are assigned before measuring the remaining work. Execute
these stages in order; encryption design must begin during stage A.

| Stage | Scope | Exit gate | Current state |
|---|---|---|---|
| A | Authoritative storage and API parity | Restart-safe messages, Keep, reactions and polls; agreed contracts; versioned migrations | IN_PROGRESS |
| B | Reliable delivery | Shared tickets, durable fanout, retries, ordering, offline catch-up; two-server tests | IN_PROGRESS: shared tickets implemented; fanout pending |
| C | Account security | Verified identity, recovery, refresh rotation, revocation, rate limits, authorization tests | PENDING |
| D | Media | Authorized object storage, resumable uploads, thumbnails, limits and scanning | PENDING |
| E | Calling | Real WebRTC, TURN/SFU, reconnect and screen capture tested on devices | PENDING |
| F | Push and clients | Background push, mobile/desktop install/update and cross-device read state | PENDING |
| G | E2EE | Reviewed protocol, device keys, verification, key changes and encrypted backup | PENDING; design starts in A |
| H | Operations | Load tests, monitoring, restore drills, moderation and retention | PENDING |
| I | Platform | Official accounts, bots, commerce/refunds/settlements and ads | PENDING |

## Current local implementation

- Worker proxy preserves paths, query strings, methods, bodies and WebSocket
  upgrade headers. It disables redirect following and reports invalid config or
  unreachable upstream without entering the demo handler.
- Go Keep saves require an accessible undeleted source message. Legacy `body`
  and `kind` inputs are accepted but never trusted. List output includes web
  client fields and excludes deleted messages or conversations the viewer left.
- Go Keep saves/deletions notify the owner's connections in that process.
- Go WebSocket tickets use hashed tokens in shared DB storage and atomic
  `DELETE ... RETURNING`, so tickets can be redeemed on another API instance.
  Expired tickets and concurrent reuse are rejected.
- For existing PostgreSQL databases apply
  [002_shared_ws_tickets.sql](../database/002_shared_ws_tickets.sql) before running
  the new API. Fresh databases include the table in the initial schema.

## Verification and limits

- `node --test scripts/cf_gateway.test.mjs scripts/cf_worker.test.mjs`: passed.
  Proxy fetch and Worker WebSocketPair are simulated; no live edge deployment.
- `go test ./internal/server -run TestKeepPersistenceAndAuthorization -count=1`:
  passed. File-backed SQLite proves restart persistence, duplicate message
  retries, private Keep access and membership removal behavior.
- The test environment does not prove PostgreSQL locking semantics, live proxy
  WebSocket transport, distributed fanout or end-to-end browser compatibility.

## Next concrete tasks

1. Complete poll and reaction API parity, including viewer-specific votes,
   toggles, list reconstruction, persistence and web refresh events.
2. Implement a durable event relay with explicit recipient scope and per-replica
   delivery. Do not mark a global outbox row delivered after notifying just one
   local hub. Address commit ordering, restart/replay, bounded buffers and lag.
3. Move all relevant mutations into transactions that also record their events.
4. Verify two API instances, reconnect catch-up, duplicate delivery and crash
   recovery; then add offline client queue behavior.
5. Add migration version tracking and PostgreSQL integration tests before any
   production cutover. Reconcile legacy CSV/XLSX reports separately; they still
   contain historical claims and are not release evidence.
