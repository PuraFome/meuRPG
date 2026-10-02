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

-- name: ListMapsUsingImage :many
-- The maps whose image this is, oldest first: DeleteGalleryImage names them
-- (MR-019), and RenameGalleryImage returns them with the image.
SELECT id, name FROM maps
WHERE campaign_id = $1 AND image_id = $2
ORDER BY created_at, id;

-- name: ListMapImageIDs :many
-- Every map of the campaign with its image, oldest first: the gallery
-- shows, on each image, the maps that use it.
SELECT id, name, image_id FROM maps
WHERE campaign_id = $1
ORDER BY created_at, id;

-- Maps (MR-008, MR-009, MR-012). Every query names the campaign next to the
-- map, or runs after a query that did: a map of another campaign matches no
-- row, which the handlers answer as "not found".

-- name: GetGalleryImageInCampaign :one
-- The image a map is made of must be one of the campaign's.
SELECT * FROM gallery_images
WHERE campaign_id = $1 AND id = $2;

-- name: CountMaps :one
-- The campaign's maps, for the limit. Read inside the transaction that
-- inserts a map, as GetGalleryUsage.
SELECT count(*)::INT4 AS map_count FROM maps
WHERE campaign_id = $1;

-- name: InsertMap :one
-- A new map starts hidden (revealed_at NULL).
INSERT INTO maps (campaign_id, name, image_id, created_at, updated_at)
VALUES (sqlc.arg(campaign_id), sqlc.arg(name), sqlc.arg(image_id), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: ListMapDetails :many
-- The campaign's maps, oldest first, with what the lists show about each:
-- its image's name and size, and how many points it has, in all and
-- revealed. Campaigns have a few maps, so one query answers ListMaps and
-- gives GetMap the names and states it needs (parents, submap targets).
SELECT m.id, m.campaign_id, m.name, m.image_id, m.revealed_at, m.revision, m.created_at, m.updated_at,
       g.name AS image_name, g.width AS image_width, g.height AS image_height,
       (SELECT count(*) FROM map_points AS p WHERE p.map_id = m.id)::INT4 AS point_count,
       (SELECT count(*) FROM map_points AS p WHERE p.map_id = m.id AND p.revealed_at IS NOT NULL)::INT4 AS revealed_point_count
FROM maps AS m
JOIN gallery_images AS g ON g.id = m.image_id
WHERE m.campaign_id = $1
ORDER BY m.created_at, m.id;

-- name: ListSubmapLinks :many
-- Every submap point of the campaign that leads to a map: the map it is on,
-- the map it leads to, and whether the point is revealed. It gives each map
-- its parents ("Submapa de ...").
SELECT p.map_id, p.target_map_id::UUID AS target_map_id, (p.revealed_at IS NOT NULL)::BOOL AS revealed
FROM map_points AS p
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND p.target_map_id IS NOT NULL
ORDER BY p.created_at, p.id;

-- name: GetMapForUpdate :one
-- FOR UPDATE locks the map's row until the transaction ends, so two edits of
-- the same map wait for each other instead of both reading the same
-- revision.
SELECT * FROM maps
WHERE campaign_id = $1 AND id = $2
FOR UPDATE;

-- name: GetMap :one
SELECT * FROM maps
WHERE campaign_id = $1 AND id = $2;

-- name: UpdateMap :one
-- revision in the WHERE clause is a second guard: the handler already
-- compared it under FOR UPDATE, so no row here means a stale revision.
UPDATE maps
SET name = sqlc.arg(name), image_id = sqlc.arg(image_id), revision = revision + 1, updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id) AND id = sqlc.arg(id) AND revision = sqlc.arg(revision)
RETURNING *;

-- name: SetMapRevealed :one
-- Revealing keeps the first revealed_at, so revealing twice changes
-- nothing; hiding clears it. PlayService.SetCurrentMap reveals through here
-- too (SessionMaps).
UPDATE maps
SET revealed_at = CASE WHEN sqlc.arg(revealed)::BOOL THEN COALESCE(revealed_at, sqlc.arg(now)::TIMESTAMPTZ) ELSE NULL END,
    updated_at = CASE WHEN (revealed_at IS NOT NULL) = sqlc.arg(revealed)::BOOL THEN updated_at ELSE sqlc.arg(now)::TIMESTAMPTZ END
WHERE campaign_id = sqlc.arg(campaign_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteMap :one
-- Points and tokens go with the map (CASCADE); submap points of other maps
-- lose their target, and a session's current map is unset (SET NULL).
DELETE FROM maps
WHERE campaign_id = $1 AND id = $2
RETURNING *;

-- name: CountMapPoints :one
-- The map's points, for the limit, inside the transaction that inserts one.
SELECT count(*)::INT4 AS point_count FROM map_points
WHERE map_id = $1;

-- name: InsertMapPoint :one
-- A new point starts hidden (revealed_at NULL).
INSERT INTO map_points (map_id, kind, name, description, x_bp, y_bp, target_map_id, created_at, updated_at)
VALUES (
    sqlc.arg(map_id), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(description), sqlc.arg(x_bp), sqlc.arg(y_bp),
    sqlc.narg(target_map_id), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: ListMapPoints :many
-- The map's points, oldest first. The handler filters them for a player.
SELECT * FROM map_points
WHERE map_id = $1
ORDER BY created_at, id;

-- name: GetMapPointForUpdate :one
-- The caller checked first that the map is the campaign's.
SELECT * FROM map_points
WHERE map_id = $1 AND id = $2
FOR UPDATE;

-- name: UpdateMapPoint :one
-- Every column the API may change, with the values the handler worked out
-- from the request and the current row.
UPDATE map_points
SET kind = sqlc.arg(kind), name = sqlc.arg(name), description = sqlc.arg(description),
    x_bp = sqlc.arg(x_bp), y_bp = sqlc.arg(y_bp), target_map_id = sqlc.narg(target_map_id),
    revealed_at = sqlc.narg(revealed_at), updated_at = sqlc.arg(now)
WHERE map_id = sqlc.arg(map_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteMapPoint :one
DELETE FROM map_points
WHERE map_id = $1 AND id = $2
RETURNING *;

-- name: ListMapTokens :many
-- The map's tokens. The handler orders them by character and filters them
-- for a player.
SELECT * FROM map_tokens
WHERE map_id = $1;

-- name: GetMapTokenForUpdate :one
SELECT * FROM map_tokens
WHERE map_id = $1 AND character_id = $2
FOR UPDATE;

-- name: InsertMapToken :one
INSERT INTO map_tokens (map_id, character_id, x_bp, y_bp, hidden, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: MoveMapToken :one
UPDATE map_tokens
SET x_bp = $3, y_bp = $4, updated_at = $5
WHERE map_id = $1 AND character_id = $2
RETURNING *;

-- name: SetMapTokenHidden :one
UPDATE map_tokens
SET hidden = sqlc.arg(hidden), updated_at = CASE WHEN hidden = sqlc.arg(hidden) THEN updated_at ELSE sqlc.arg(now)::TIMESTAMPTZ END
WHERE map_id = sqlc.arg(map_id) AND character_id = sqlc.arg(character_id)
RETURNING *;

-- name: DeleteMapToken :one
DELETE FROM map_tokens
WHERE map_id = $1 AND character_id = $2
RETURNING *;

-- name: ImageIsOnAVisibleMap :one
-- Whether the image is the background of a map the players see now: a
-- revealed map, or the open session's current map (NULL when none). The
-- image route asks it for a player (RN-10); maps_image_id_idx finds the
-- maps.
SELECT EXISTS (
    SELECT 1 FROM maps
    WHERE campaign_id = sqlc.arg(campaign_id) AND image_id = sqlc.arg(image_id)
      AND (revealed_at IS NOT NULL OR id = sqlc.narg(current_map_id)::UUID)
);
