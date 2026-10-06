-- +goose Up
-- What slice 10.7b adds to a puzzle's run (MR-038, RN-27).
--
-- A round is what the limits of "Ao errar" count in: it begins when the master shows
-- the puzzle, restarts it ("Recomeçar", "Gerar outro começo") or shows a closed one
-- again. round_started_at is when it began (NULL while the run is only prepared, not
-- shown) and round_start_seq the run's moves_made at that moment, so the moves of the
-- round are the ones of puzzle_moves after that seq. The time limit counts from
-- round_started_at; nothing is stored when it runs out: it is worked out from the clock.
--
-- plays counts how many times the master played a sequence in this run, and
-- play_started_at is when the last play began: the players read the sequence a step at a
-- time while it plays, and never again after (the server works the steps out from
-- the clock). A play does not end by itself in the database either.
ALTER TABLE puzzle_runs
    ADD COLUMN IF NOT EXISTS plays INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS play_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS round_start_seq INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS round_started_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE puzzle_runs
    DROP COLUMN IF EXISTS round_started_at,
    DROP COLUMN IF EXISTS round_start_seq,
    DROP COLUMN IF EXISTS play_started_at,
    DROP COLUMN IF EXISTS plays;
