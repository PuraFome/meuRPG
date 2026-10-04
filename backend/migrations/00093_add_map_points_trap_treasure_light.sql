-- +goose Up
-- The new kinds of point of Etapa 9: 'trap' (Armadilha, MR-035, D5), 'treasure'
-- (Tesouro, MR-041, D8) and 'light' (Luz, MR-036, D6). Each kind has columns
-- of its own, all NULL on the other kinds (the CHECKs say so), so a point of
-- one kind can never carry another's data.
--
-- A trap: trap is what the master wrote (JSON in the API's own shape, the
-- messages of TrapSpec: the preset it came from, the DCs, the area, the
-- trigger and the effect in parts), checked by the API against the rules
-- content. Only the master ever reads it (RN-10). trap_state is 'armed',
-- 'triggered' or 'disarmed' and has its own column because the live game
-- changes it. trap_triggered_at is when it first fired and is never cleared:
-- a trap that fired is public for good, even if the master later marks it
-- disarmed or sets it armed again.
--
-- A treasure: treasure_value_po is its worth in gold pieces (0 to 1,000,000);
-- its description is the point's. treasure_found_at is when the master marked
-- it found (the finders are in map_treasure_finders, 00095), and
-- treasure_session_id the game session that was open then, if any, so the
-- session's summary counts it. treasure_converted_award_id is the XP award
-- that turned it into XP ("Voltar à cidade", slice 9.11): a converted
-- treasure's value and found state cannot change.
--
-- The two IDs have no foreign key, on purpose. map_points and game_sessions
-- already refer to each other (the open scene, 00066), and a reference to
-- game_sessions or to xp_awards (which refers to encounters, which refer to
-- map_points) would close a cycle that the test databases' table copy cannot
-- build (package dbtest). Nothing is lost: a session and an award are only
-- ever deleted with their campaign, which deletes the map's points too.
--
-- A light: light_preset is the key of the light preset it was made from
-- ("light:torch") or NULL for a custom one; light_bright_ft and light_dim_ft
-- are its radii in feet: bright light up to the first, dim light for that much
-- further past it (the SRD's way to say it). Multiples of 5 (a square), 0 to
-- 120, not both 0. (Each CHECK says IS NOT NULL out loud: a CHECK passes when its
-- test is NULL.)
--
-- One statement with several parts, as 00086, so re-running it is safe.
ALTER TABLE map_points
    ADD COLUMN IF NOT EXISTS trap JSONB NULL,
    ADD COLUMN IF NOT EXISTS trap_state TEXT NULL,
    ADD COLUMN IF NOT EXISTS trap_triggered_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS treasure_value_po INT4 NULL,
    ADD COLUMN IF NOT EXISTS treasure_found_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS treasure_session_id UUID NULL,
    ADD COLUMN IF NOT EXISTS treasure_converted_award_id UUID NULL,
    ADD COLUMN IF NOT EXISTS light_preset TEXT NULL,
    ADD COLUMN IF NOT EXISTS light_bright_ft INT4 NULL,
    ADD COLUMN IF NOT EXISTS light_dim_ft INT4 NULL,
    DROP CONSTRAINT IF EXISTS map_points_kind_valid,
    ADD CONSTRAINT map_points_kind_valid CHECK (kind IN ('battle', 'submap', 'scene', 'trap', 'treasure', 'light')),
    DROP CONSTRAINT IF EXISTS map_points_trap_valid,
    ADD CONSTRAINT map_points_trap_valid CHECK (
        (kind = 'trap' AND trap IS NOT NULL AND trap_state IS NOT NULL AND trap_state IN ('armed', 'triggered', 'disarmed'))
        OR (kind <> 'trap' AND trap IS NULL AND trap_state IS NULL AND trap_triggered_at IS NULL)
    ),
    DROP CONSTRAINT IF EXISTS map_points_treasure_valid,
    ADD CONSTRAINT map_points_treasure_valid CHECK (
        (kind = 'treasure' AND treasure_value_po IS NOT NULL AND treasure_value_po BETWEEN 0 AND 1000000)
        OR (kind <> 'treasure' AND treasure_value_po IS NULL AND treasure_found_at IS NULL
            AND treasure_session_id IS NULL AND treasure_converted_award_id IS NULL)
    ),
    DROP CONSTRAINT IF EXISTS map_points_treasure_converted_was_found,
    ADD CONSTRAINT map_points_treasure_converted_was_found CHECK (
        treasure_converted_award_id IS NULL OR treasure_found_at IS NOT NULL
    ),
    DROP CONSTRAINT IF EXISTS map_points_light_valid,
    ADD CONSTRAINT map_points_light_valid CHECK (
        (kind = 'light' AND light_bright_ft IS NOT NULL AND light_dim_ft IS NOT NULL
            AND light_bright_ft BETWEEN 0 AND 120 AND light_dim_ft BETWEEN 0 AND 120
            AND light_bright_ft % 5 = 0 AND light_dim_ft % 5 = 0 AND light_bright_ft + light_dim_ft > 0)
        OR (kind <> 'light' AND light_preset IS NULL AND light_bright_ft IS NULL AND light_dim_ft IS NULL)
    );

-- +goose Down
DELETE FROM map_points WHERE kind IN ('trap', 'treasure', 'light');

ALTER TABLE map_points
    DROP CONSTRAINT IF EXISTS map_points_light_valid,
    DROP CONSTRAINT IF EXISTS map_points_treasure_converted_was_found,
    DROP CONSTRAINT IF EXISTS map_points_treasure_valid,
    DROP CONSTRAINT IF EXISTS map_points_trap_valid,
    DROP CONSTRAINT IF EXISTS map_points_kind_valid,
    DROP COLUMN IF EXISTS light_dim_ft,
    DROP COLUMN IF EXISTS light_bright_ft,
    DROP COLUMN IF EXISTS light_preset,
    DROP COLUMN IF EXISTS treasure_converted_award_id,
    DROP COLUMN IF EXISTS treasure_session_id,
    DROP COLUMN IF EXISTS treasure_found_at,
    DROP COLUMN IF EXISTS treasure_value_po,
    DROP COLUMN IF EXISTS trap_triggered_at,
    DROP COLUMN IF EXISTS trap_state,
    DROP COLUMN IF EXISTS trap;

ALTER TABLE map_points
    ADD CONSTRAINT map_points_kind_valid CHECK (kind IN ('battle', 'submap', 'scene'));
