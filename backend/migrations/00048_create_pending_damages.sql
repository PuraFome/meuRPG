-- +goose Up
-- pending_damages are the damage of an attack that hit (MR-012, MR-014): the
-- attack roll opens one, RollDamage rolls it, and the target's kind decides
-- the rest. An NPC target takes it at once ('applied'); a player's character
-- waits for the master ('rolled') until he applies or discards it (RN-02).
-- It is a row, not a field of the attack's event, so a retry or a reload finds
-- it and the master sees what is still open when he passes a turn.
--
-- status is 'awaiting_roll' (hit, damage not rolled), 'rolled' (rolled, for a
-- player's character, waiting for the master), 'applied' or 'discarded'.
-- dice_count, dice_sides and dice_bonus are the damage to roll, copied from the
-- attacker's sheet when the attack hits (a sheet changed later never moves a
-- roll already open); dice_count is already doubled for a critical hit. 0 dice
-- is a flat number (dice_bonus alone). damage_type is a content key such as
-- 'damage-type:fire'.
--
-- faces are what the app rolled (empty for a physical roll or while
-- awaiting_roll), physical says the player typed the sum, amount is the damage
-- (NULL until rolled). created_at and resolved_at are timestamps.
--
-- Deleting the encounter, or either combatant, deletes the row. There is no
-- index by attacker or target: only removing a combatant looks them up that
-- way. No personal data: fiction, numbers and IDs.
CREATE TABLE IF NOT EXISTS pending_damages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    attacker_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    attack_key TEXT NOT NULL,
    status TEXT NOT NULL,
    critical BOOL NOT NULL,
    dice_count INT4 NOT NULL,
    dice_sides INT4 NOT NULL,
    dice_bonus INT4 NOT NULL,
    damage_type TEXT NOT NULL,
    faces INT4[] NOT NULL DEFAULT '{}',
    physical BOOL NOT NULL DEFAULT false,
    amount INT4 NULL,
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT pending_damages_status_valid CHECK (status IN ('awaiting_roll', 'rolled', 'applied', 'discarded')),
    CONSTRAINT pending_damages_dice_valid CHECK (dice_count BETWEEN 0 AND 100 AND dice_sides BETWEEN 0 AND 100 AND dice_bonus BETWEEN -1000 AND 1000),
    CONSTRAINT pending_damages_amount_valid CHECK (amount IS NULL OR amount >= 0),
    CONSTRAINT pending_damages_key_length CHECK (char_length(attack_key) BETWEEN 1 AND 100)
);

-- +goose Down
DROP TABLE IF EXISTS pending_damages;
