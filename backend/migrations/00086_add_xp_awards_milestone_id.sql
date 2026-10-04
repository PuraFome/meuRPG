-- +goose Up
-- The planned milestone a 'milestone' award marks (MR-016): "Marcar como
-- alcançado" and "Dar a mais alguém" write it, the ad hoc "Registrar um marco
-- fora da lista" leaves it NULL. SET NULL when the master removes a milestone
-- that was only ever planned again (all its awards undone): the history keeps
-- the award and its reason. Index: 00087.
--
-- One statement with several parts, as 00066, so re-running it is safe (an
-- inline REFERENCES would add the constraint again every time).
ALTER TABLE xp_awards
    ADD COLUMN IF NOT EXISTS milestone_id UUID NULL,
    DROP CONSTRAINT IF EXISTS xp_awards_milestone_id_fkey,
    ADD CONSTRAINT xp_awards_milestone_id_fkey
        FOREIGN KEY (milestone_id) REFERENCES planned_milestones (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE xp_awards
    DROP CONSTRAINT IF EXISTS xp_awards_milestone_id_fkey,
    DROP COLUMN IF EXISTS milestone_id;
