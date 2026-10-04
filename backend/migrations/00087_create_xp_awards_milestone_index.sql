-- +goose Up
-- The awards of a milestone: whether it is reached, when and for whom. It also
-- serves the ON DELETE SET NULL when a milestone is removed.
CREATE INDEX IF NOT EXISTS xp_awards_milestone_id_idx ON xp_awards (milestone_id);

-- +goose Down
DROP INDEX IF EXISTS xp_awards_milestone_id_idx;
