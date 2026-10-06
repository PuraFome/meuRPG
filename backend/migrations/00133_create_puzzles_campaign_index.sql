-- +goose Up
-- The campaign's list of puzzles, newest first (MR-038).
CREATE INDEX IF NOT EXISTS puzzles_campaign_id_created_at_idx
    ON puzzles (campaign_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS puzzles_campaign_id_created_at_idx;
