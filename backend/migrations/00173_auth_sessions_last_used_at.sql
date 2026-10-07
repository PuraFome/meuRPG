-- +goose Up
-- last_used_at is when the session last made a request, for the idle timeout
-- (ASVS 5.0 V7.3.1): a session unused for SESSION_IDLE_TIMEOUT (14 days by
-- default) stops working, even before expires_at. The app writes it at most
-- once every 10 minutes per session, so a busy session is not a write per
-- request. Sessions that exist now start with the time of this migration:
-- nobody is signed out by it. New rows get created_at from the app.
ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE auth_sessions DROP COLUMN IF EXISTS last_used_at;
