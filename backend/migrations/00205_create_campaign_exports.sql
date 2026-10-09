-- +goose Up
-- campaign_exports is one export of a campaign as a package (MR-050): the
-- zip is written on the server, to the blob store, and kept 24 hours.
--
-- The life of a row:
--   - running: the zip is being written (percent moves). A row that stays
--     running for long was cut off by a restart and is failed by the next read.
--   - done: the file is in the store under blob_key, until expires_at (24
--     hours after it finished).
--   - failed, canceled: nothing is kept; the row says which, until expires_at.
--
-- id is 32 random hex digits (128 bits): it is the download address and the
-- only secret a download needs besides the master's session, which the route
-- checks again. campaign_id and requested_by are plain UUIDs without foreign
-- keys on purpose: a row must outlive the campaign or the account long enough
-- for the cleanup to find its file and delete it (a cascade would drop the row
-- and leave the file where nothing names it). The cleanup deletes the file
-- first and the row after, and it also takes the rows whose campaign or account
-- is gone.
--
-- create_key and create_hash are the idempotency key of StartCampaignExport
-- (scoped to the caller) and the hash of its request.
CREATE TABLE IF NOT EXISTS campaign_exports (
    id TEXT PRIMARY KEY,
    campaign_id UUID NOT NULL,
    requested_by UUID NOT NULL,
    state TEXT NOT NULL,
    percent INT4 NOT NULL DEFAULT 0,
    failure TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL DEFAULT '',
    blob_key TEXT NOT NULL DEFAULT '',
    byte_size INT8 NOT NULL DEFAULT 0,
    entry_count INT4 NOT NULL DEFAULT 0,
    create_key TEXT NULL,
    create_hash TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT campaign_exports_id_shape CHECK (id ~ '^[0-9a-f]{32}$'),
    CONSTRAINT campaign_exports_state_valid CHECK (state IN ('running', 'done', 'failed', 'canceled')),
    CONSTRAINT campaign_exports_failure_valid CHECK (failure IN ('', 'too_big', 'interrupted')),
    CONSTRAINT campaign_exports_percent_valid CHECK (percent BETWEEN 0 AND 100)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_exports;
