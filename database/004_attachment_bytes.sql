-- Existing PostgreSQL: apply before API rollout. Old metadata has no byte payload.
BEGIN;
CREATE TABLE IF NOT EXISTS chat.attachment_blobs (
 attachment_id UUID PRIMARY KEY REFERENCES chat.attachments(id) ON DELETE CASCADE,
 content BYTEA NOT NULL
);
COMMIT;
