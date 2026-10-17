-- +goose Up
-- One line per create_key (which carries the campaign and the author): the unique index is what makes
-- the idempotency of SendSessionMessage hold when two calls with the same key race (see 00180).
CREATE UNIQUE INDEX IF NOT EXISTS session_messages_create_key_idx
    ON session_messages (create_key) WHERE create_key IS NOT NULL;
-- The cascades and the SET NULLs that come from the campaign, the account, the characters and the
-- gallery scan the table by these columns.
CREATE INDEX IF NOT EXISTS session_messages_campaign_id_idx ON session_messages (campaign_id);
CREATE INDEX IF NOT EXISTS session_messages_author_user_id_idx ON session_messages (author_user_id) WHERE author_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS session_messages_image_id_idx ON session_messages (image_id) WHERE image_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS session_messages_speaker_image_id_idx ON session_messages (speaker_image_id) WHERE speaker_image_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS session_messages_speaker_ref_idx ON session_messages (speaker_ref) WHERE speaker_ref IS NOT NULL;
-- The voices made today, for the daily cap.
CREATE INDEX IF NOT EXISTS session_messages_audio_made_at_idx ON session_messages (audio_made_at) WHERE audio_made_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS session_message_readers_user_id_idx ON session_message_readers (user_id);

-- +goose Down
DROP INDEX IF EXISTS session_message_readers_user_id_idx;
DROP INDEX IF EXISTS session_messages_audio_made_at_idx;
DROP INDEX IF EXISTS session_messages_speaker_ref_idx;
DROP INDEX IF EXISTS session_messages_speaker_image_id_idx;
DROP INDEX IF EXISTS session_messages_image_id_idx;
DROP INDEX IF EXISTS session_messages_author_user_id_idx;
DROP INDEX IF EXISTS session_messages_campaign_id_idx;
DROP INDEX IF EXISTS session_messages_create_key_idx;
