-- +goose Up
-- A spell cast with a bonus action leaves no other spell for the turn except a
-- cantrip with a casting time of 1 action (SRD 5.1, Casting Time). spell_cast says
-- a spell other than that cantrip and other than a bonus action one was cast in
-- the combatant's current turn, and bonus_spell_cast that a bonus action spell
-- was; the start of the next turn clears both.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS spell_cast BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS bonus_spell_cast BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN IF EXISTS bonus_spell_cast,
    DROP COLUMN IF EXISTS spell_cast;
