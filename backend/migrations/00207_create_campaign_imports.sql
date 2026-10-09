-- +goose Up
-- campaign_imports is a package being uploaded in parts (MR-050), bound to the
-- account that began it. The parts themselves are blobs
-- (imports/<id>/parts/<n>); campaign_import_parts says which arrived.
--
-- A person has one upload at a time (a unique index on user_id, in 00208):
-- beginning another discards the first. The parts are kept 1 hour after the
-- last one and are then deleted by the cleanup, file first and row after.
-- user_id is a plain UUID without a foreign key, for the reason given on
-- campaign_exports: the sweeper takes the uploads of an account that went and deletes their parts.
--
-- fingerprint is what the app makes from the file (name, size, time) so that
-- choosing the same file again resumes the upload. file_name is only shown.
CREATE TABLE IF NOT EXISTS campaign_imports (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    file_name TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    total_bytes INT8 NOT NULL,
    part_size INT4 NOT NULL,
    part_count INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT campaign_imports_total_valid CHECK (total_bytes BETWEEN 1 AND 209715200),
    CONSTRAINT campaign_imports_parts_valid CHECK (part_size > 0 AND part_count BETWEEN 1 AND 1000),
    CONSTRAINT campaign_imports_name_length CHECK (char_length(file_name) BETWEEN 1 AND 120),
    CONSTRAINT campaign_imports_fingerprint_length CHECK (char_length(fingerprint) BETWEEN 1 AND 200)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_imports;
