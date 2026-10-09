-- +goose Up
-- hidden_area_hits is the table rule "Criaturas escondidas atingidas por uma área"
-- (RN-24): what a spell that hits a hidden creature does to its hiding. The spell
-- always affects the creature (the SRD says each creature in the area is affected);
-- the rule only decides whether the players learn of it.
--   - 'reveal' (the default): the creature appears to the players, as when the
--     master reveals it.
--   - 'keep_hidden': the creature takes the effect and stays hidden.
--   - 'ask': when a player's spell hits a hidden creature, the turn waits and the
--     master chooses between the two (hidden_reveals).
-- A campaign with no row, or a row from before the column, has 'reveal'.
ALTER TABLE campaign_table_rules ADD COLUMN IF NOT EXISTS hidden_area_hits TEXT NOT NULL DEFAULT 'reveal';

ALTER TABLE campaign_table_rules DROP CONSTRAINT IF EXISTS campaign_table_rules_hidden_area_hits_valid;
ALTER TABLE campaign_table_rules ADD CONSTRAINT campaign_table_rules_hidden_area_hits_valid CHECK (hidden_area_hits IN ('reveal', 'keep_hidden', 'ask'));

-- +goose Down
ALTER TABLE campaign_table_rules DROP CONSTRAINT IF EXISTS campaign_table_rules_hidden_area_hits_valid;
ALTER TABLE campaign_table_rules DROP COLUMN IF EXISTS hidden_area_hits;
