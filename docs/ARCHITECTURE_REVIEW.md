# Architecture review
Go plus PostgreSQL is the intended authoritative service. SQLite is a local compatibility/test adapter and does not establish PostgreSQL transaction semantics. The current WebSocket hub distributes only within its process; DB-backed tickets do not solve replica fanout.

The optional edge proxy must preserve Go authentication and errors. The fallback in-memory Worker is not durable messenger storage. UI parity remains an acceptance gate even where server poll/reaction tests pass.

Use transactional messages plus recipient-scoped durable events for reliable delivery; define replay cursors and per-replica acknowledgement before implementing a relay. Never treat a single local hub notification as global event delivery.

Media needs authorized byte storage and integrity verification before reporting ready. Calling needs client WebRTC media transport, TURN credentials and an SFU decision for group calls. Push needs a provider adapter, retry/deduplication and background client integration.

Preserve existing tables and apply ordered, reviewed migrations. No schema change was made for this checkpoint's security fixes.
