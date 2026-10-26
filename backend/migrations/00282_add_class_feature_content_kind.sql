-- +goose Up
-- campaign_content gets a ninth kind, 'classfeature' (MR-025): a feature the master adds to a class,
-- at a class level, that may take the place of a feature the class has. Its key is
-- "classfeature:<slug>@mesa" and its data is the protojson of TableClassFeature in
-- rules/v1/table_content.proto.
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat', 'option', 'classfeature'));

-- +goose Down
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat', 'option'));
