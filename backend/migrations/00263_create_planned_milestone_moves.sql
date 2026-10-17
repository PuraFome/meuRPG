-- +goose Up
-- The idempotency keys of MoveMilestone. A move changes the positions of rows that exist and
-- creates none, so its key cannot live on a created row: one row here is one move that was made.
-- move_key is the app's key with its campaign in front (<campaign>:<key>) and move_hash the hash
-- of the request minus the key, so a retry replays and another request on the same key is
-- refused. Deleting the campaign deletes its keys (CASCADE). Opaque values, no personal data.
CREATE TABLE IF NOT EXISTS planned_milestone_moves (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    move_key TEXT NOT NULL,
    move_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, move_key)
);

-- +goose Down
DROP TABLE IF EXISTS planned_milestone_moves;
