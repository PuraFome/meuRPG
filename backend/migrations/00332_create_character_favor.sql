-- +goose Up
-- The Inspiration the master grants a player character (PHB 2014 ch. 4, outside SRD 5.1), called favor in the code because
-- "inspiration" already means Bardic Inspiration. A character has it or not, never more than one: the primary key is the
-- character, so a second grant finds the row. granted_by_user_id is the master that granted it. The row goes when the character or the campaign goes.
CREATE TABLE IF NOT EXISTS character_favor (
    character_id UUID PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    granted_by_user_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS character_favor_campaign_idx ON character_favor (campaign_id);

-- A roll held for the player's answer remembers that the player spent the Inspiration on it (the spend comes in a later slice).
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS favor_spent BOOL NOT NULL DEFAULT false;

-- The grant, the spend and the pass are session events, so the key of a retry finds what it did.
INSERT INTO session_event_kinds (kind) VALUES
    ('favor_granted'),
    ('favor_spent'),
    ('favor_passed')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('favor_granted', 'favor_spent', 'favor_passed');
DELETE FROM session_event_kinds WHERE kind IN ('favor_granted', 'favor_spent', 'favor_passed');
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS favor_spent;
DROP TABLE IF EXISTS character_favor;
