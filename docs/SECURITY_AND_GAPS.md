# Independent technical honesty report

This repository is an **implementation foundation** for a LINE-class messenger, not a completed reproduction of LINE and not safe to deploy publicly as-is.

## Explicit unimplemented P0/P1 release blockers

1. Email and phone possession verification; account recovery and MFA/passkey; brute-force/rate-limit protection.
2. Refresh-token rotation, session/device revocation, robust identity lifecycle; short-lived access JWTs are currently the only usable credential.
3. End-to-end encryption protocol, per-device multi-party key handling, verification and secure backups. Schema exists; **messages are presently stored in server-readable plaintext**.
4. Production authorization review, tenancy isolation proofs, privacy request workflows and retention/deletion propagation; no independent security audit.
5. Distributed WebSocket fanout. Current realtime hub is **in-memory per process**; durable outbox is recorded but not drained to NATS. Event delivery across replicas is not guaranteed.
6. Actual WebRTC audio/video, signaling, SFU, TURN and desktop native capture are NOT implemented.
7. Actual media upload, signed object storage, transcoding, malware inspection and at-rest encryption are NOT implemented.
8. Payment, refund, creator settlement, billing and ad delivery/auction are NOT implemented; drafts only.
9. Official account campaigns, queue-backed broadcast, customer support UI, bots and production webhooks are NOT implemented.
10. API semantics and protobuf code generation are not yet integrated. Proto files describe target contracts, not running gRPC handlers.
11. DB migration versioning, reversibility, zero-downtime migration test and partitioning strategy must be implemented. Initial SQL is not tested against a live PostgreSQL here.
12. Backpressure, group chat contention, duplicate direct conversation race, client offline queue and guaranteed per-device sync are not production complete.
13. Alerting/monitoring/tracing, audit trails for sensitive admin actions, breach response, risk assessment and compliance reviews remain outstanding.
14. Real Android/iOS native project scaffolds, signed desktop binaries, store submission, multi-browser UX accessibility tests not completed.

## Current safeguards

- Database membership checks for chat read/write; partner roles enforced server-side.
- Password PBKDF2-HMAC-SHA256 with a per-user 16-byte random salt and 310000 rounds (development baseline; switch to audited memory-hard Argon2id for production).
- HMAC-SHA256 bearer JWT validation with strict algorithm and issuer/audience/time checks; 32-byte minimum signing secret.
- Short-lived single-use WebSocket tickets to avoid putting the bearer JWT in a long-lived URL.
- Secrets of developer apps are only returned once; only their SHA-256 hashes are stored. OAuth grants are not implemented.
- Transactional message persistence, monotonic per-conversation sequence and idempotent client message ID.

## Required validation after restoring network/toolchain

`cd backend && go mod tidy && go test ./... && go vet ./...`;
`npm install && npm run build:apps`;
`cd mobile/flutter_app && flutter create --platforms=android,ios . && flutter pub get && flutter analyze && flutter test`;
`docker compose up --build -d && python3 scripts/smoke_test.py`;
`psql -f database/001_all_phases.sql` on a new DB; `protoc --proto_path=proto ...` for all `.proto` files.

No claims are made that those checks passed in this environment. Do not use this repository with real personal data or real payments until above blockers are cleared.
