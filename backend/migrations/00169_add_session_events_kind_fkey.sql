-- +goose Up
-- Every session event points at its kind in session_event_kinds (00168). One
-- statement adds the constraint and validates it, once: from now on a new kind is
-- an INSERT in a migration and never checks the old rows again. ON DELETE RESTRICT
-- (the default) because a kind that has events must not disappear. Dropped first,
-- if present, so re-running this migration is safe (CockroachDB DDL is not atomic).
ALTER TABLE session_events
    DROP CONSTRAINT IF EXISTS session_events_kind_fkey,
    ADD CONSTRAINT session_events_kind_fkey FOREIGN KEY (kind) REFERENCES session_event_kinds (kind) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE session_events DROP CONSTRAINT IF EXISTS session_events_kind_fkey;
