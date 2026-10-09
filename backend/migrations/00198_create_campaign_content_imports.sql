-- +goose Up
-- campaign_content_imports keeps the idempotency key of ImportTableContent (MR-025), one row
-- per applied import that carried a key. An import writes many entries and updates others,
-- so the key cannot live on one entry's row (campaign_content.create_key is CreateTableEntry's):
-- the row keeps the key, the hash of the whole request and the answer, so a retry with the
-- same key and the same request returns what the first call answered and writes nothing,
-- and the same key with another request is refused.
--
--   - create_key: the campaign's ID, a colon and the 1 to 64 characters the app chose once
--     for the action; unique, so one key is one import.
--   - create_hash: the hash of the request minus its key.
--   - response: the ImportTableContentResponse (protobuf) the first call answered, at most
--     the 300 entries of a table with their violations-free outcomes.
--
-- Deleting the campaign deletes its imports. The key and the hash are opaque; the response
-- holds names the master wrote, like campaign_content (docs/privacy.md's inventory).
CREATE TABLE IF NOT EXISTS campaign_content_imports (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    create_key TEXT NOT NULL,
    create_hash TEXT NOT NULL,
    response BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, create_key),
    CONSTRAINT campaign_content_imports_response_size CHECK (octet_length(response) <= 1048576)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_content_imports;
