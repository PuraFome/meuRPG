-- +goose Up
-- pick_swap_allowed is the table rule "Trocar uma escolha no aumento de atributo": at each
-- Ability Score Improvement level the level-up also lets the player give up one earlier pick
-- of a group (a cantrip, a fighting style, a metamagic option, an invocation, a pact boon)
-- for another of the same pool. False by default, which is what every existing campaign did
-- before the rule existed: a pick stays until the master edits the sheet.
ALTER TABLE campaign_table_rules
    ADD COLUMN IF NOT EXISTS pick_swap_allowed BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE campaign_table_rules
    DROP COLUMN IF EXISTS pick_swap_allowed;
