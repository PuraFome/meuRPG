-- +goose Up
-- campaign_content gets an eighth kind, 'option' (MR-025): an option the master adds to one of the
-- SRD's option sets (an invocation, a Metamagic option, a fighting style...). Its key is
-- "option:<set>:<slug>@mesa", which names the set, so the key may be longer than the others'
-- (up to 100 characters); its data is the protojson of TableOption in rules/v1/table_content.proto.
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat', 'option'));
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_key_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_key_valid
    CHECK (content_key LIKE kind || ':%@mesa' AND char_length(content_key) BETWEEN 9 AND 100);
ALTER TABLE campaign_content_off DROP CONSTRAINT IF EXISTS campaign_content_off_key_valid;
ALTER TABLE campaign_content_off ADD CONSTRAINT campaign_content_off_key_valid CHECK (char_length(content_key) BETWEEN 3 AND 100);

-- +goose Down
ALTER TABLE campaign_content_off DROP CONSTRAINT IF EXISTS campaign_content_off_key_valid;
ALTER TABLE campaign_content_off ADD CONSTRAINT campaign_content_off_key_valid CHECK (char_length(content_key) BETWEEN 3 AND 80);
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_key_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_key_valid
    CHECK (content_key LIKE kind || ':%@mesa' AND char_length(content_key) BETWEEN 9 AND 80);
ALTER TABLE campaign_content DROP CONSTRAINT IF EXISTS campaign_content_kind_valid;
ALTER TABLE campaign_content ADD CONSTRAINT campaign_content_kind_valid
    CHECK (kind IN ('class', 'subclass', 'race', 'subrace', 'background', 'spell', 'feat'));
