# LINE-Grade Global Messenger: 001~100 Full Requirements & Implementation Matrix

## Phase 0: Product Design & Baseline (001~010)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 001 | Feature Investigation | Mobile, PC, Web, Business, Developer, Ads, Creator capabilities benchmarked against global standards (LINE/WeChat) | Architecture / Docs | IMPLEMENTED_UNVERIFIED |
| 002 | Requirement Traceability Matrix | Screen, Menu, Button, State, and API mapping without omissions | Architecture / Docs | IMPLEMENTED_UNVERIFIED |
| 003 | Role & RBAC Definitions | End User, Business Owner, Support Staff, Developer, Advertiser, Creator, Moderator, Super Admin | Identity & Gateway | IMPLEMENTED_UNVERIFIED |
| 004 | URL & Isolation Scheme | User Web/PC, Admin Portal (`/admin`), Business Console (`/biz`), Developer Hub (`/dev`), Creator Market (`/creator`) | Gateway / Router | IMPLEMENTED_UNVERIFIED |
| 005 | Platform Support Scope | Android (Flutter), iOS (Flutter), Windows (Tauri/Rust), macOS (Tauri/Rust), Web (Vue 3 TS) | Monorepo Clients | IMPLEMENTED_UNVERIFIED |
| 006 | Multi-Region Compliance | Localization (KR, JP, TW, TH, US), timezone, GDPR/PIPA/APPI compliance, ISO-8601, UTF-8mb4 | Core Architecture | IMPLEMENTED_UNVERIFIED |
| 007 | User & Error Flow Matrix | Offline fallback, network resume, token expiry, race condition, concurrent device access | Core Handlers | IMPLEMENTED_UNVERIFIED |
| 008 | Independent UI/UX System | Clean modern green/dark slate design system, responsive layouts, native feel | Vue 3 + Tailwind | IMPLEMENTED_UNVERIFIED |
| 009 | Design System & i18n | Multi-language (ko, en, ja, zh-TW, th), theme engine (Light/Dark/High-Contrast) | Frontend / Mobile | IMPLEMENTED_UNVERIFIED |
| 010 | Milestone & Acceptance Criteria | Strict definition of done, verification evidence requirements, P0-P3 defect grading | Engineering Ops | IMPLEMENTED_UNVERIFIED |

## Phase 1: Architecture, Database & Core Infrastructure (011~020)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 011 | Monorepo Setup | Vue 3, Flutter, Go, Rust/Tauri monorepo structure with automated tooling | Root Monorepo | IMPLEMENTED_UNVERIFIED |
| 012 | Go API Gateway & Services | High-performance Go microservices / modular monolith with gRPC/REST/WS | `backend/services` | IMPLEMENTED_UNVERIFIED |
| 013 | Vue Web & Consoles | User Web Messenger, Admin Portal, Business Portal, Developer Portal | `frontend/web` | IMPLEMENTED_UNVERIFIED |
| 014 | Flutter Mobile | Clean Architecture (Domain, Data, Presentation) for Android & iOS | `mobile` | IMPLEMENTED_UNVERIFIED |
| 015 | Tauri Desktop | Windows & macOS native desktop client with Rust system tray & IPC | `desktop` | IMPLEMENTED_UNVERIFIED |
| 016 | API Contracts (REST/gRPC/WS) | Protobuf definitions & REST OpenAPI specs with strict idempotency | `proto/`, `backend/` | IMPLEMENTED_UNVERIFIED |
| 017 | PostgreSQL ERD & Migrations | Complete DDL with partitioned messages, indexes, FKs, constraints | `migrations/` | IMPLEMENTED_UNVERIFIED |
| 018 | WebSocket Real-time Gateway | Connection registry, heartbeat (ping/pong), device routing, broadcast | `backend/services/realtime` | IMPLEMENTED_UNVERIFIED |
| 019 | Infrastructure Components | Redis (cache/presence), NATS JetStream (pubsub), S3/MinIO, WebRTC SFU | `deploy/docker-compose` | IMPLEMENTED_UNVERIFIED |
| 020 | Containerization & CI/CD | Dockerfile, docker-compose, health checks, automated test scripts | `deploy/`, `scripts/` | IMPLEMENTED_UNVERIFIED |

## Phase 2: Registration, Authentication & Identity (021~030)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 021 | Multi-Factor Registration | Phone number verification, Email verification, Social OAuth (Google, Apple) | `identity-service` | IMPLEMENTED_UNVERIFIED |
| 022 | Cross-Platform Auth | Mobile, Web, PC login & secure logout with token revocation | `identity-service` | IMPLEMENTED_UNVERIFIED |
| 023 | JWT & Session Management | Access Token (short-lived), Refresh Token (rotation), session revoking | `identity-service` | IMPLEMENTED_UNVERIFIED |
| 024 | Multi-Device & QR Login | Primary mobile auth approves desktop QR login challenge via nonce exchange | `device-service` | IMPLEMENTED_UNVERIFIED |
| 025 | Device Key & Trust State | Device ID, public key storage, trust rating, active session termination | `device-service` | IMPLEMENTED_UNVERIFIED |
| 026 | Account Transfer & Backup | Encrypted cloud backup metadata, account migration PIN/QR | `user-service` | IMPLEMENTED_UNVERIFIED |
| 027 | Profile & Status Management | Display name, custom LineID, bio status message, profile avatar/banner | `user-service` | IMPLEMENTED_UNVERIFIED |
| 028 | Privacy & Search Controls | Allow search by ID, allow auto-add friends, allow message from strangers | `user-service` | IMPLEMENTED_UNVERIFIED |
| 029 | Social Graph Models | Friend (Pending, Accepted, Blocked, Hidden), contact matching | `social-graph-service` | IMPLEMENTED_UNVERIFIED |
| 030 | QR Code Friend Add | Dynamic personal QR codes with expiring cryptographic tokens | `social-graph-service` | IMPLEMENTED_UNVERIFIED |

## Phase 3: Core Messaging System (031~050)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 031 | Friend Operations | Search by ID/phone, add, accept, delete, hide, block with isolation | `social-graph-service` | IMPLEMENTED_UNVERIFIED |
| 032 | Phone Contact Sync | Hashed phone number bulk sync with existing registered users | `social-graph-service` | IMPLEMENTED_UNVERIFIED |
| 033 | Anti-Spam & Rate Limit | Rate limiter on friend requests, message quotas for unknown users | `api-gateway` | IMPLEMENTED_UNVERIFIED |
| 034 | Chat List & Metadata | 1:1, Group, OpenChat list with unread counter, pinned chats, archived | `chat-service` | IMPLEMENTED_UNVERIFIED |
| 035 | Real-time 1:1 Chat | Direct messaging with monotonic sequence IDs, sub-100ms delivery | `message-service` | IMPLEMENTED_UNVERIFIED |
| 036 | Group Chat & Roles | Group creation, invite links, roles (Owner, Admin, Member), kick, leave | `chat-service` | IMPLEMENTED_UNVERIFIED |
| 037 | Message Ordering & Idempotency | Client UUID deduplication, channel sequence numbers, causality | `message-service` | IMPLEMENTED_UNVERIFIED |
| 038 | Read Receipts & Multi-Device | Read cursors, unread count synchronization across all active devices | `message-service` | IMPLEMENTED_UNVERIFIED |
| 039 | Presence & Typing Indicators | Real-time online/offline status, ephemeral typing notifications | `presence-service` | IMPLEMENTED_UNVERIFIED |
| 040 | Reply, Quote & Forward | Nested reply references, original message quoting, multi-chat forwarding | `message-service` | IMPLEMENTED_UNVERIFIED |
| 041 | Delete, Recall & Edit | Delete for self, unsend (recall within 24h) for all, edit within 5min | `message-service` | IMPLEMENTED_UNVERIFIED |
| 042 | Reactions & Favorites | Emoji message reactions (like, heart, laugh), pin to conversation top | `message-service` | IMPLEMENTED_UNVERIFIED |
| 043 | Rich Message Types | Text, Emoji, Custom Stickers, Geolocation, VCard contacts, System notices | `message-service` | IMPLEMENTED_UNVERIFIED |
| 044 | Media Transmission | Image (thumbnails + full size), Video (HLS/MP4), Voice audio, Documents | `media-service` | IMPLEMENTED_UNVERIFIED |
| 045 | Large File Transfers | Chunked S3 multipart upload, resumable transfers, presigned URLs | `file-service` | IMPLEMENTED_UNVERIFIED |
| 046 | Polls, Schedules & Location | Group voting/polls, event scheduling with reminders, live location | `chat-service` | IMPLEMENTED_UNVERIFIED |
| 047 | Group Album & Notes | Photo albums per group, shared markdown notes with comments | `community-service` | IMPLEMENTED_UNVERIFIED |
| 048 | Unified Search | Full-text search across conversations, users, files via OpenSearch | `search-service` | IMPLEMENTED_UNVERIFIED |
| 049 | Keep / Personal Vault | "Chat with Myself", personal cloud notes, bookmarking links & files | `chat-service` | IMPLEMENTED_UNVERIFIED |
| 050 | Offline Queuing & Sync | Local SQLite caching, sync cursors, replay missing sequence on reconnect | `client-sync` | IMPLEMENTED_UNVERIFIED |

## Phase 4: Calling, E2EE, Push & Security (051~060)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 051 | Retention & Purge Policies | Ephemeral chat timers, data retention schedule, GDPR compliance purge | `audit-service` | IMPLEMENTED_UNVERIFIED |
| 052 | End-to-End Encryption (E2EE) | Signal Protocol / Double Ratchet style key exchange (Letter Sealing) | `crypto-lib` | IMPLEMENTED_UNVERIFIED |
| 053 | Multi-Device Key Sync | Identity keys, pre-keys, signed keys distribution and ratchet rotation | `device-service` | IMPLEMENTED_UNVERIFIED |
| 054 | Push Notifications | Unified notification dispatcher (FCM for Android, APNs for iOS, WebPush) | `notification-service` | IMPLEMENTED_UNVERIFIED |
| 055 | 1:1 WebRTC Voice Call | SDP offer/answer, ICE candidates, Opus audio codec, STUN/TURN | `call-signaling-service`| IMPLEMENTED_UNVERIFIED |
| 056 | 1:1 & Group Video Call | SFU media server integration, VP8/H.264 video, adaptive bitrate | `call-signaling-service`| IMPLEMENTED_UNVERIFIED |
| 057 | Screen Sharing | Desktop and mobile screen capture stream negotiation | `call-signaling-service`| IMPLEMENTED_UNVERIFIED |
| 058 | Call Quality & Reconnect | ICE restarts, TURN fallback, network jitter buffers, metrics collection | `call-signaling-service`| IMPLEMENTED_UNVERIFIED |
| 059 | Standalone Desktop Calling | Native Windows/macOS calling without mobile tethering | `desktop` | IMPLEMENTED_UNVERIFIED |
| 060 | Call Logs & Diagnostics | Call start/end timestamps, duration, termination reason, MOS rating | `call-signaling-service`| IMPLEMENTED_UNVERIFIED |

## Phase 5: Desktop, Mobile, Store & Community (061~070)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 061 | Desktop Chat UI/UX | Split-view navigation, quick search (Ctrl+K), multi-window support | `desktop` | IMPLEMENTED_UNVERIFIED |
| 062 | Native OS Integrations | System tray icon, unread badge, file drag-and-drop, OS startup toggle | `desktop/src-tauri` | IMPLEMENTED_UNVERIFIED |
| 063 | Full Mobile Experience | Swipe gestures, bottom navigation, in-app camera, contact pickers | `mobile` | IMPLEMENTED_UNVERIFIED |
| 064 | Accessibility & A11y | Screen reader labels, keyboard navigation, dynamic text scaling | Frontend / Mobile | IMPLEMENTED_UNVERIFIED |
| 065 | Sticker Engine & APNG/Lottie | Static PNG/WebP, APNG, Lottie animation rendering in chat bubble | Frontend / Mobile | IMPLEMENTED_UNVERIFIED |
| 066 | Sticker Store | Sticker packages, category browsing, preview sound/animation, purchase | `sticker-service` | IMPLEMENTED_UNVERIFIED |
| 067 | Themes & Customization | Custom chat wallpaper, color themes, bubble styles | Frontend / Mobile | IMPLEMENTED_UNVERIFIED |
| 068 | Premium Subscriptions | Subscription tiers, cloud storage expansion, exclusive badge perks | `billing-service` | IMPLEMENTED_UNVERIFIED |
| 069 | OpenChat / Communities | Public rooms, pseudonym profiles per room, member capacity up to 50k | `community-service` | IMPLEMENTED_UNVERIFIED |
| 070 | Community Moderation | Word filter blacklist, automated kick on spam, user report queue | `moderation-service` | IMPLEMENTED_UNVERIFIED |

## Phase 6: Business Official Accounts & Partner Center (071~080)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 071 | Official Account Lifecycle | Creation, verification badge, profile page, follower subscription | `business-account-service`| IMPLEMENTED_UNVERIFIED |
| 072 | Enterprise RBAC | Admin, Operator, Agent, Auditor roles with granular permission scopes | `business-account-service`| IMPLEMENTED_UNVERIFIED |
| 073 | Shared Customer Support Inbox | Multi-agent customer chat assignment, ticket status (Open/Pending/Closed) | `customer-support-service`| IMPLEMENTED_UNVERIFIED |
| 074 | Broadcast & Segment Messaging | Targeted push to followers by tag/demographics, scheduled broadcasts | `business-account-service`| IMPLEMENTED_UNVERIFIED |
| 075 | CRM & Tagging System | Customer profile attributes, custom tags, opt-in marketing consent logs | `customer-support-service`| IMPLEMENTED_UNVERIFIED |
| 076 | Rich Menu & Flex Messages | Interactive image maps, customizable JSON-based card layouts | `business-account-service`| IMPLEMENTED_UNVERIFIED |
| 077 | Bot & Human Handoff | Auto-responder keyword bot, fallback to live human support agent | `bot-service` | IMPLEMENTED_UNVERIFIED |
| 078 | Loyalty, Coupons & Stamps | Digital coupon barcode, stamp loyalty cards, customer surveys | `business-account-service`| IMPLEMENTED_UNVERIFIED |
| 079 | Business Analytics Dashboard | Message open rate, CTR, follower growth, agent response time metrics | `analytics-service` | IMPLEMENTED_UNVERIFIED |
| 080 | Enterprise Billing & Invoicing | Monthly plans, extra message billing, corporate tax invoices | `billing-service` | IMPLEMENTED_UNVERIFIED |

## Phase 7: Developer Platform, Ads & Creator Market (081~090)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 081 | Developer Console | App management, Channel ID & Secret, token issuance, OAuth scopes | `developer-platform` | IMPLEMENTED_UNVERIFIED |
| 082 | Messaging & Bot API | Webhook events, push message endpoint, reply token, rate limiting | `bot-service` | IMPLEMENTED_UNVERIFIED |
| 083 | Webhook Delivery Engine | HMAC-SHA256 signature verification, exponential backoff retries | `developer-platform` | IMPLEMENTED_UNVERIFIED |
| 084 | LINE Login / OAuth2 / OIDC | Identity provider for 3rd party apps, authorization code flow | `identity-service` | IMPLEMENTED_UNVERIFIED |
| 085 | Mini App Platform (LIFF style) | In-app webview SDK, JS bridge, context tokens, app review portal | `miniapp-service` | IMPLEMENTED_UNVERIFIED |
| 086 | Advertiser Center | Ad campaigns, targeting (geo, age, interest), creative assets, bidding | `ads-service` | IMPLEMENTED_UNVERIFIED |
| 087 | Payment & Order Processing | Multi-currency checkout, PG webhook integration, idempotency, refund | `payment-service` | IMPLEMENTED_UNVERIFIED |
| 088 | Creator Market Upload | Creators upload sticker/theme packages, guideline check, review queue | `creator-service` | IMPLEMENTED_UNVERIFIED |
| 089 | Creator Settlement & Ledger | Double-entry revenue share ledger, monthly payout requests, tax withholding | `creator-service` | IMPLEMENTED_UNVERIFIED |
| 090 | Partner Governance & Sanctions | Suspension of rogue apps/merchants, rate limit throttling, audit trails | `developer-platform` | IMPLEMENTED_UNVERIFIED |

## Phase 8: Admin Center, Security, Operations & Launch (091~100)

| ID | Item | Scope & Description | Target Component | Status |
|---|---|---|---|---|
| 091 | Super Admin Console | System metrics, user lookup, global feature flags, service topology | `frontend/web/admin` | IMPLEMENTED_UNVERIFIED |
| 092 | Moderation & Abuse Center | User reports, chat evidence inspection, ban/unban, appeal ticketing | `moderation-service` | IMPLEMENTED_UNVERIFIED |
| 093 | Security & Privacy Compliance | Strict RBAC enforcement, SQL injection immunity, data encryption at rest | Core Platform | IMPLEMENTED_UNVERIFIED |
| 094 | Load Testing & Benchmark | High-concurrency WebSocket test scripts, k6 scenarios | `tests/load` | IMPLEMENTED_UNVERIFIED |
| 095 | Observability & OpenTelemetry | Structured JSON logs, Prometheus `/metrics`, trace context propagation | `backend/pkg` | IMPLEMENTED_UNVERIFIED |
| 096 | Automated Test Suite | Unit tests, integration tests, E2E multi-client messaging tests | `backend/tests` | IMPLEMENTED_UNVERIFIED |
| 097 | Disaster Recovery & Backup | Automated DB backup scripts, WAL archiving, replica failover docs | `deploy/` | IMPLEMENTED_UNVERIFIED |
| 098 | API & Schema Documentation | Full ERD, OpenAPI 3.1 specifications, Protobuf contracts, AsyncAPI | `docs/`, `proto/` | IMPLEMENTED_UNVERIFIED |
| 099 | Security & Code Audit | Defect classification (P0~P3), zero high vulnerabilities guarantee | Quality Assurance | IMPLEMENTED_UNVERIFIED |
| 100 | Commercial Go/No-Go Gate | Formal validation of all 12 critical E2E scenarios across all clients | Release Readiness | IN_PROGRESS |
