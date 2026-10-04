-- +goose Up
-- A creature of a character is a combatant of the new kind 'creature'
-- (MR-037, Etapa 9). Its character_id is the OWNER's character (the column
-- stays NOT NULL, and deleting the character deletes the creature's combatant
-- with it); creature_id says which creature it is, and the rows of kind
-- 'creature' are the only ones that have it. monster_key, summon_attack and
-- summon_group_id are copied from the creature when it joins, as speed_ft is,
-- so a combat never reads another table to know what a combatant may do.
-- user_id is the owner's player, so the same "may this player act for it"
-- check as for a character works (the master acts for anyone).
--
-- A creature has its hit points on the combatant, as an NPC does
-- (combatants_hit_points_valid now lets a 'creature' carry them) and never
-- makes death saves: at 0 it is defeated.
--
-- Initiative: creatures that share a summon_group_id share one initiative roll
-- (RN-18, the rule of Conjurar Animais; ours for Animar os Mortos too).
--
-- dismissed hides the combatant of a creature whose concentration ended: it
-- leaves the order, the map and the turns, but the row (its id, hit points,
-- conditions, the log lines about it) stays, so the master's undo brings the same
-- combatant back.
--
-- One statement with several parts, so re-running it is safe (see 00036 and
-- 00086: an inline REFERENCES would add the foreign key again every time).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS creature_id UUID NULL,
    DROP CONSTRAINT IF EXISTS combatants_creature_id_fkey,
    ADD CONSTRAINT combatants_creature_id_fkey FOREIGN KEY (creature_id) REFERENCES character_creatures (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS monster_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS summon_attack TEXT NULL,
    ADD COLUMN IF NOT EXISTS summon_group_id UUID NULL,
    ADD COLUMN IF NOT EXISTS dismissed BOOL NOT NULL DEFAULT false,
    DROP CONSTRAINT IF EXISTS combatants_kind_valid,
    ADD CONSTRAINT combatants_kind_valid CHECK (kind IN ('player', 'npc', 'creature')),
    DROP CONSTRAINT IF EXISTS combatants_hit_points_valid,
    ADD CONSTRAINT combatants_hit_points_valid CHECK (
        (kind = 'player' AND hp_current IS NULL AND hp_max IS NULL AND hp_temp IS NULL)
        OR (
            kind IN ('npc', 'creature') AND hp_max IS NOT NULL AND hp_current IS NOT NULL AND hp_temp IS NOT NULL
            AND hp_max >= 1 AND hp_current BETWEEN 0 AND hp_max AND hp_temp >= 0
        )
    ),
    DROP CONSTRAINT IF EXISTS combatants_creature_valid,
    ADD CONSTRAINT combatants_creature_valid CHECK (
        (kind = 'creature') = (creature_id IS NOT NULL)
        AND (kind = 'creature') = (monster_key IS NOT NULL)
        AND (kind = 'creature') = (summon_attack IS NOT NULL)
        AND (kind = 'creature') = (summon_group_id IS NOT NULL)
        AND (summon_attack IS NULL OR summon_attack IN ('none', 'reaction', 'full'))
    );

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_creature_valid,
    DROP CONSTRAINT IF EXISTS combatants_hit_points_valid,
    ADD CONSTRAINT combatants_hit_points_valid CHECK (
        (kind = 'player' AND hp_current IS NULL AND hp_max IS NULL AND hp_temp IS NULL)
        OR (
            kind = 'npc' AND hp_max IS NOT NULL AND hp_current IS NOT NULL AND hp_temp IS NOT NULL
            AND hp_max >= 1 AND hp_current BETWEEN 0 AND hp_max AND hp_temp >= 0
        )
    ),
    DROP CONSTRAINT IF EXISTS combatants_kind_valid,
    ADD CONSTRAINT combatants_kind_valid CHECK (kind IN ('player', 'npc')),
    DROP COLUMN IF EXISTS dismissed,
    DROP COLUMN IF EXISTS summon_group_id,
    DROP COLUMN IF EXISTS summon_attack,
    DROP COLUMN IF EXISTS monster_key,
    DROP CONSTRAINT IF EXISTS combatants_creature_id_fkey,
    DROP COLUMN IF EXISTS creature_id;
