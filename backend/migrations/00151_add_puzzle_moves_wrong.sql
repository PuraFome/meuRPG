-- +goose Up
-- True for a move that was wrong (MR-038, RN-27): a wrong riddle answer, a wrong bell, a
-- wrong cipher message. These are the moves that spend the player's attempt and fire
-- the trap of "Ao errar". The lights, the lock and the pillars have no wrong move.
--
-- The move column holds the player's own typed text for the riddle and the cipher (the
-- protojson of the move): it is kept as long as the session, like every move, and goes
-- with it (ON DELETE CASCADE from the run).
ALTER TABLE puzzle_moves
    ADD COLUMN IF NOT EXISTS wrong BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE puzzle_moves
    DROP COLUMN IF EXISTS wrong;
