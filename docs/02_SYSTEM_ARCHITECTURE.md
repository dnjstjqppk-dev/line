# LINE-Grade Global Messenger: System Architecture

## 1. High-Level Architecture Overview

```
                      +------------------------------------------+
                      |         Clients & Edge Layer             |
                      |  - Mobile App (Flutter: Android/iOS)     |
                      |  - Desktop App (Tauri 2 + Vue 3: Win/Mac)|
                      |  - Web Client (Vue 3 + TypeScript)       |
                      |  - Business & Admin Consoles (Vue 3)     |
                      +--------------------+---------------------+
                                           | HTTPS / WSS / WebRTC
                                           v
                      +------------------------------------------+
                      |           API & WS Gateway               |
                      |  - Reverse Proxy & TLS Termination       |
                      |  - JWT Authentication & RBAC Enforcement |
                      |  - Rate Limiting & DoS Shield            |
                      |  - WebSocket Session Registry & Routing  |
                      +--------------------+---------------------+
                                           |
         +---------------------------------+---------------------------------+
         |                                 |                                 |
         v                                 v                                 v
+------------------+             +--------------------+            +--------------------+
|  Identity & Auth |             |  Messaging Engine  |            |  Social Graph &    |
|  - Registration  |             |  - Conversation    |            |  User Profiles     |
|  - Password/SMS  |             |  - Message Ordering|            |  - Friends, Blocks |
|  - Token Refresh |             |  - Read Cursors    |            |  - Search, Bio     |
|  - Multi-Device  |             |  - Ephemeral Sync  |            |  - QR Code Tokens  |
+--------+---------+             +---------+----------+            +---------+----------+
         |                                 |                                 |
         +---------------------------------+---------------------------------+
                                           |
    +------------------+-------------------+-------------------+------------------+
    |                  |                   |                   |                  |
    v                  v                   v                   v                  v
+----------+     +------------+      +------------+      +-----------+      +-----------+
| Media &  |     | WebRTC     |      | Business & |      | Developer |      | Moderation|
| Files S3 |     | Signaling  |      | Customer   |      | & Bot API |      | & Admin   |
| Engine   |     | & SFU Hub  |      | Support    |      | Platform  |      | Console   |
+----+-----+     +-----+------+      +-----+------+      +-----+-----+      +-----+-----+
     |                 |                   |                   |                  |
     +-----------------+-------------------+-------------------+------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |          Data & Transport Tier           |
                      |  - PostgreSQL 16 (Relational DB & WAL)   |
                      |  - Redis 7 (Session, Cache, Presence)   |
                      |  - NATS JetStream (Async Event Broker)   |
                      |  - MinIO / S3 (Encrypted Object Storage) |
                      |  - OpenSearch (Full-Text Search Engine)  |
                      +------------------------------------------+
```

## 2. Microservice Boundaries & Responsibilities

| Service Name | Protocol | Responsibility |
|---|---|---|
| `api-gateway` | HTTP, WSS | External entry point, routing, TLS, rate limiting, auth middleware |
| `identity-service` | REST/gRPC | Auth, registration, token issuance, refresh rotation, OAuth2 |
| `user-service` | REST/gRPC | Profile management, privacy settings, avatars, user search |
| `device-service` | REST/gRPC | Device pairing, QR challenge login, device trust, E2EE keys |
| `social-graph-service`| REST/gRPC | Friend requests, friendships, blocks, contact sync, QR friend codes |
| `chat-service` | REST/gRPC | 1:1, Group, and Channel metadata, membership management |
| `message-service` | REST/gRPC | Monotonic sequence message storage, delivery receipts, recall |
| `realtime-service` | WSS/gRPC | WebSocket persistent connections, presence, typing, live events |
| `presence-service` | Redis PubSub | Ephemeral online/offline state, last seen timestamps |
| `media-service` | REST | Pre-signed upload URLs, image resizing, video transcoding |
| `file-service` | REST | Multipart upload, download authorization, virus scanning hooks |
| `call-signaling-service`| WSS/gRPC | WebRTC SDP offer/answer, ICE candidate forwarding, SFU integration |
| `notification-service` | Worker | Push notifications via FCM, APNs, Web Push |
| `search-service` | REST | Message, friend, and document indexing via OpenSearch |
| `community-service` | REST | OpenChat, community boards, shared albums, group notes |
| `business-account-service`| REST | Official account lifecycle, follower broadcasts, Rich Menu |
| `customer-support-service`| REST | Multi-agent support inbox, conversation ticketing, agent assignment|
| `bot-service` | Webhook/REST| Webhook delivery, bot auto-replies, keyword triggers |
| `developer-platform` | REST | Developer apps, API keys, OAuth scopes, sandbox environments |
| `miniapp-service` | REST | Mini app directory, JS SDK bridges, review workflow |
| `sticker-service` | REST | Sticker package catalog, animations, purchase entitlements |
| `payment-service` | REST | Payment processing, checkout, refund handling, PG integration |
| `billing-service` | REST | Subscription tiers, invoicing, usage-based billing |
| `ads-service` | REST | Ad campaign management, creative banners, impressions tracking |
| `creator-service` | REST | Creator portal, package submission, revenue share settlement |
| `moderation-service` | REST | Abuse reporting, automated text filters, ban/freeze operations |
| `audit-service` | gRPC/Worker | Immutable security audit logs, compliance reports |
| `analytics-service` | Worker/REST| Real-time KPIs, engagement stats, official account metrics |

## 3. Real-Time Message Synchronization Protocol

1. **Client Send**: Client sends `POST /api/v1/messages` with `conversation_id`, `client_msg_id` (UUID for deduplication), and `content`.
2. **Persistence & Ordering**: `message-service` runs in an ACID transaction:
   - Increments conversation sequence generator (`seq = seq + 1`).
   - Persists message to PostgreSQL with `(conversation_id, seq)` unique key.
   - Publishes `MessageCreated` event to NATS JetStream / Redis channel.
3. **Real-Time Distribution**: `realtime-service` receives the event:
   - Queries active recipients in conversation.
   - For every connected device session of recipients (and sender's other devices), delivers packet via WebSocket.
4. **Delivery & Read Confirmation**:
   - Recipient client sends `ACK` packet over WebSocket.
   - Recipient client updates read cursor (`POST /api/v1/messages/read`).
   - Read cursor event is pushed to sender's devices in real time.
5. **Offline & Recovery**:
   - Clients maintain `last_seen_seq` locally.
   - Upon reconnect, client sends `sync_request` with `last_seen_seq`.
   - Server streams all messages where `seq > last_seen_seq`.
