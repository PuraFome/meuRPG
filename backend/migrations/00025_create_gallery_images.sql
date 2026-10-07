-- +goose Up
-- gallery_images lists a campaign's images (MR-019): the gallery the master
-- uses for maps and for the campaign document. The files themselves are
-- not here: the image and its thumbnail live in the blob store (package
-- internal/platform/blob) under campaigns/<campaign_id>/images/<id> and
-- .../<id>.thumb. The API writes the files first and this row last, so a
-- row always has its files.
--
-- id has no DEFAULT: the API makes it before writing the files, because
-- their keys carry it.
--
-- Every stored image was decoded and encoded again by the API (package
-- internal/maps/images), so it has no metadata (EXIF, GPS...), and it is a
-- JPEG or a PNG, whatever the uploaded file was. width, height and
-- byte_size describe that stored image; byte_size counts toward the
-- campaign's quota. The CHECKs repeat the API's limits.
--
-- name comes from the uploaded file's name and the master can change it.
-- It is free text, like a campaign's name.
--
-- uploaded_by is who sent the image, for when a campaign has more than one
-- master (RN-13). It is not in the API. Deleting that account keeps the
-- image with the campaign (SET NULL); deleting the campaign deletes the
-- rows (CASCADE), but not the files: whatever deletes a campaign must also
-- delete its blob prefix (docs/privacy.md). No index on uploaded_by:
-- only an account deletion looks it up, as for campaigns.created_by.
CREATE TABLE IF NOT EXISTS gallery_images (
    id UUID PRIMARY KEY,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    uploaded_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    width INT4 NOT NULL,
    height INT4 NOT NULL,
    byte_size INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT gallery_images_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT gallery_images_content_type_valid CHECK (content_type IN ('image/jpeg', 'image/png')),
    CONSTRAINT gallery_images_size_valid CHECK (width BETWEEN 1 AND 8192 AND height BETWEEN 1 AND 8192),
    CONSTRAINT gallery_images_byte_size_valid CHECK (byte_size BETWEEN 1 AND 10485760)
);

-- +goose Down
DROP TABLE IF EXISTS gallery_images;
