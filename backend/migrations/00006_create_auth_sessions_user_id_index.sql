-- +goose Up
-- Finds a user's sessions, for "sign out everywhere" and for the ON DELETE
-- CASCADE when an account is deleted.
--
-- This index used to be written inside 00004's CREATE TABLE, which
-- CockroachDB allows but PostgreSQL does not. sqlc reads the migrations with
-- the PostgreSQL parser, so every index now gets its own CREATE INDEX. On a
-- database where 00004 already created the index, IF NOT EXISTS makes this a
-- no-op.
CREATE INDEX IF NOT EXISTS auth_sessions_user_id_idx ON auth_sessions (user_id);

-- +goose Down
DROP INDEX IF EXISTS auth_sessions_user_id_idx;
