-- +goose Up
-- Erasing an account takes its ID out of the lists of who saw an NPC act that
-- session_events keep inside their payloads (docs/privacy.md, "Delete the
-- account"). The erasure is a DELETE FROM users, and the foreign keys do the rest
-- (actor_user_id is SET NULL); the payloads are JSON, so no key reaches them. A BEFORE
-- DELETE trigger on users does it in the same statement, hence the same transaction:
-- either the account and its ID in the history both go, or neither does.
--
-- The IDs live in these places of a payload (play/combat_events.go, combat_traps.go):
--   seen_by                  the players who saw every NPC of the event
--   cover_seen_by            the players told a cover the map gave
--   hits[].cover_seen_by     the same, one list per target of a cast
--   trap.caught[].seen_by    the same, one list per creature a trap caught
--   cover_users              the players that the bits of a cast's seen_mask and
--                            cover_seen_mask count, by position
-- A list of the first four loses the ID and keeps the order of the others; an empty
-- list stays (an event no one may read). cover_users keeps every position, because
-- the masks name players by it: the erased account's place becomes "", which no
-- account has. The event row stays; nothing else of the payload changes.
--
-- Only the events of the campaigns the account belongs to are looked at (a payload
-- names only players of its own campaign), through game_sessions' unique index on
-- (campaign_id, session_number) and session_events' on (game_session_id, seq); the text
-- test then skips the events that do not hold the ID. The work is proportional to the
-- history of the person's campaigns, once, in the statement that erases them.
--
-- The functions are written in SQL and PL/pgSQL that PostgreSQL and CockroachDB both
-- accept (a function is not IMMUTABLE here: CockroachDB refuses a subquery in one).

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION session_event_ids_without(ids JSONB, uid TEXT) RETURNS JSONB
LANGUAGE SQL AS $$
    SELECT CASE WHEN jsonb_typeof(ids) = 'array'
        THEN COALESCE((
            SELECT jsonb_agg(e.v ORDER BY e.n)
            FROM jsonb_array_elements(ids) WITH ORDINALITY AS e (v, n)
            WHERE e.v <> to_jsonb(uid)
        ), '[]'::JSONB)
        ELSE ids
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION session_event_ids_blanked(ids JSONB, uid TEXT) RETURNS JSONB
LANGUAGE SQL AS $$
    SELECT CASE WHEN jsonb_typeof(ids) = 'array'
        THEN COALESCE((
            SELECT jsonb_agg(CASE WHEN e.v = to_jsonb(uid) THEN '""'::JSONB ELSE e.v END ORDER BY e.n)
            FROM jsonb_array_elements(ids) WITH ORDINALITY AS e (v, n)
        ), '[]'::JSONB)
        ELSE ids
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION session_event_payload_without_user(payload JSONB, uid TEXT) RETURNS JSONB
LANGUAGE plpgsql AS $$
DECLARE
    r JSONB := payload;
BEGIN
    IF jsonb_typeof(r -> 'seen_by') = 'array' THEN
        r := jsonb_set(r, '{seen_by}', session_event_ids_without(r -> 'seen_by', uid));
    END IF;
    IF jsonb_typeof(r -> 'cover_seen_by') = 'array' THEN
        r := jsonb_set(r, '{cover_seen_by}', session_event_ids_without(r -> 'cover_seen_by', uid));
    END IF;
    IF jsonb_typeof(r -> 'cover_users') = 'array' THEN
        r := jsonb_set(r, '{cover_users}', session_event_ids_blanked(r -> 'cover_users', uid));
    END IF;
    IF jsonb_typeof(r -> 'hits') = 'array' THEN
        FOR i IN 0 .. jsonb_array_length(r -> 'hits') - 1 LOOP
            IF jsonb_typeof(r -> 'hits' -> i -> 'cover_seen_by') = 'array' THEN
                r := jsonb_set(r, ARRAY['hits', i::TEXT, 'cover_seen_by'],
                    session_event_ids_without(r -> 'hits' -> i -> 'cover_seen_by', uid));
            END IF;
        END LOOP;
    END IF;
    IF jsonb_typeof(r #> '{trap,caught}') = 'array' THEN
        FOR i IN 0 .. jsonb_array_length(r #> '{trap,caught}') - 1 LOOP
            IF jsonb_typeof(r #> ARRAY['trap', 'caught', i::TEXT, 'seen_by']) = 'array' THEN
                r := jsonb_set(r, ARRAY['trap', 'caught', i::TEXT, 'seen_by'],
                    session_event_ids_without(r #> ARRAY['trap', 'caught', i::TEXT, 'seen_by'], uid));
            END IF;
        END LOOP;
    END IF;
    RETURN r;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION users_scrub_session_events() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    erased UUID;
BEGIN
    erased := (OLD).id; -- the parentheses are what CockroachDB needs to read a field of OLD
    UPDATE session_events
    SET payload = session_event_payload_without_user(payload, erased::TEXT)
    WHERE game_session_id IN (
            SELECT gs.id
            FROM campaign_members m
            JOIN game_sessions gs ON gs.campaign_id = m.campaign_id
            WHERE m.user_id = erased
        )
        AND payload::TEXT LIKE '%' || erased::TEXT || '%';
    RETURN OLD;
END
$$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS users_scrub_session_events ON users;
CREATE TRIGGER users_scrub_session_events
    BEFORE DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION users_scrub_session_events();

-- +goose Down
DROP TRIGGER IF EXISTS users_scrub_session_events ON users;
DROP FUNCTION IF EXISTS users_scrub_session_events();
DROP FUNCTION IF EXISTS session_event_payload_without_user(JSONB, TEXT);
DROP FUNCTION IF EXISTS session_event_ids_blanked(JSONB, TEXT);
DROP FUNCTION IF EXISTS session_event_ids_without(JSONB, TEXT);
