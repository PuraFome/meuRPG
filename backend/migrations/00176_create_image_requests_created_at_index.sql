-- +goose Up
-- The server's daily cap on generated images (MR-039) counts the day's rows
-- across all campaigns: without this index that is a scan of the whole table.
CREATE INDEX IF NOT EXISTS image_requests_created_at_idx
    ON image_requests (created_at);

-- +goose Down
DROP INDEX IF EXISTS image_requests_created_at_idx;
