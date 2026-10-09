-- +goose Up
-- The parts of an upload that arrived: part_number from 1, and the length
-- received (every part but the last has the upload's part_size). They go with
-- the upload.
CREATE TABLE IF NOT EXISTS campaign_import_parts (
    import_id UUID NOT NULL REFERENCES campaign_imports (id) ON DELETE CASCADE,
    part_number INT4 NOT NULL,
    byte_size INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (import_id, part_number),
    CONSTRAINT campaign_import_parts_number_valid CHECK (part_number >= 1),
    CONSTRAINT campaign_import_parts_size_valid CHECK (byte_size >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_import_parts;
