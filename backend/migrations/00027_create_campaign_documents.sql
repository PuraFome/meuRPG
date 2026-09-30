-- +goose Up
-- campaign_documents holds each campaign's document (MR-018): the Markdown
-- text the master writes to prepare the game, with links to maps, character
-- sheets and gallery images. Only the master reads and writes it
-- (CampaignDocumentService, package campaigns). The server never parses the
-- text: the web app renders it (docs/arquitetura.md).
--
-- One document per campaign: campaign_id is the primary key. A campaign
-- with no row has an empty document at revision 0. The first save inserts
-- the row at revision 1, and each save after that raises it by one, but
-- only if the revision is still the one the master read, so two tabs
-- saving at once never overwrite each other: the second one gets
-- `aborted`.
--
-- body is at most 200 KiB: 204,800 bytes of UTF-8. octet_length counts
-- bytes, the same unit the API checks (campaigns.MaxDocumentBytes). The
-- application validates the text first; the CHECK is the last line of
-- defense.
--
-- Deleting the campaign deletes its document. updated_by is who saved it
-- last; deleting that account keeps the document with no editor (SET
-- NULL), because the document belongs to the campaign, not to whoever
-- edited it. There is no index on updated_by: only an account deletion
-- looks rows up by it, and scanning this small table then is fine, as for
-- campaigns.created_by.
CREATE TABLE IF NOT EXISTS campaign_documents (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    revision INT4 NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT campaign_documents_body_size CHECK (octet_length(body) <= 204800),
    CONSTRAINT campaign_documents_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_documents;
