-- +goose Up
-- campaign_table_rules holds the rules a table chooses on "Regras da mesa"
-- (MR-025, RN-24), one row per campaign. No row means every setting has its
-- default, which is what the app did before the rules existed (the SRD's):
-- the code reads a missing row as the defaults, so no existing campaign needs a
-- backfill and a campaign that never opens the page never gets a row.
--
-- The dice mode (RN-18) stays on campaigns.dice_mode: a preset of the table's
-- style writes it there. "Estilo da mesa" is not stored: it is worked out from
-- the dice mode, combat_starts_with_map and fog_on_new_maps, and a campaign
-- that matches no preset is "Personalizado".
--
--   - hit_points_rule: how a level-up's hit points are decided ('roll',
--     'average' or 'player_chooses', the default).
--   - ability_standard_array, ability_point_buy, ability_roll_4d6 and
--     ability_typed: the ways a player may make a new sheet's ability scores.
--     All four by default, at least one always.
--   - critical_rule: 'doubled_dice' (the SRD's, the default) or 'max_plus_roll'.
--     Stored here; the combat applies it in a later slice (10.4b).
--   - death_saves: 'visible_to_all' (the default) or 'owner_and_master'. Stored
--     here; the combat applies it in a later slice (10.4b).
--   - combat_starts_with_map: whether "Iniciar combate" starts on a map by
--     default (true by default, as today).
--   - fog_on_new_maps: whether a map created from now on has the fog of war on
--     (false by default, as today).
--   - house_rules: the table's reminders, up to 20 (the API also limits each to
--     200 characters). The app never enforces them.
--
-- Deleting the campaign deletes the row. No personal data.
CREATE TABLE IF NOT EXISTS campaign_table_rules (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    hit_points_rule TEXT NOT NULL DEFAULT 'player_chooses',
    ability_standard_array BOOL NOT NULL DEFAULT true,
    ability_point_buy BOOL NOT NULL DEFAULT true,
    ability_roll_4d6 BOOL NOT NULL DEFAULT true,
    ability_typed BOOL NOT NULL DEFAULT true,
    critical_rule TEXT NOT NULL DEFAULT 'doubled_dice',
    death_saves TEXT NOT NULL DEFAULT 'visible_to_all',
    combat_starts_with_map BOOL NOT NULL DEFAULT true,
    fog_on_new_maps BOOL NOT NULL DEFAULT false,
    house_rules TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT campaign_table_rules_hit_points_valid CHECK (hit_points_rule IN ('roll', 'average', 'player_chooses')),
    CONSTRAINT campaign_table_rules_critical_valid CHECK (critical_rule IN ('doubled_dice', 'max_plus_roll')),
    CONSTRAINT campaign_table_rules_death_saves_valid CHECK (death_saves IN ('visible_to_all', 'owner_and_master')),
    CONSTRAINT campaign_table_rules_ability_methods_valid CHECK (ability_standard_array OR ability_point_buy OR ability_roll_4d6 OR ability_typed),
    CONSTRAINT campaign_table_rules_house_rules_valid CHECK (cardinality(house_rules) <= 20)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_table_rules;
