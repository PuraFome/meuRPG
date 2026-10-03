-- +goose Up
-- A player's notes in a campaign, newest first: what ListNotes asks, and what
-- the 300-note limit counts.
CREATE INDEX IF NOT EXISTS player_notes_campaign_id_author_user_id_updated_at_idx
    ON player_notes (campaign_id, author_user_id, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS player_notes_campaign_id_author_user_id_updated_at_idx;
