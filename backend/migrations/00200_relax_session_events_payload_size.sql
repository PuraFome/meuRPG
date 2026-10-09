-- +goose Up
-- A spell's area can reach every creature of a combat (it holds up to 40), and the
-- event of the cast and the one of its damage roll keep what each of them did.
-- Ten of those fit the 4 KiB the table allowed since it was made; forty need more
-- room, so the limit is 16 KiB. It is still a limit, and a small one: a payload is
-- IDs and numbers, never text (docs/data.md).
ALTER TABLE session_events DROP CONSTRAINT IF EXISTS session_events_payload_valid;
ALTER TABLE session_events ADD CONSTRAINT session_events_payload_valid CHECK (
    jsonb_typeof(payload) = 'object' AND octet_length(payload::TEXT) <= 16384
);

-- +goose Down
ALTER TABLE session_events DROP CONSTRAINT IF EXISTS session_events_payload_valid;
ALTER TABLE session_events ADD CONSTRAINT session_events_payload_valid CHECK (
    jsonb_typeof(payload) = 'object' AND octet_length(payload::TEXT) <= 4096
);
