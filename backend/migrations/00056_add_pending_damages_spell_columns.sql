-- +goose Up
-- A pending damage can come from a spell (MR-014, Etapa 6). cast_id groups the
-- pending damages of one cast: an area spell's damage is one roll for the whole
-- cast, so rolling one of them settles the others. healing says it is a heal,
-- not a damage (the table's amount is what the target regains). half says the
-- target saved and the spell halves the damage. applied_amount is what the
-- master applied when it is not the rolled amount (ApplyPendingDamage.amount;
-- RN-02: the master has the last word). attack_total is the attack roll's total
-- while the hit waits for the target's reaction (Escudo), so the comparison with
-- the new armor class does not need the roll again. roll_total is the roll's
-- total (the dice plus the modifier) before a half damage halves it: amount is
-- what lands, and a physical roll has no faces to add up again.
--
-- One statement with several parts, so re-running it is safe (see 00036). No
-- foreign key for cast_id: it is only a label that the pending damages of a
-- cast share, and the combat's events say what the cast was.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS cast_id UUID NULL,
    ADD COLUMN IF NOT EXISTS healing BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS half BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS applied_amount INT4 NULL,
    ADD COLUMN IF NOT EXISTS attack_total INT4 NULL,
    ADD COLUMN IF NOT EXISTS roll_total INT4 NULL,
    DROP CONSTRAINT IF EXISTS pending_damages_applied_amount_valid,
    ADD CONSTRAINT pending_damages_applied_amount_valid CHECK (applied_amount IS NULL OR applied_amount >= 0);

-- +goose Down
ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_applied_amount_valid,
    DROP COLUMN IF EXISTS roll_total,
    DROP COLUMN IF EXISTS attack_total,
    DROP COLUMN IF EXISTS applied_amount,
    DROP COLUMN IF EXISTS half,
    DROP COLUMN IF EXISTS healing,
    DROP COLUMN IF EXISTS cast_id;
