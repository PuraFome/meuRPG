-- +goose Up
-- The areas a spell keeps on the map while it lasts (Moonbeam, Spirit Guardians, Cloudkill, Web): one
-- row for each. A zone belongs to a combat and to the combatant that cast it (owner_id), and goes
-- away with either. squares are the grid squares it covers now, [[col,row],...], kept so that the
-- movement hooks only look them up; origin_col and origin_row are its point of origin. The save and
-- the damage are the cast's, kept at cast time (save_dc, damage_dice at the slot level, the type the
-- caster picked), so a trigger needs no other read. cast_id is the id of the cast that made it: the
-- undo of the cast finds the zone by it. ends_round is the round whose start of the owner's turn ends
-- it (NULL: it ends with the concentration or is dismissed); concentration says the owner's
-- concentration holds it. hit_marks keeps, for each creature, the turn it was last hit on
-- ({combatant id: "round:turn leader"}): a creature is hit once on a turn.
CREATE TABLE IF NOT EXISTS spell_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    spell_key TEXT NOT NULL,
    cast_id TEXT NOT NULL,
    slot_level INT4 NOT NULL,
    origin_col INT4 NOT NULL,
    origin_row INT4 NOT NULL,
    squares JSONB NOT NULL,
    save_ability TEXT NOT NULL DEFAULT '',
    save_dc INT4 NOT NULL DEFAULT 0,
    damage_dice TEXT NOT NULL DEFAULT '',
    damage_type TEXT NOT NULL DEFAULT '',
    concentration BOOL NOT NULL,
    started_round INT4 NOT NULL,
    ends_round INT4 NULL,
    hit_marks JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT spell_zones_squares_valid CHECK (jsonb_typeof(squares) = 'array' AND jsonb_array_length(squares) <= 1024),
    CONSTRAINT spell_zones_hit_marks_valid CHECK (jsonb_typeof(hit_marks) = 'object'),
    CONSTRAINT spell_zones_rounds_valid CHECK (started_round >= 0 AND (ends_round IS NULL OR ends_round >= started_round)),
    CONSTRAINT spell_zones_origin_valid CHECK (origin_col >= 0 AND origin_row >= 0)
);

CREATE INDEX IF NOT EXISTS spell_zones_encounter_id_idx ON spell_zones (encounter_id);
CREATE INDEX IF NOT EXISTS spell_zones_owner_id_idx ON spell_zones (owner_id);

-- The events a zone leaves: a creature caught by one (spell_zone_triggered: ids, the saving throw and
-- the damage) and an area that changed place (spell_zone_moved: the zone's spell and caster).
INSERT INTO session_event_kinds (kind) VALUES
    ('spell_zone_triggered'),
    ('spell_zone_moved')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('spell_zone_triggered', 'spell_zone_moved');
DELETE FROM session_event_kinds WHERE kind IN ('spell_zone_triggered', 'spell_zone_moved');
DROP TABLE IF EXISTS spell_zones;
