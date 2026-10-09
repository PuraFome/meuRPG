-- +goose Up
-- hit_points_max_bonus is what Aid (Ajuda) adds to a creature's hit point
-- maximum (5 at the 2nd spell level, 5 more for each level above), kept until
-- the master ends the spell. The sheet's maximum is never stored (it comes from the
-- derived sheet), so the bonus is: a player's character keeps it with its vitals
-- and an NPC or a creature keeps it on its combatant, whose hp_max is already
-- the effective maximum, bonus included.
--
-- One statement for each table, with several parts, so re-running it is safe
-- (see 00036).
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS hit_points_max_bonus INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS character_vitals_hit_points_max_bonus_valid,
    ADD CONSTRAINT character_vitals_hit_points_max_bonus_valid CHECK (hit_points_max_bonus >= 0);

ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS hp_max_bonus INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS combatants_hp_max_bonus_valid,
    ADD CONSTRAINT combatants_hp_max_bonus_valid CHECK (hp_max_bonus >= 0);

-- extra_dice is how many weapon damage dice a feature adds to the damage of a
-- critical hit on top of dice_count (the barbarian's Brutal Critical: 1, 2 or 3).
-- They are always rolled, whatever the table's critical rule is.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS extra_dice INT4 NOT NULL DEFAULT 0;

-- jumped says the move that made an opportunity offer was a long jump, not a walk:
-- the master's panel reads "saltou" instead of "saiu".
ALTER TABLE opportunity_offers
    ADD COLUMN IF NOT EXISTS jumped BOOL NOT NULL DEFAULT false;

-- The kind of event for a lasting effect that ended (Escudo at the start of the
-- caster's turn, Ajuda when the master ends it).
INSERT INTO session_event_kinds (kind) VALUES ('combat_effect_ended')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_event_kinds WHERE kind = 'combat_effect_ended';

ALTER TABLE opportunity_offers
    DROP COLUMN IF EXISTS jumped;

ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS extra_dice;

ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_hp_max_bonus_valid,
    DROP COLUMN IF EXISTS hp_max_bonus;

ALTER TABLE character_vitals
    DROP CONSTRAINT IF EXISTS character_vitals_hit_points_max_bonus_valid,
    DROP COLUMN IF EXISTS hit_points_max_bonus;
