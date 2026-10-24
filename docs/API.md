# Implemented REST contract — MVP

Base: `http://localhost:8080/api/v1`. JSON transport, bearer access token (HS256 JWT, 15-minute expiry). 201 = created, 200 = succeeded, 401 = auth, 403 = permission, 400 = invalid input. OpenAPI and Proto definitions for the **entire envisioned platform** are reference contracts, not proof of working handlers.

## Authentication

| Verb | Route | Role | State |
|---|---|---|---|
| POST | `/auth/register` | anonymous | implemented; email verification missing |
| POST | `/auth/login` | anonymous | implemented; throttling/refresh missing |
| GET | `/me` | signed in | implemented |
| GET | `/users?search=name` | signed in | implemented |
| GET | `/friends` | signed in | implemented |
| POST | `/friends` `{ "friend_id": "UUID" }` | signed in | implemented |

## Chat

| Verb | Route | Role | State |
|---|---|---|---|
| GET | `/conversations` | user | implemented |
| POST | `/conversations` `{ "kind": "direct", "member_ids": ["UUID"], "title": "title" }` | user | implemented |
| GET | `/conversations/{id}/messages?before=101&limit=50` | member | implemented |
| POST | `/conversations/{id}/messages` `{ "client_id": "unique", "kind": "text", "body": "hello" }` | member | implemented |
| POST | `/conversations/{id}/read` `{ "sequence": 100 }` | member | implemented |
| POST | `/ws-ticket` | signed in | implemented; in-memory 30-second one-use ticket |
| GET | `/ws?ticket=...` | valid ticket | implemented; single process only |

WebSocket server events: `{"type":"message.created","message":{"id":"...","conversation_id":"...","sender_id":"...","client_id":"...","sequence":1,"kind":"text","body":"hi","created_at":"..."}}`.

## Admin and partner

| Verb | Route | Role | State |
|---|---|---|---|
| GET | `/admin/stats` | platform_admin, support | implemented |
| GET | `/admin/users` | platform_admin, support | implemented |
| GET | `/admin/reports` | platform_admin, moderator | implemented |
| POST | `/reports` | signed in | implemented |
| GET/POST | `/partner/organizations` | signed in | implemented |
| GET/POST | `/partner/organizations/{id}/accounts` | member/admin respectively | implemented |
| GET/POST | `/developer/organizations` | signed in | implemented |
| GET/POST | `/developer/organizations/{id}/apps` | organization member / owner | implemented |
| POST | `/creator/profile` | signed in | implemented |
| GET/POST | `/creator/packs` | signed in/creator | implemented (draft-only) |
| GET/POST | `/ads/advertisers` | organization member / owner | implemented (pending-only) |
| GET/POST | `/ads/campaigns` | organization member / admin | implemented (draft-only) |

### Go microservice startup mode

`SERVICE_NAME=gateway` exposes all implemented domains in a single-process development gateway. A `SERVICE_NAME` of `identity`, `social`, `chat`, `realtime`, `partner`, `developer`, `creator`, `ads`, `moderation`, or `admin` runs only that domain's handlers, with a shared database and explicit shared key. This is **not** a production-grade independent service mesh: gateway-to-service proxy routing, distributed message broadcast, and deployment-level service isolation must be built.
