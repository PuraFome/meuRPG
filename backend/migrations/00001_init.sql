-- +goose Up
-- The schema history starts here, empty on purpose.
--
-- The new MeuRPG does not reuse the old app's tables (server/src/db/schema.sql):
-- it starts from a fresh schema (decided on 2026-09-29). Each module adds its
-- own tables in the migrations that follow, together with its code and tests.
SELECT 1;

-- +goose Down
SELECT 1;
