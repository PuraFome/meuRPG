-- +goose Up
-- trap_damages is the damage a fired trap did to a player's character OUTSIDE a
-- combat (MR-035, Etapa 9, D5, RN-02): there is no combatant to hold a
-- pending_damages row, so the damage waits here for the master, who applies it
-- to the character's vitals (changing the amount first, if he wants) or
-- discards it. In a combat the same damage is a pending_damages row.
--
-- The server rolls it when the trap fires: dice_count d dice_sides + dice_bonus
-- (0 dice is a flat number, doubled for a critical hit of the trap's attack),
-- faces are what the dice showed, roll_total the roll before a half damage halves
-- it, and amount what lands (half, when the character passed the save and the
-- trap halves). status is 'rolled' (waiting), 'applied' or 'discarded';
-- applied_amount is what the master applied when it is not amount.
--
-- fire_id labels the damages of one firing. trap_point_id has no foreign key
-- (see 00110); the session and the character do, and deleting either deletes the
-- row. No personal data: fiction, numbers and IDs.
CREATE TABLE IF NOT EXISTS trap_damages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    trap_point_id UUID NOT NULL,
    fire_id UUID NOT NULL,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    critical BOOL NOT NULL DEFAULT false,
    dice_count INT4 NOT NULL,
    dice_sides INT4 NOT NULL,
    dice_bonus INT4 NOT NULL,
    damage_type TEXT NOT NULL,
    faces INT4[] NOT NULL DEFAULT '{}',
    roll_total INT4 NOT NULL,
    half BOOL NOT NULL DEFAULT false,
    amount INT4 NOT NULL,
    applied_amount INT4 NULL,
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT trap_damages_status_valid CHECK (status IN ('rolled', 'applied', 'discarded')),
    CONSTRAINT trap_damages_dice_valid CHECK (dice_count BETWEEN 0 AND 100 AND dice_sides BETWEEN 0 AND 100 AND dice_bonus BETWEEN -1000 AND 1000),
    CONSTRAINT trap_damages_amount_valid CHECK (amount >= 0 AND (applied_amount IS NULL OR applied_amount >= 0))
);

-- +goose Down
DROP TABLE IF EXISTS trap_damages;
