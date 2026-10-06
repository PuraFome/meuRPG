-- +goose Up
-- campaign_content is the table's own rules content (MR-025, RN-23, ADR-0018):
-- the classes, subclasses, races, subraces, backgrounds and spells that the
-- master writes for one campaign. One row per entry.
--
-- content_key is "<kind>:<slug>@mesa" ("class:cavaleiro-runico@mesa"). The server
-- makes it from the name when the entry is created, and it never changes, not
-- even when the name does: sheets store the key. kind is the key's prefix.
-- name_pt is the Portuguese name (the table's entries have no English name).
-- data is the protojson (proto field names) of the kind's message in
-- rules/v1/table_content.proto, without the key and the archive mark. revision
-- is the campaign's content revision (campaign_content_state) at the entry's
-- last change, and updated_at when: a sheet saved at an older revision is told
-- the entry changed, if the change left it with new issues. Archiving and
-- unarchiving are not changes: they set archived_at and leave revision and
-- updated_at alone.
--
-- Nothing is deleted: archived_at retires an entry. Sheets that use it keep
-- working, but it is not offered as a new choice (ADR-0018, section 7).
--
-- Limits: 300 entries per campaign and 64 KiB of data per entry are checked by
-- the server (a refusal the editor shows). The CHECK on data is the last line of
-- defense, with room for the JSONB text being longer than the compact protojson.
--
-- Deleting the campaign deletes its content. Names and texts are written by the
-- master: they are in docs/privacidade.md's inventory.
CREATE TABLE IF NOT EXISTS campaign_content (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    content_key TEXT NOT NULL,
    kind TEXT NOT NULL,
    name_pt TEXT NOT NULL,
    data JSONB NOT NULL,
    revision INT4 NOT NULL,
    archived_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, content_key),
    CONSTRAINT campaign_content_kind_valid CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell')),
    CONSTRAINT campaign_content_key_valid CHECK (
        content_key LIKE kind || ':%@mesa' AND char_length(content_key) BETWEEN 9 AND 80
    ),
    CONSTRAINT campaign_content_name_length CHECK (char_length(name_pt) BETWEEN 1 AND 80),
    CONSTRAINT campaign_content_data_object CHECK (jsonb_typeof(data) = 'object'),
    CONSTRAINT campaign_content_data_size CHECK (octet_length(data::TEXT) <= 131072),
    CONSTRAINT campaign_content_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_content;
