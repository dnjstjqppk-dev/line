-- Global Messenger: independently designed schema covering phases 0-9.
-- PostgreSQL 16+. Execute against a fresh database. Passwords/tokens never stored as plaintext.
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS social;
CREATE SCHEMA IF NOT EXISTS chat;
CREATE SCHEMA IF NOT EXISTS realtime;
CREATE SCHEMA IF NOT EXISTS calling;
CREATE SCHEMA IF NOT EXISTS crypto;
CREATE SCHEMA IF NOT EXISTS community;
CREATE SCHEMA IF NOT EXISTS business;
CREATE SCHEMA IF NOT EXISTS developer;
CREATE SCHEMA IF NOT EXISTS creator;
CREATE SCHEMA IF NOT EXISTS commerce;
CREATE SCHEMA IF NOT EXISTS ads;
CREATE SCHEMA IF NOT EXISTS safety;
CREATE SCHEMA IF NOT EXISTS ops;

CREATE TABLE identity.users (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), email TEXT NOT NULL, password_hash TEXT NOT NULL,
 display_name TEXT NOT NULL CHECK(length(display_name) BETWEEN 1 AND 120),
 handle TEXT UNIQUE, phone_e164 TEXT UNIQUE, avatar_url TEXT, bio TEXT NOT NULL DEFAULT '',
 role TEXT NOT NULL DEFAULT 'user' CHECK(role IN ('user','support','moderator','platform_admin')),
 state TEXT NOT NULL DEFAULT 'active' CHECK(state IN ('active','suspended','deleted')),
 locale TEXT NOT NULL DEFAULT 'ko', created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ,
 CONSTRAINT email_canonical CHECK(email=lower(email))
);
CREATE UNIQUE INDEX users_email_uq ON identity.users(lower(email));
CREATE INDEX users_display_idx ON identity.users(lower(display_name));
CREATE TABLE identity.devices (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 name TEXT NOT NULL, platform TEXT NOT NULL, trusted BOOLEAN NOT NULL DEFAULT false,
 last_seen_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE identity.sessions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 device_id UUID REFERENCES identity.devices(id), refresh_token_hash TEXT NOT NULL UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ, last_seen_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE identity.verification_challenges (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID REFERENCES identity.users(id),
 destination TEXT NOT NULL, purpose TEXT NOT NULL, code_hash TEXT NOT NULL,
 attempts INT NOT NULL DEFAULT 0, expires_at TIMESTAMPTZ NOT NULL, consumed_at TIMESTAMPTZ
);
CREATE TABLE identity.consents (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 purpose TEXT NOT NULL, version TEXT NOT NULL, granted BOOLEAN NOT NULL,
 recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE identity.user_settings (
 user_id UUID PRIMARY KEY REFERENCES identity.users(id), searchable_by_email BOOLEAN NOT NULL DEFAULT false,
 searchable_by_phone BOOLEAN NOT NULL DEFAULT false, show_read_receipts BOOLEAN NOT NULL DEFAULT true,
 allow_friend_requests BOOLEAN NOT NULL DEFAULT true, theme TEXT NOT NULL DEFAULT 'system', updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE social.friendships (
 user_id UUID NOT NULL REFERENCES identity.users(id), friend_id UUID NOT NULL REFERENCES identity.users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id, friend_id), CHECK(user_id<>friend_id)
);
CREATE TABLE social.blocks (
 user_id UUID NOT NULL REFERENCES identity.users(id), blocked_id UUID NOT NULL REFERENCES identity.users(id),
 reason TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id,blocked_id), CHECK(user_id<>blocked_id)
);
CREATE TABLE social.friend_requests (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), sender_id UUID NOT NULL REFERENCES identity.users(id),
 recipient_id UUID NOT NULL REFERENCES identity.users(id), status TEXT NOT NULL DEFAULT 'pending'
 CHECK(status IN ('pending','accepted','rejected','cancelled')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), decided_at TIMESTAMPTZ,
 CHECK(sender_id<>recipient_id)
);
CREATE INDEX friend_requests_to_idx ON social.friend_requests(recipient_id,status);
CREATE TABLE social.invite_links (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), creator_id UUID NOT NULL REFERENCES identity.users(id),
 token_hash TEXT NOT NULL UNIQUE, target TEXT NOT NULL, expires_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ
);

CREATE TABLE chat.conversations (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), kind TEXT NOT NULL CHECK(kind IN ('direct','group','self')),
 title TEXT, image_url TEXT, created_by UUID NOT NULL REFERENCES identity.users(id),
 last_sequence BIGINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ
);
CREATE TABLE chat.conversation_members (
 conversation_id UUID NOT NULL REFERENCES chat.conversations(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner','admin','member')),
 last_read_sequence BIGINT NOT NULL DEFAULT 0, joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 left_at TIMESTAMPTZ, muted_until TIMESTAMPTZ, PRIMARY KEY(conversation_id,user_id)
);
CREATE INDEX member_by_user_idx ON chat.conversation_members(user_id,joined_at DESC);
CREATE TABLE chat.messages (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), conversation_id UUID NOT NULL REFERENCES chat.conversations(id),
 sender_id UUID NOT NULL REFERENCES identity.users(id), client_id TEXT NOT NULL,
 sequence BIGINT NOT NULL CHECK(sequence>0), kind TEXT NOT NULL DEFAULT 'text',
 body TEXT NOT NULL DEFAULT '', ciphertext BYTEA, reply_to UUID REFERENCES chat.messages(id),
 metadata JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 edited_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ,
 UNIQUE(conversation_id, sequence), UNIQUE(conversation_id,sender_id,client_id),
 CHECK(length(body)<=100000)
);
CREATE INDEX messages_conversation_seq_idx ON chat.messages(conversation_id, sequence DESC);
CREATE TABLE chat.message_receipts (
 message_id UUID NOT NULL REFERENCES chat.messages(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 delivered_at TIMESTAMPTZ, read_at TIMESTAMPTZ, PRIMARY KEY(message_id,user_id)
);
CREATE TABLE chat.message_reactions (
 message_id UUID NOT NULL REFERENCES chat.messages(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 reaction TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(message_id,user_id,reaction)
);
CREATE TABLE chat.attachments (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), message_id UUID REFERENCES chat.messages(id),
 owner_id UUID NOT NULL REFERENCES identity.users(id), object_key TEXT NOT NULL UNIQUE,
 mime_type TEXT NOT NULL, original_name TEXT NOT NULL, size_bytes BIGINT NOT NULL CHECK(size_bytes>=0),
 checksum_sha256 TEXT, encryption_mode TEXT NOT NULL DEFAULT 'none', status TEXT NOT NULL DEFAULT 'pending',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE chat.attachment_blobs (
 attachment_id UUID PRIMARY KEY REFERENCES chat.attachments(id) ON DELETE CASCADE,
 content BYTEA NOT NULL
);
CREATE TABLE chat.pinned_messages (
 conversation_id UUID NOT NULL REFERENCES chat.conversations(id), message_id UUID NOT NULL REFERENCES chat.messages(id),
 pinned_by UUID NOT NULL REFERENCES identity.users(id), pinned_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(conversation_id,message_id)
);
CREATE TABLE chat.albums (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), conversation_id UUID NOT NULL REFERENCES chat.conversations(id),
 title TEXT NOT NULL, created_by UUID NOT NULL REFERENCES identity.users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE chat.album_items (
 album_id UUID NOT NULL REFERENCES chat.albums(id), attachment_id UUID NOT NULL REFERENCES chat.attachments(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(album_id,attachment_id)
);
CREATE TABLE chat.polls (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), conversation_id UUID NOT NULL REFERENCES chat.conversations(id),
 question TEXT NOT NULL, multiple BOOLEAN NOT NULL DEFAULT FALSE, created_by UUID NOT NULL REFERENCES identity.users(id),
 closes_at TIMESTAMPTZ, closed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE chat.poll_options (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), poll_id UUID NOT NULL REFERENCES chat.polls(id), label TEXT NOT NULL);
CREATE TABLE chat.poll_votes (
 option_id UUID NOT NULL REFERENCES chat.poll_options(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 voted_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(option_id,user_id)
);
CREATE TABLE chat.saved_messages (
 user_id UUID NOT NULL REFERENCES identity.users(id), message_id UUID NOT NULL REFERENCES chat.messages(id),
 saved_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id,message_id)
);

CREATE TABLE realtime.presence (
 user_id UUID NOT NULL REFERENCES identity.users(id), device_id UUID NOT NULL REFERENCES identity.devices(id),
 last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id,device_id)
);
CREATE TABLE realtime.ws_tickets (
 token_hash TEXT PRIMARY KEY, user_id UUID NOT NULL REFERENCES identity.users(id),
 expires_at_unix BIGINT NOT NULL
);
CREATE INDEX ws_tickets_expiry_idx ON realtime.ws_tickets(expires_at_unix);
CREATE TABLE realtime.push_tokens (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), device_id UUID NOT NULL REFERENCES identity.devices(id),
 provider TEXT NOT NULL, token_ciphertext BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 revoked_at TIMESTAMPTZ
);
CREATE TABLE realtime.notification_settings (
 user_id UUID PRIMARY KEY REFERENCES identity.users(id), push_enabled BOOLEAN NOT NULL DEFAULT true,
 show_preview BOOLEAN NOT NULL DEFAULT false, quiet_start TIME, quiet_end TIME, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE realtime.delivery_outbox (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, conversation_id UUID NOT NULL REFERENCES chat.conversations(id),
 message_id UUID NOT NULL REFERENCES chat.messages(id), event_type TEXT NOT NULL,
 payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), published_at TIMESTAMPTZ
);
CREATE INDEX delivery_outbox_pending_idx ON realtime.delivery_outbox(created_at) WHERE published_at IS NULL;

CREATE TABLE calling.calls (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), conversation_id UUID REFERENCES chat.conversations(id),
 initiated_by UUID NOT NULL REFERENCES identity.users(id), kind TEXT NOT NULL CHECK(kind IN ('voice','video')),
 state TEXT NOT NULL DEFAULT 'ringing', started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 connected_at TIMESTAMPTZ, ended_at TIMESTAMPTZ, end_reason TEXT
);
CREATE TABLE calling.participants (
 call_id UUID NOT NULL REFERENCES calling.calls(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 joined_at TIMESTAMPTZ, left_at TIMESTAMPTZ, state TEXT NOT NULL DEFAULT 'invited', PRIMARY KEY(call_id,user_id)
);
CREATE TABLE calling.quality_samples (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, call_id UUID NOT NULL REFERENCES calling.calls(id),
 user_id UUID NOT NULL REFERENCES identity.users(id), sample_time TIMESTAMPTZ NOT NULL DEFAULT now(),
 rtt_ms INT, jitter_ms INT, packet_loss_ratio REAL
);
CREATE TABLE crypto.device_keys (
 device_id UUID PRIMARY KEY REFERENCES identity.devices(id), identity_public_key BYTEA NOT NULL,
 signed_prekey BYTEA NOT NULL, signed_prekey_signature BYTEA NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE crypto.prekeys (
 device_id UUID NOT NULL REFERENCES identity.devices(id), key_id INTEGER NOT NULL, public_key BYTEA NOT NULL,
 consumed_at TIMESTAMPTZ, PRIMARY KEY(device_id,key_id)
);
CREATE TABLE crypto.key_change_events (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, user_id UUID NOT NULL REFERENCES identity.users(id),
 device_id UUID NOT NULL REFERENCES identity.devices(id), changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE crypto.encrypted_backups (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 encrypted_object_key TEXT NOT NULL, key_derivation_metadata JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE community.rooms (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), owner_id UUID NOT NULL REFERENCES identity.users(id),
 name TEXT NOT NULL, description TEXT, visibility TEXT NOT NULL DEFAULT 'public', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE community.members (
 room_id UUID NOT NULL REFERENCES community.rooms(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 nickname TEXT, role TEXT NOT NULL DEFAULT 'member', joined_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(room_id,user_id)
);
CREATE TABLE community.posts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), room_id UUID NOT NULL REFERENCES community.rooms(id),
 author_id UUID NOT NULL REFERENCES identity.users(id), body TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ
);

CREATE TABLE business.organizations (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), owner_id UUID NOT NULL REFERENCES identity.users(id),
 name TEXT NOT NULL, legal_name TEXT, country TEXT, verification_state TEXT NOT NULL DEFAULT 'pending',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE business.memberships (
 organization_id UUID NOT NULL REFERENCES business.organizations(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 role TEXT NOT NULL CHECK(role IN ('owner','admin','agent','analyst','billing')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(organization_id,user_id)
);
CREATE TABLE business.official_accounts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL REFERENCES business.organizations(id),
 name TEXT NOT NULL, description TEXT, avatar_url TEXT, status TEXT NOT NULL DEFAULT 'pending',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE business.contacts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 user_id UUID NOT NULL REFERENCES identity.users(id), opted_in BOOLEAN NOT NULL DEFAULT false,
 tags JSONB NOT NULL DEFAULT '[]'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(official_account_id,user_id)
);
CREATE TABLE business.support_tickets (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 contact_id UUID NOT NULL REFERENCES business.contacts(id), agent_id UUID REFERENCES identity.users(id),
 status TEXT NOT NULL DEFAULT 'open', priority TEXT NOT NULL DEFAULT 'normal', internal_note TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE business.rich_menus (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 title TEXT NOT NULL, definition JSONB NOT NULL, enabled BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE business.auto_replies (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 trigger_type TEXT NOT NULL, match_value TEXT, response JSONB NOT NULL, active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE business.campaigns (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 name TEXT NOT NULL, message_template JSONB NOT NULL, status TEXT NOT NULL DEFAULT 'draft',
 scheduled_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE business.campaign_recipients (
 campaign_id UUID NOT NULL REFERENCES business.campaigns(id), contact_id UUID NOT NULL REFERENCES business.contacts(id),
 delivery_state TEXT NOT NULL DEFAULT 'pending', delivered_at TIMESTAMPTZ, PRIMARY KEY(campaign_id,contact_id)
);
CREATE TABLE business.coupons (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), official_account_id UUID NOT NULL REFERENCES business.official_accounts(id),
 code TEXT NOT NULL, expires_at TIMESTAMPTZ, max_redemptions INT, UNIQUE(official_account_id,code)
);
CREATE TABLE business.coupon_redemptions (
 coupon_id UUID NOT NULL REFERENCES business.coupons(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 redeemed_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(coupon_id,user_id)
);

CREATE TABLE developer.organizations (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), owner_id UUID NOT NULL REFERENCES identity.users(id),
 name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE developer.memberships (
 organization_id UUID NOT NULL REFERENCES developer.organizations(id), user_id UUID NOT NULL REFERENCES identity.users(id),
 role TEXT NOT NULL DEFAULT 'developer', PRIMARY KEY(organization_id,user_id)
);
CREATE TABLE developer.apps (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL REFERENCES developer.organizations(id),
 name TEXT NOT NULL, client_id TEXT NOT NULL UNIQUE, client_secret_hash TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'development', redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
 scopes TEXT[] NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE developer.webhooks (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), app_id UUID NOT NULL REFERENCES developer.apps(id),
 endpoint_url TEXT NOT NULL, signing_secret_ciphertext BYTEA NOT NULL, enabled BOOLEAN NOT NULL DEFAULT true,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE developer.webhook_deliveries (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), webhook_id UUID NOT NULL REFERENCES developer.webhooks(id),
 payload JSONB NOT NULL, attempts INT NOT NULL DEFAULT 0, last_status_code INT,
 next_attempt_at TIMESTAMPTZ, succeeded_at TIMESTAMPTZ
);
CREATE TABLE developer.api_keys (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), app_id UUID NOT NULL REFERENCES developer.apps(id),
 key_hash TEXT NOT NULL UNIQUE, scope TEXT[] NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), revoked_at TIMESTAMPTZ
);
CREATE TABLE developer.miniapps (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), app_id UUID NOT NULL REFERENCES developer.apps(id),
 manifest JSONB NOT NULL, origin TEXT NOT NULL, review_state TEXT NOT NULL DEFAULT 'pending', published_at TIMESTAMPTZ
);

CREATE TABLE creator.creators (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL UNIQUE REFERENCES identity.users(id),
 display_name TEXT NOT NULL, payout_status TEXT NOT NULL DEFAULT 'not_configured', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE creator.sticker_packs (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), creator_id UUID NOT NULL REFERENCES creator.creators(id),
 title TEXT NOT NULL, description TEXT, price_minor BIGINT NOT NULL DEFAULT 0 CHECK(price_minor>=0),
 currency CHAR(3) NOT NULL DEFAULT 'KRW', status TEXT NOT NULL DEFAULT 'draft', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE creator.stickers (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), pack_id UUID NOT NULL REFERENCES creator.sticker_packs(id),
 object_key TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'static', ordinal INT NOT NULL, UNIQUE(pack_id,ordinal)
);
CREATE TABLE creator.themes (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), creator_id UUID NOT NULL REFERENCES creator.creators(id),
 title TEXT NOT NULL, theme_asset_key TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'draft'
);
CREATE TABLE creator.reviews (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), pack_id UUID REFERENCES creator.sticker_packs(id), theme_id UUID REFERENCES creator.themes(id),
 moderator_id UUID REFERENCES identity.users(id), outcome TEXT NOT NULL, remarks TEXT, reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK ((pack_id IS NOT NULL)::int + (theme_id IS NOT NULL)::int = 1)
);

CREATE TABLE commerce.products (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), type TEXT NOT NULL, resource_id UUID,
 name TEXT NOT NULL, currency CHAR(3) NOT NULL DEFAULT 'KRW', price_minor BIGINT NOT NULL CHECK(price_minor>=0),
 active BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE commerce.orders (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), buyer_id UUID NOT NULL REFERENCES identity.users(id),
 state TEXT NOT NULL DEFAULT 'pending', currency CHAR(3) NOT NULL, total_minor BIGINT NOT NULL CHECK(total_minor>=0),
 idempotency_key TEXT NOT NULL UNIQUE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE commerce.order_items (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), order_id UUID NOT NULL REFERENCES commerce.orders(id),
 product_id UUID NOT NULL REFERENCES commerce.products(id), quantity INT NOT NULL CHECK(quantity>0), unit_price_minor BIGINT NOT NULL CHECK(unit_price_minor>=0)
);
CREATE TABLE commerce.payments (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), order_id UUID NOT NULL REFERENCES commerce.orders(id),
 provider TEXT NOT NULL, provider_transaction_id TEXT UNIQUE, state TEXT NOT NULL DEFAULT 'initiated',
 amount_minor BIGINT NOT NULL CHECK(amount_minor>=0), currency CHAR(3) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE commerce.refunds (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), payment_id UUID NOT NULL REFERENCES commerce.payments(id),
 amount_minor BIGINT NOT NULL CHECK(amount_minor>0), state TEXT NOT NULL DEFAULT 'requested', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE commerce.ledger_entries (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), event_id UUID NOT NULL, account_code TEXT NOT NULL,
 direction TEXT NOT NULL CHECK(direction IN ('debit','credit')), amount_minor BIGINT NOT NULL CHECK(amount_minor>=0),
 currency CHAR(3) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ledger_event_idx ON commerce.ledger_entries(event_id);
CREATE TABLE commerce.subscriptions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 product_id UUID NOT NULL REFERENCES commerce.products(id), provider TEXT NOT NULL,
 provider_subscription_id TEXT, state TEXT NOT NULL DEFAULT 'pending', current_period_end TIMESTAMPTZ
);
CREATE TABLE creator.settlements (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), creator_id UUID NOT NULL REFERENCES creator.creators(id),
 period_start DATE NOT NULL, period_end DATE NOT NULL, amount_minor BIGINT NOT NULL,
 currency CHAR(3) NOT NULL, state TEXT NOT NULL DEFAULT 'pending', CHECK(period_end>=period_start)
);

CREATE TABLE ads.advertisers (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL REFERENCES business.organizations(id),
 name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending'
);
CREATE TABLE ads.campaigns (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), advertiser_id UUID NOT NULL REFERENCES ads.advertisers(id),
 name TEXT NOT NULL, objective TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'draft',
 daily_budget_minor BIGINT NOT NULL DEFAULT 0 CHECK(daily_budget_minor>=0),
 currency CHAR(3) NOT NULL DEFAULT 'KRW', starts_at TIMESTAMPTZ, ends_at TIMESTAMPTZ
);
CREATE TABLE ads.ad_groups (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), campaign_id UUID NOT NULL REFERENCES ads.campaigns(id),
 name TEXT NOT NULL, targeting JSONB NOT NULL DEFAULT '{}'::jsonb, bid_minor BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE ads.creatives (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), ad_group_id UUID NOT NULL REFERENCES ads.ad_groups(id),
 media_object_key TEXT, headline TEXT NOT NULL, destination_url TEXT NOT NULL, review_state TEXT NOT NULL DEFAULT 'pending'
);
CREATE TABLE ads.events (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, creative_id UUID NOT NULL REFERENCES ads.creatives(id),
 event_type TEXT NOT NULL, occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(), attribution_token_hash TEXT
);

CREATE TABLE safety.reports (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), reporter_id UUID REFERENCES identity.users(id),
 subject_user_id UUID REFERENCES identity.users(id), subject_conversation_id UUID REFERENCES chat.conversations(id),
 category TEXT NOT NULL, details TEXT, state TEXT NOT NULL DEFAULT 'open', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE safety.moderation_actions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), report_id UUID REFERENCES safety.reports(id),
 subject_user_id UUID REFERENCES identity.users(id), actor_id UUID NOT NULL REFERENCES identity.users(id),
 action TEXT NOT NULL, reason TEXT, expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE safety.appeals (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), action_id UUID NOT NULL REFERENCES safety.moderation_actions(id),
 user_id UUID NOT NULL REFERENCES identity.users(id), explanation TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'pending',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE ops.audit_logs (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, actor_user_id UUID REFERENCES identity.users(id),
 action TEXT NOT NULL, target_type TEXT, target_id TEXT, data JSONB NOT NULL DEFAULT '{}'::jsonb,
 ip_address INET, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_created_idx ON ops.audit_logs(created_at DESC);
CREATE TABLE ops.feature_flags (
 key TEXT PRIMARY KEY, enabled BOOLEAN NOT NULL DEFAULT false, config JSONB NOT NULL DEFAULT '{}'::jsonb,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE ops.outbox_events (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, aggregate_type TEXT NOT NULL,
 aggregate_id UUID NOT NULL, event_type TEXT NOT NULL, payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), published_at TIMESTAMPTZ
);
CREATE TABLE ops.privacy_requests (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES identity.users(id),
 kind TEXT NOT NULL CHECK(kind IN ('export','delete','rectify')), status TEXT NOT NULL DEFAULT 'requested',
 requested_at TIMESTAMPTZ NOT NULL DEFAULT now(), completed_at TIMESTAMPTZ
);
