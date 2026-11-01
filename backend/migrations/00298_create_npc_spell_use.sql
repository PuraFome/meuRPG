-- +goose Up
-- What a hand-made NPC spent of its spell slots and per-day uses lasts until the table's long rest (SRD 5.1,
-- "Resting": slots and uses of "per day" features come back on a long rest). The count of a fight lives on the
-- combatant (combatants.slots_used, 00221); this table keeps the count of the NPC itself, so a new combat starts
-- with it. The keys are the combatant's: "1" to "9" for a slot level, "pact", "res:<resource>" for a use.
-- A monster of a combat (the combat_only NPC the app keeps for a creature) never has a row: each fight is a new
-- individual and starts with all its slots and uses.
-- The highest count any copy of the NPC holds in a combat is kept here as it changes, and each copy gets it when it
-- joins a combat; the long rest deletes the rows of the campaign. The row goes with its character.
--
-- Production is live: the table only adds, so running the migration twice is safe.
CREATE TABLE IF NOT EXISTS npc_spell_use (
    character_id UUID NOT NULL PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    slots_used JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS npc_spell_use;
