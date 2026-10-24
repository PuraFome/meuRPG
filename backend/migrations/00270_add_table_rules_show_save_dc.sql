-- +goose Up
-- show_save_dc is the table rule "Os jogadores veem a CD dos testes de resistência" (RN-24,
-- RN-10): whether the player who rolls a saving throw the app asks of them also reads its DC.
-- False by default, which is what every existing campaign did before the rule existed: only the
-- master reads the DC of a save (and, for a spell, its caster's player, who has it on their sheet).
-- When true, the player of the creature that rolls the save reads the DC of that save, for a
-- zone, a lasting effect, a spell cast by an NPC or another character, and a trap; no other player
-- gains anything.
ALTER TABLE campaign_table_rules
    ADD COLUMN IF NOT EXISTS show_save_dc BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE campaign_table_rules
    DROP COLUMN IF EXISTS show_save_dc;
