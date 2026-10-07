-- +goose Up
-- display_name is how other members of a campaign see this user. The user
-- types it in the app (IdentityService.UpdateProfile); it is never copied
-- from the sign-in provider, which is why sign-in asks only for "openid
-- email" (docs/privacy.md). NULL until the user sets one.
--
-- At most 40 characters (ADR-0009). The application trims and validates the
-- name first; the CHECK is the last line of defense.
--
-- One statement with three parts, so re-running it is safe: ADD COLUMN IF
-- NOT EXISTS skips a column that exists, but CockroachDB would still add a
-- second copy of an unnamed CHECK (or fail on a named one), so the CHECK is
-- dropped, if present, and added again.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name TEXT NULL,
    DROP CONSTRAINT IF EXISTS users_display_name_length,
    ADD CONSTRAINT users_display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 40);

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
