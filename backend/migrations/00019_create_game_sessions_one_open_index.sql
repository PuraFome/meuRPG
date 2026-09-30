-- +goose Up
-- A campaign has at most one open game session (ended_at IS NULL). The
-- application checks first, inside the transaction, to answer with a clear
-- error; this index is what makes it true even when two starts race. It
-- also finds the open session quickly.
CREATE UNIQUE INDEX IF NOT EXISTS game_sessions_one_open_per_campaign
    ON game_sessions (campaign_id)
    WHERE ended_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS game_sessions_one_open_per_campaign;
