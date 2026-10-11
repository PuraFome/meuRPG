-- +goose Up
-- The riders a monk's hit offers (Open Hand Technique on a Flurry of Blows hit,
-- Stunning Strike on a melee hit): one row per offer, open until the player picks
-- or the round ends. effect_no_reaction is what an effect that takes the reaction
-- away (the Open Hand rider) leaves on a combatant, worked out with the others.
CREATE TABLE IF NOT EXISTS hit_riders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    attacker_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('open_hand', 'stunning_strike')),
    round INT4 NOT NULL,
    choice TEXT NULL,
    used BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS hit_riders_encounter_id_idx ON hit_riders (encounter_id);

ALTER TABLE combatants ADD COLUMN IF NOT EXISTS effect_no_reaction BOOL NOT NULL DEFAULT false;

INSERT INTO session_event_kinds (kind) VALUES ('hit_rider') ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'hit_rider';
DELETE FROM session_event_kinds WHERE kind = 'hit_rider';
ALTER TABLE combatants DROP COLUMN IF EXISTS effect_no_reaction;
DROP TABLE IF EXISTS hit_riders;
