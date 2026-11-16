-- +goose Up
-- The infusions of an Artificer (SRD 5.1, the Artificer's Infuse Item): one row for each item an infusion is on.
-- A record names the infusion (infusion_key, the key of the option in the table feature's content), the item and the
-- character that holds it. The artificer made it (artificer_character_id); holder_character_id is the character
-- whose sheet has the item, which is the artificer itself for now.
-- item_kind is 'gear' (item_key is a gear key of the holder's sheet) or 'object' (item_key is 'object:<record id>',
-- so two named objects never share a key; object_name is what the table called it). parameter is the damage type
-- an infusion asks for, or ''. attuned says the holder attuned to the item.
-- since is when the infusion began or last moved: the oldest live record of an artificer is the first to end when
-- a new infusion is over the limit. A move sets it again.
-- materialises_id links to the item or creature an infusion made. Nothing writes it yet.
--
-- A record is kept when it ends (ended_at, ended_reason): 'ended' (the owner or the master ended it), 'replaced'
-- (it was over the limit and a new infusion ended it), 'swapped' (given up at a level-up), 'item_gone' (the piece
-- left the holder's sheet) and 'taken' (another infusion took the item of a record that is not valid now).
-- object_name is free text written by a player: it is in docs/privacy.md's inventory.
--
-- Deleting the campaign or either character deletes its records.
CREATE TABLE IF NOT EXISTS character_infusions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    artificer_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    holder_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    infusion_key TEXT NOT NULL,
    item_kind TEXT NOT NULL,
    item_key TEXT NOT NULL,
    object_name TEXT NOT NULL DEFAULT '',
    parameter TEXT NOT NULL DEFAULT '',
    attuned BOOLEAN NOT NULL DEFAULT false,
    since TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NULL,
    ended_reason TEXT NULL,
    materialises_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT character_infusions_item_kind_valid CHECK (item_kind IN ('gear', 'object')),
    CONSTRAINT character_infusions_ended_valid CHECK (
        (ended_at IS NULL) = (ended_reason IS NULL)
        AND (ended_reason IS NULL OR ended_reason IN ('ended', 'replaced', 'swapped', 'item_gone', 'taken'))
    )
);

-- One live record for each item of a holder.
CREATE UNIQUE INDEX IF NOT EXISTS character_infusions_one_live_item_idx
    ON character_infusions (holder_character_id, item_key) WHERE ended_at IS NULL;

-- The live records of an artificer, oldest first.
CREATE INDEX IF NOT EXISTS character_infusions_live_by_artificer_idx
    ON character_infusions (artificer_character_id, since, id) WHERE ended_at IS NULL;

-- The infusions are session events, so the key of a retry finds what it did. Nothing writes them yet.
INSERT INTO session_event_kinds (kind) VALUES
    ('infusion_infused'),
    ('infusion_moved'),
    ('infusion_ended')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('infusion_infused', 'infusion_moved', 'infusion_ended');
DELETE FROM session_event_kinds WHERE kind IN ('infusion_infused', 'infusion_moved', 'infusion_ended');
-- CASCADE: CockroachDB keeps a unique index as a constraint and refuses to drop it without it.
DROP INDEX IF EXISTS character_infusions_one_live_item_idx CASCADE;
DROP INDEX IF EXISTS character_infusions_live_by_artificer_idx;
DROP TABLE IF EXISTS character_infusions;
