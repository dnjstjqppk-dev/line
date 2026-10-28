-- Migration 003: Polls and reactions parity
ALTER TABLE chat.polls ADD COLUMN IF NOT EXISTS multiple BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE chat.polls ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS poll_options_poll_id_idx ON chat.poll_options(poll_id);
CREATE INDEX IF NOT EXISTS poll_votes_option_user_idx ON chat.poll_votes(option_id, user_id);
CREATE INDEX IF NOT EXISTS message_reactions_msg_idx ON chat.message_reactions(message_id);

