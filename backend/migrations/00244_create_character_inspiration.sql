-- +goose Up
-- The Bardic Inspiration die a character holds out of a running combat (SRD 5.1, Bard 1):
-- its size (6, 8, 10 or 12), the character of the bard that gave it, and the game time it
-- has left (10 minutes, 600 seconds; it only moves when the master moves game time, like
-- character_effects.seconds_left). One row per character: a creature has at most one die at
-- a time. When a combat takes the character in the die moves onto its combatant
-- (combatants.inspiration_*), and back when the combat ends or the character leaves it.
CREATE TABLE IF NOT EXISTS character_inspiration (
    character_id UUID PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    source_character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    sides INT4 NOT NULL CHECK (sides IN (6, 8, 10, 12)),
    seconds_left INT4 NOT NULL CHECK (seconds_left BETWEEN 1 AND 600),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS character_inspiration_campaign_idx ON character_inspiration (campaign_id);

-- A roll of a character out of a combat that waits for the player's answer about the die
-- (the d20 was rolled; the player decides before the result is shown). request is the request
-- that made the roll and faces the d20 it rolled, so the answer settles that very roll.
CREATE TABLE IF NOT EXISTS inspiration_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('scene_check', 'group_check')),
    idempotency_key UUID NOT NULL,
    request BYTEA NOT NULL,
    faces INT4[] NOT NULL,
    modifier INT4 NOT NULL,
    total INT4 NOT NULL,
    counted INT4 NOT NULL,
    physical BOOL NOT NULL,
    answer_key UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS inspiration_holds_character_idx ON inspiration_holds (character_id);

-- The give is a session event, so the key of a retry finds what it did.
INSERT INTO session_event_kinds (kind) VALUES ('inspiration_given') ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'inspiration_given';
DELETE FROM session_event_kinds WHERE kind = 'inspiration_given';
DROP TABLE IF EXISTS inspiration_holds;
DROP TABLE IF EXISTS character_inspiration;
