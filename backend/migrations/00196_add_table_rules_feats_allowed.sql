-- +goose Up
-- feats_allowed is the table rule "Talentos" (MR-025, MR-025): whether the
-- campaign plays with feats, an optional rule of the game. When true, the guided
-- level-up's Ability Score Improvement step offers a feat in place of the
-- ability increase. False by default (feats not used), which is what every
-- existing campaign did before the rule existed.
ALTER TABLE campaign_table_rules
    ADD COLUMN IF NOT EXISTS feats_allowed BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE campaign_table_rules
    DROP COLUMN IF EXISTS feats_allowed;
