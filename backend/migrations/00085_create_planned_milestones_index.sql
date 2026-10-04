-- +goose Up
-- A campaign's milestones in order: what every list reads. It also serves the
-- ON DELETE CASCADE when the campaign is deleted.
CREATE INDEX IF NOT EXISTS planned_milestones_campaign_id_position_idx
    ON planned_milestones (campaign_id, position);

-- +goose Down
DROP INDEX IF EXISTS planned_milestones_campaign_id_position_idx;
