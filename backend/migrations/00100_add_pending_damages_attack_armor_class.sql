-- +goose Up
-- attack_armor_class is the armor class the attack that opened this damage was
-- compared with: the target's sheet, its ac_bonus and the cover it had against
-- the attacker (MR-034, D4). Escudo (+5) compares the kept total with this
-- number plus 5, so a hit that cover had already raised is not judged against
-- the bare sheet. NULL for a damage opened before this column (Escudo then
-- falls back to the sheet's armor class). It never goes to a player (RN-20).
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS attack_armor_class INT4 NULL;

-- +goose Down
ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS attack_armor_class;
