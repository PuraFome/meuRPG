-- +goose Up
-- max_attempts is how many times each player's character may roll a scene
-- action while the scene stays open (MR-015, Etapa 8, question 55: the master
-- sets the limit per action). 1 by default, 1 to 5, and 0 means unlimited.
-- Closing the scene and opening it again starts the count afresh; the rolls
-- themselves are session events, so lowering the limit erases nothing.
--
-- One statement with several parts, as 00067, so re-running it is safe.
ALTER TABLE scene_actions
    ADD COLUMN IF NOT EXISTS max_attempts INT4 NOT NULL DEFAULT 1,
    DROP CONSTRAINT IF EXISTS scene_actions_max_attempts_valid,
    ADD CONSTRAINT scene_actions_max_attempts_valid CHECK (max_attempts BETWEEN 0 AND 5);

-- +goose Down
ALTER TABLE scene_actions
    DROP CONSTRAINT IF EXISTS scene_actions_max_attempts_valid,
    DROP COLUMN IF EXISTS max_attempts;
