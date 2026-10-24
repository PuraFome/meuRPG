-- +goose Up
-- A scene open to only some players keeps the accounts of its players in the history (MR-015, RN-10): the
-- scene_opened event holds them in `players`, and each scene_check_rolled the ones in the scene when the check
-- was rolled in `audience` (the session summary counts the roll for them and the master only). Erasing an
-- account takes its ID out of these lists as out of the other who-saw lists of a payload
-- (00266, docs/privacy.md, "Delete the account"): the trigger already calls
-- session_event_payload_without_user, so only that function learns the two keys. A list that loses its last ID
-- stays, empty (the roll then counts for the master alone). Running it twice is safe: CREATE OR REPLACE.

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
    IF jsonb_typeof(r -> 'players') = 'array' THEN
        r := jsonb_set(r, '{players}', session_event_ids_without(r -> 'players', uid));
    END IF;
    IF jsonb_typeof(r -> 'audience') = 'array' THEN
        r := jsonb_set(r, '{audience}', session_event_ids_without(r -> 'audience', uid));
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

-- +goose Down
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
