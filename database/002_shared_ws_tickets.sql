-- Apply to existing PostgreSQL databases before deploying the shared-ticket API.
-- New databases already include this table in 001_all_phases.sql.
BEGIN;
CREATE TABLE IF NOT EXISTS realtime.ws_tickets (
 token_hash TEXT PRIMARY KEY, user_id UUID NOT NULL REFERENCES identity.users(id),
 expires_at_unix BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS ws_tickets_expiry_idx ON realtime.ws_tickets(expires_at_unix);
COMMIT;
