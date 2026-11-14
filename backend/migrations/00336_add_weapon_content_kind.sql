-- +goose Up
-- campaign_content gets an eleventh kind, 'weapon': a weapon the SRD does not have (category, damage,
-- properties, weight, price). Its key is "weapon:<slug>@mesa" and its data is the protojson of TableWeapon in
-- rules/v1/table_content.proto.
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat', 'option', 'classfeature', 'creature', 'weapon'));

-- +goose Down
DELETE FROM campaign_content WHERE kind = 'weapon';
DELETE FROM campaign_content_off WHERE content_key LIKE 'weapon:%';
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat', 'option', 'classfeature', 'creature'));
