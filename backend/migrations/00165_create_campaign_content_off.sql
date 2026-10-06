-- +goose Up
-- campaign_content_off is the master's "Opções para os jogadores" (MR-025, RN-23,
-- ADR-0018): one row per option the master switched OFF for the players of one
-- campaign. Everything is on by default, so an option with no row is on, and
-- switching it on again deletes the row.
--
-- content_key is the key of a class, subclass, race, subrace, background or spell
-- of the SRD ("class:wizard") or of the table's own content ("race:anao@mesa").
-- It is not a foreign key: the SRD keys live in the rules snapshot, not in a
-- table. The server checks that a key exists before it writes it; a row whose key
-- the content no longer has is ignored when the content is read.
--
-- Every write to this table also bumps the campaign's content revision
-- (campaign_content_state) in the same transaction, so the live content, cached
-- by (campaign, revision), is always read with the set that goes with it.
--
-- Deleting the campaign deletes its rows. Nothing here is personal data.
CREATE TABLE IF NOT EXISTS campaign_content_off (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    content_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, content_key),
    CONSTRAINT campaign_content_off_key_valid CHECK (char_length(content_key) BETWEEN 3 AND 80)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_content_off;
