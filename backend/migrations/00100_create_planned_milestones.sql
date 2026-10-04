-- +goose Up
-- planned_milestones are the milestones the master wrote ahead of time for a
-- campaign that levels by milestones (MR-016, RN-09, RN-12, question 45): one
-- short line each, in the master's own order. At most 100 per campaign (the
-- API checks, inside the transaction that inserts one). The text is free text
-- the master writes, 1 to 120 characters, like the reason of an XP award; it
-- is fiction, and a player only ever receives it once the milestone is
-- reached (RN-20).
--
-- "Reached" is not a column: a milestone is reached while at least one of the
-- xp_awards that name it (xp_awards.milestone_id, 00102) is not undone, so
-- undoing the last mark makes it planned again with nothing to keep in step.
-- A reached milestone is never edited, moved nor removed (the API refuses).
--
-- position orders the milestones of a campaign, from 0. It is not unique: a
-- move renumbers the campaign's milestones in one transaction. Deleting the
-- campaign deletes its milestones (CASCADE). No personal data.
CREATE TABLE IF NOT EXISTS planned_milestones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    position INT4 NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT planned_milestones_position_valid CHECK (position >= 0),
    CONSTRAINT planned_milestones_text_length CHECK (char_length(text) BETWEEN 1 AND 120)
);

-- +goose Down
DROP TABLE IF EXISTS planned_milestones;
