-- +goose Up
-- The session's casts, newest first (ListSpellCasts' log).
CREATE INDEX IF NOT EXISTS spell_casts_game_session_id_started_at_idx
    ON spell_casts (game_session_id, started_at DESC);

-- The casts going or lasting: what a character concentrates on, what a combat
-- carries, what a rest ends. Only the live ones are in the index.
CREATE INDEX IF NOT EXISTS spell_casts_caster_id_live_idx
    ON spell_casts (caster_id, started_at)
    WHERE status IN ('casting', 'active');

-- A caster has one cast going at a time (SRD 5.1, "Longer Casting Times": the action
-- of every turn goes to the casting), and concentrates on one spell at a time
-- ("Duration": a creature can't concentrate on two spells at once). The session's row
-- lock serializes the writes; these make the rule a fact of the table.
CREATE UNIQUE INDEX IF NOT EXISTS spell_casts_one_casting_idx
    ON spell_casts (caster_id)
    WHERE status = 'casting';

CREATE UNIQUE INDEX IF NOT EXISTS spell_casts_one_concentration_idx
    ON spell_casts (caster_id)
    WHERE concentrating AND status IN ('casting', 'active');

-- The ON DELETE CASCADE when a campaign is deleted.
CREATE INDEX IF NOT EXISTS spell_casts_campaign_id_idx
    ON spell_casts (campaign_id);

-- +goose Down
DROP INDEX IF EXISTS spell_casts_campaign_id_idx;
DROP INDEX IF EXISTS spell_casts_one_concentration_idx;
DROP INDEX IF EXISTS spell_casts_one_casting_idx;
DROP INDEX IF EXISTS spell_casts_caster_id_live_idx;
DROP INDEX IF EXISTS spell_casts_game_session_id_started_at_idx;
