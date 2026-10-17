-- +goose Up
-- A contest roll, the answer to a contest and a Hide are held for a Cutting Words window as a cast, an
-- attack and a damage already are: the list of 00221 plus 'contest', 'contest_answer' and 'hide'. Dropped
-- first by name, so the migration can run twice.
ALTER TABLE reaction_holds
    DROP CONSTRAINT IF EXISTS reaction_holds_kind_valid,
    ADD CONSTRAINT reaction_holds_kind_valid CHECK (kind IN ('cast', 'attack', 'damage', 'contest', 'contest_answer', 'hide'));

-- +goose Down
DELETE FROM reaction_holds WHERE kind IN ('contest', 'contest_answer', 'hide');
ALTER TABLE reaction_holds
    DROP CONSTRAINT IF EXISTS reaction_holds_kind_valid,
    ADD CONSTRAINT reaction_holds_kind_valid CHECK (kind IN ('cast', 'attack', 'damage'));
