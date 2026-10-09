-- +goose Up
-- campaign_content gets a seventh kind, 'feat' (MR-025): the feats the master writes for a
-- campaign, an optional rule of the game. Its key is "feat:<slug>@mesa" and its data is the
-- protojson of TableFeat in rules/v1/table_content.proto, like every other kind. The rest of
-- the table (the key form, the sizes, the revision) is unchanged.
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat'));

-- +goose Down
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell'));
