-- +goose Up
-- A hit rider whose saving throw a creature with Legendary Resistance makes (Stunning Strike, an Open Hand
-- technique, a creature's rider) is held for the master as a cast is: the list of 00262 plus 'hit_rider'.
-- Dropped first by name, so the migration can run twice.
ALTER TABLE reaction_holds
    DROP CONSTRAINT IF EXISTS reaction_holds_kind_valid,
    ADD CONSTRAINT reaction_holds_kind_valid CHECK (kind IN ('cast', 'attack', 'damage', 'contest', 'contest_answer', 'hide', 'hit_rider'));

-- +goose Down
DELETE FROM reaction_holds WHERE kind = 'hit_rider';
ALTER TABLE reaction_holds
    DROP CONSTRAINT IF EXISTS reaction_holds_kind_valid,
    ADD CONSTRAINT reaction_holds_kind_valid CHECK (kind IN ('cast', 'attack', 'damage', 'contest', 'contest_answer', 'hide'));
