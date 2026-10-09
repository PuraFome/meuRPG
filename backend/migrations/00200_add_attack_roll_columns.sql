-- +goose Up
-- pending_damages: the damage of a hit by parts. parts are the lines the hit offers
-- (the weapon, the extras a player may add with the condition of each, the automatic
-- lines), part_rolls what each rolled, steps the resistance, vulnerability and
-- immunity steps of the damage and after_steps the damage after them. landed_before is
-- the hit points (and temporary ones) an NPC had before the damage landed on it at
-- once, for the master's correction of an extra; NULL otherwise. All are JSON of the
-- damage module; a pending damage that is one roll (a spell's, a trap's) keeps '[]'.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS parts JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS part_rolls JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS steps JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS landed_before JSONB NULL,
    ADD COLUMN IF NOT EXISTS after_steps INT4 NULL;

-- combatants: attacked_hostile and took_damage are what decide whether a rage ends at
-- the end of the barbarian's turn (SRD 5.1, Barbarian, Rage): it attacked a hostile
-- creature since its last turn, it took damage since then. They go back to false when
-- its own turn starts. rage_end_pending says the turn waits for the player's answer to
-- "A sua fúria vai acabar?". condition_sources say where a condition comes from and
-- when it ends (Stunned by a Stunning Strike), as JSON. sneak_attack_turn and
-- colossus_slayer_turn are the turn ("<round>:<combatant on turn>") in which the
-- feature last added its damage: both are once per turn.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS attacked_hostile BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS took_damage BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS rage_end_pending BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS condition_sources JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS sneak_attack_turn TEXT NULL,
    ADD COLUMN IF NOT EXISTS colossus_slayer_turn TEXT NULL;

-- A hint try rolled with advantage or disadvantage keeps the other d20, which of the
-- two was rolled first (d20 is the one that counts) and the mode.
ALTER TABLE puzzle_hint_tries
    ADD COLUMN IF NOT EXISTS d20_b INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS counted INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS roll_mode TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE puzzle_hint_tries
    DROP COLUMN IF EXISTS roll_mode,
    DROP COLUMN IF EXISTS counted,
    DROP COLUMN IF EXISTS d20_b;

ALTER TABLE combatants
    DROP COLUMN IF EXISTS colossus_slayer_turn,
    DROP COLUMN IF EXISTS sneak_attack_turn,
    DROP COLUMN IF EXISTS condition_sources,
    DROP COLUMN IF EXISTS rage_end_pending,
    DROP COLUMN IF EXISTS took_damage,
    DROP COLUMN IF EXISTS attacked_hostile;

ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS after_steps,
    DROP COLUMN IF EXISTS landed_before,
    DROP COLUMN IF EXISTS steps,
    DROP COLUMN IF EXISTS part_rolls,
    DROP COLUMN IF EXISTS parts;
