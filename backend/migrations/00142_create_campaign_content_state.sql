-- +goose Up
-- campaign_content_state holds the campaign's content revision: the number of
-- writes to its table content (campaign_content), going up by one in the same
-- transaction as each write. No row is revision 0, a campaign with no content of
-- its own. ADR-0018 put it in campaigns.content_revision; it is a table of the
-- characters module instead, so that module never writes a campaigns column
-- (a cross-module write) and reads the revision in its own queries.
--
-- Everything that reads the rules content of a campaign reads this row, in the
-- caller's transaction when it has one, and the live content is cached by
-- (campaign, revision). Every write to the content starts by bumping this row, so
-- two writes of one campaign run one after the other. A write of a sheet (create,
-- update, level-up) reads the content again inside its own transaction and checks
-- the sheet again if the revision moved since the handler first read it, so it is
-- ordered against a content write.
--
-- An archive or an unarchive also bumps it (the cache must see them), but they are
-- not a change of the entry: campaign_content.revision and updated_at stay.
--
-- Deleting the campaign deletes its row.
CREATE TABLE IF NOT EXISTS campaign_content_state (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    revision INT4 NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT campaign_content_state_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_content_state;
