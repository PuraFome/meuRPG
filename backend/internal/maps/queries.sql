-- name: GetGalleryUsage :one
-- How much of the quota the campaign uses: its images and their bytes.
-- Upload reads it inside the transaction that inserts the new row, so two
-- uploads racing cannot both squeeze under the limit: CockroachDB's
-- SERIALIZABLE isolation makes one of them retry and count again.
SELECT
    count(*)::INT4 AS image_count,
    COALESCE(sum(byte_size), 0)::INT8 AS byte_count
FROM gallery_images
WHERE campaign_id = $1;

-- name: InsertGalleryImage :one
INSERT INTO gallery_images (id, campaign_id, uploaded_by, name, content_type, width, height, byte_size, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListGalleryImages :many
-- Newest first; id breaks ties, so the order never changes between calls.
SELECT * FROM gallery_images
WHERE campaign_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetGalleryImage :one
-- An image by its ID alone, to serve it: the caller's membership in its
-- campaign is checked right after.
SELECT * FROM gallery_images
WHERE id = $1;

-- name: RenameGalleryImage :one
UPDATE gallery_images
SET name = $3
WHERE campaign_id = $1 AND id = $2
RETURNING *;

-- name: DeleteGalleryImage :one
-- The row goes first; the caller deletes the files after the commit.
DELETE FROM gallery_images
WHERE campaign_id = $1 AND id = $2
RETURNING *;
