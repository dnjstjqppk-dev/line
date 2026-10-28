# Codebase inventory
2026-10-28. Source inventory excludes dependencies, generated builds and binary archives; it is not a statement that every file has been reviewed.
| Area | Source | Runtime / responsibility |
|---|---|---|
| API | backend/cmd/server; backend/internal/server | Go HTTP domain handlers |
| Authentication | backend/internal/auth | Password hashing and JWT |
| Realtime | backend/internal/realtime; backend/internal/server/realtime.go | Process-local hub and DB-backed tickets |
| DB | database/001_all_phases.sql; 002_shared_ws_tickets.sql; 003_polls_and_reactions.sql | PostgreSQL schema and incremental SQL |
| Local DB adapter | backend/internal/db | SQLite SQL translation used by tests |
| Web | apps/user-web and five console apps; apps/shared | Vue 3 / TypeScript / Vite |
| Mobile | mobile/flutter_app/lib/main.dart; android; test | Flutter Android client |
| Desktop | desktop/user-desktop/src-tauri | Tauri 2 / Rust; not built this run |
| Edge | scripts/cf_worker.js; scripts/build_cf.js | In-memory handler or optional Go proxy |
| Contracts | proto/messenger/v1; docs/API.md | Contract documents; not proof of runtime parity |
| Tooling | scripts; .vscode/tasks.json; .github | Build/test/deploy configuration |
Existing root APK and ZIP are historical artifacts and were not re-certified. The new debug APK is under mobile/flutter_app/build/app/outputs/flutter-apk.
No AGENTS.md was found by the project file search.
