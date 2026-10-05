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
--
-- revealed_point_count is how many points every player sees: not a light, which
-- no player ever receives, and either revealed, a triggered trap or a found
-- treasure (a trap revealed to some characters only is the handler's to add).
SELECT m.id, m.campaign_id, m.name, m.image_id, m.revealed_at, m.revision, m.created_at, m.updated_at, m.grid_columns,
       m.fog_enabled, m.base_light, m.group_vision, m.layers_revision, m.light_revision, m.vision_epoch,
       g.name AS image_name, g.width AS image_width, g.height AS image_height,
       (SELECT count(*) FROM map_points AS p WHERE p.map_id = m.id)::INT4 AS point_count,
       (SELECT count(*) FROM map_points AS p
        WHERE p.map_id = m.id AND p.kind <> 'light'
          AND (p.revealed_at IS NOT NULL OR p.trap_triggered_at IS NOT NULL OR p.treasure_found_at IS NOT NULL))::INT4 AS revealed_point_count
FROM maps AS m
JOIN gallery_images AS g ON g.id = m.image_id
WHERE m.campaign_id = $1
ORDER BY m.created_at, m.id;

-- name: ListSubmapLinks :many
-- Every submap point of the campaign that leads to a map: the map it is on,
-- the map it leads to, and whether the point is revealed. It gives each map
-- its parents ("Submapa de ...").
SELECT p.map_id, p.target_map_id::UUID AS target_map_id, (p.revealed_at IS NOT NULL)::BOOL AS revealed, p.x_bp, p.y_bp
FROM map_points AS p
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND p.kind = 'submap' AND p.target_map_id IS NOT NULL
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

-- name: SetMapGrid :one
-- The master's grid (MR-013): NULL clears it, and a map without a grid has no
-- fog of war. It is a change to the map itself, so updated_at moves, but the
-- revision (the name and the image's guard) does not.
UPDATE maps
SET grid_columns = sqlc.narg(grid_columns), fog_enabled = fog_enabled AND sqlc.narg(grid_columns)::INT4 IS NOT NULL,
    updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: SetMapFog :one
-- The fog of war's settings (MR-036): each one the handler sends replaces the
-- current value, the others stay. updated_at moves only when something a
-- player reads changes (the switch or "Visão do grupo"): the base light is the
-- master's, and a player must not learn that it changed.
UPDATE maps
SET fog_enabled = COALESCE(sqlc.narg(fog_enabled)::BOOL, fog_enabled),
    base_light = COALESCE(sqlc.narg(base_light)::TEXT, base_light),
    group_vision = COALESCE(sqlc.narg(group_vision)::BOOL, group_vision),
    updated_at = CASE WHEN COALESCE(sqlc.narg(fog_enabled)::BOOL, fog_enabled) <> fog_enabled
                        OR COALESCE(sqlc.narg(group_vision)::BOOL, group_vision) <> group_vision
                      THEN sqlc.arg(now)::TIMESTAMPTZ ELSE updated_at END
WHERE campaign_id = sqlc.arg(campaign_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: GetMapLayers :one
-- The map's painted layers; no row means nothing is painted.
SELECT * FROM map_layers
WHERE map_id = $1;

-- name: UpsertMapLayers :exec
-- Writes the four layers of a map (NULL for a layer with nothing painted). The
-- handler holds the map's row lock (GetMapForUpdate), so two batches of paint
-- take turns.
INSERT INTO map_layers (map_id, difficult_terrain, walls, cover, light, updated_at)
VALUES (sqlc.arg(map_id), sqlc.narg(difficult_terrain), sqlc.narg(walls), sqlc.narg(cover), sqlc.narg(light), sqlc.arg(now))
ON CONFLICT (map_id) DO UPDATE
SET difficult_terrain = excluded.difficult_terrain, walls = excluded.walls, cover = excluded.cover,
    light = excluded.light, updated_at = excluded.updated_at;

-- name: DeleteMapLayers :execrows
-- Clears every layer of the map: the grid's columns or the image changed.
DELETE FROM map_layers
WHERE map_id = $1;

-- name: BumpMapLayersRevision :one
-- A painted layer changed, or all of them were cleared: readers must read them
-- again. It leaves updated_at and revision (the name and image's guard) alone.
UPDATE maps
SET layers_revision = layers_revision + 1
WHERE id = $1
RETURNING layers_revision;

-- name: BumpMapLightRevision :one
-- The painted light changed. No player reads the light, so this is not the
-- number they see: the master reads layers_revision + light_revision.
UPDATE maps
SET light_revision = light_revision + 1
WHERE id = $1
RETURNING light_revision;

-- name: GetMapGrid :one
-- A map's grid and its image's size, for the rows (package play, through
-- SessionMaps).
SELECT m.grid_columns, g.width AS image_width, g.height AS image_height
FROM maps AS m
JOIN gallery_images AS g ON g.id = m.image_id
WHERE m.campaign_id = $1 AND m.id = $2;

-- name: GetMapPointInCampaign :one
-- A point by its ID alone, if it is on one of the campaign's maps: the
-- battle point a combat starts from.
SELECT p.* FROM map_points AS p
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND p.id = $2;

-- name: UpsertMapTokenPosition :exec
-- Where a combatant ended its combat (package play): the token moves, or is
-- created visible (a player's character starts visible, like PlaceMapToken).
-- An existing token keeps its hidden flag.
INSERT INTO map_tokens (map_id, character_id, x_bp, y_bp, hidden, updated_at)
VALUES ($1, $2, $3, $4, false, $5)
ON CONFLICT (map_id, character_id) DO UPDATE
SET x_bp = excluded.x_bp, y_bp = excluded.y_bp, updated_at = excluded.updated_at;

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
INSERT INTO map_points (
    map_id, kind, name, description, hooks, show_dc, x_bp, y_bp, target_map_id,
    trap, trap_state, trap_triggered_at, treasure_value_po, light_preset, light_bright_ft, light_dim_ft, created_at, updated_at
)
VALUES (
    sqlc.arg(map_id), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(description), sqlc.arg(hooks), sqlc.arg(show_dc), sqlc.arg(x_bp), sqlc.arg(y_bp),
    sqlc.narg(target_map_id), sqlc.narg(trap), sqlc.narg(trap_state), sqlc.narg(trap_triggered_at), sqlc.narg(treasure_value_po),
    sqlc.narg(light_preset), sqlc.narg(light_bright_ft), sqlc.narg(light_dim_ft), sqlc.arg(now), sqlc.arg(now)
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
SET kind = sqlc.arg(kind), name = sqlc.arg(name), description = sqlc.arg(description), hooks = sqlc.arg(hooks),
    show_dc = sqlc.arg(show_dc), x_bp = sqlc.arg(x_bp), y_bp = sqlc.arg(y_bp), target_map_id = sqlc.narg(target_map_id),
    revealed_at = sqlc.narg(revealed_at), trap = sqlc.narg(trap), trap_state = sqlc.narg(trap_state), trap_triggered_at = sqlc.narg(trap_triggered_at),
    treasure_value_po = sqlc.narg(treasure_value_po), treasure_found_at = sqlc.narg(treasure_found_at),
    treasure_session_id = sqlc.narg(treasure_session_id), light_preset = sqlc.narg(light_preset),
    light_bright_ft = sqlc.narg(light_bright_ft), light_dim_ft = sqlc.narg(light_dim_ft), updated_at = sqlc.arg(now)
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

-- name: SetMapTokenCarriedLight :one
-- The light a character carries (MR-036): a preset's key, NULL for none. Moving
-- the token is a change to it (the master and the owner read it again).
UPDATE map_tokens
SET carried_light = sqlc.narg(carried_light), updated_at = sqlc.arg(now)
WHERE map_id = sqlc.arg(map_id) AND character_id = sqlc.arg(character_id)
RETURNING *;

-- name: ListPointRevealsOfMap :many
-- Who knows each trap of a map, for the master's read.
SELECT r.* FROM map_point_reveals AS r
JOIN map_points AS p ON p.id = r.point_id
WHERE p.map_id = $1
ORDER BY r.at, r.character_id;

-- name: ListPointRevealsOfCampaign :many
-- Every trap reveal of the campaign's maps, with whether the trap is already
-- visible to everyone: what a player's reads need to know which traps their
-- characters know (RN-10).
SELECT r.point_id, p.map_id, r.character_id,
       (p.revealed_at IS NOT NULL OR p.trap_triggered_at IS NOT NULL)::BOOL AS public
FROM map_point_reveals AS r
JOIN map_points AS p ON p.id = r.point_id
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1;

-- name: InsertPointReveal :execrows
-- Tells a character about a trap, once: a second reveal inserts nothing.
INSERT INTO map_point_reveals (point_id, character_id, how, at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (point_id, character_id) DO NOTHING;

-- name: SetTreasureFound :one
-- Marks a treasure found (MR-041). The session is the one open at that time, or
-- NULL. A treasure already found keeps its first time and session.
UPDATE map_points
SET treasure_found_at = COALESCE(treasure_found_at, sqlc.arg(found_at)::TIMESTAMPTZ),
    treasure_session_id = CASE WHEN treasure_found_at IS NULL THEN sqlc.narg(session_id)::UUID ELSE treasure_session_id END,
    updated_at = sqlc.arg(now)
WHERE map_id = sqlc.arg(map_id) AND id = sqlc.arg(id) AND kind = 'treasure'
RETURNING *;

-- name: ClearTreasureFound :one
-- Takes the found mark off a treasure.
UPDATE map_points
SET treasure_found_at = NULL, treasure_session_id = NULL, updated_at = sqlc.arg(now)
WHERE map_id = sqlc.arg(map_id) AND id = sqlc.arg(id) AND kind = 'treasure'
RETURNING *;

-- name: InsertTreasureFinder :exec
INSERT INTO map_treasure_finders (point_id, character_id)
VALUES ($1, $2)
ON CONFLICT (point_id, character_id) DO NOTHING;

-- name: DeleteTreasureFinders :exec
DELETE FROM map_treasure_finders
WHERE point_id = $1;

-- name: ListTreasureFindersOfMap :many
-- Who found each treasure of a map.
SELECT f.* FROM map_treasure_finders AS f
JOIN map_points AS p ON p.id = f.point_id
WHERE p.map_id = $1;

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

-- name: LeaveImage :execrows
-- Leaves the campaign's gallery image with the players (MR-028). Selecting
-- from gallery_images makes an image deleted meanwhile, or another
-- campaign's, insert nothing; an image already left keeps its left_at.
INSERT INTO campaign_left_images (campaign_id, image_id, left_at)
SELECT g.campaign_id, g.id, sqlc.arg(now)::TIMESTAMPTZ FROM gallery_images g
WHERE g.campaign_id = sqlc.arg(campaign_id) AND g.id = sqlc.arg(image_id)
ON CONFLICT (campaign_id, image_id) DO NOTHING;

-- name: ListLeftImages :many
-- The images left with the players, in the order they were left (id breaks
-- ties).
SELECT g.* FROM campaign_left_images l
JOIN gallery_images g ON g.id = l.image_id
WHERE l.campaign_id = $1
ORDER BY l.left_at, g.id;

-- name: TakeBackLeftImage :execrows
DELETE FROM campaign_left_images
WHERE campaign_id = $1 AND image_id = $2;

-- name: ImageIsLeft :one
-- Whether the image is left with the players. The image route asks it for a
-- player (RN-10).
SELECT EXISTS (
    SELECT 1 FROM campaign_left_images
    WHERE campaign_id = $1 AND image_id = $2
);

-- The actions of an RP scene (MR-015). A scene point has at most 20; the
-- handler counts them inside the transaction that inserts one.

-- name: ListSceneActions :many
-- One point's actions, in order.
SELECT * FROM scene_actions
WHERE point_id = $1
ORDER BY position, created_at, id;

-- name: ListSceneActionsOfMap :many
-- Every action of a map's points, for a map read: grouped by the handler,
-- each point's in order.
SELECT a.* FROM scene_actions AS a
JOIN map_points AS p ON p.id = a.point_id
WHERE p.map_id = $1
ORDER BY a.point_id, a.position, a.created_at, a.id;

-- name: CountSceneActions :one
SELECT count(*)::INT4 AS action_count FROM scene_actions
WHERE point_id = $1;

-- name: InsertSceneAction :one
INSERT INTO scene_actions (point_id, position, key, name, dc, max_attempts, created_at, updated_at)
VALUES (sqlc.arg(point_id), sqlc.arg(position), sqlc.arg(key), sqlc.arg(name), sqlc.narg(dc), sqlc.arg(max_attempts), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: GetSceneActionForUpdate :one
SELECT * FROM scene_actions
WHERE point_id = $1 AND id = $2
FOR UPDATE;

-- name: UpdateSceneAction :one
UPDATE scene_actions
SET key = sqlc.arg(key), name = sqlc.arg(name), dc = sqlc.narg(dc), max_attempts = sqlc.arg(max_attempts), updated_at = sqlc.arg(now)
WHERE point_id = sqlc.arg(point_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: SetSceneActionPosition :exec
UPDATE scene_actions
SET position = $2
WHERE id = $1;

-- name: DeleteSceneAction :execrows
DELETE FROM scene_actions
WHERE point_id = $1 AND id = $2;

-- name: DeleteSceneActionsOfPoint :exec
-- A point that stops being a scene has no actions.
DELETE FROM scene_actions
WHERE point_id = $1;

-- name: GetScenePoint :one
-- A SCENE point of the campaign's maps by its ID alone, hidden or not: the one
-- the master opens in a session (package play, through SessionMaps).
SELECT p.* FROM map_points AS p
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND p.id = $2 AND p.kind = 'scene';

-- name: ListSceneClues :many
-- A SCENE point's clues, in the master's order.
SELECT * FROM scene_clues
WHERE point_id = $1
ORDER BY position, created_at, id;

-- name: ListSceneCluesOfMap :many
-- Every clue of a map's points, for the master's map read: grouped by the
-- handler, each point's in order.
SELECT c.* FROM scene_clues AS c
JOIN map_points AS p ON p.id = c.point_id
WHERE p.map_id = $1
ORDER BY c.point_id, c.position, c.created_at, c.id;

-- name: InsertSceneClue :one
INSERT INTO scene_clues (point_id, position, text, created_at, updated_at)
VALUES (sqlc.arg(point_id), sqlc.arg(position), sqlc.arg(text), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: GetSceneClueForUpdate :one
SELECT * FROM scene_clues
WHERE point_id = $1 AND id = $2
FOR UPDATE;

-- name: UpdateSceneClue :one
UPDATE scene_clues
SET text = sqlc.arg(text), updated_at = sqlc.arg(now)
WHERE point_id = sqlc.arg(point_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: SetSceneCluePosition :exec
UPDATE scene_clues SET position = $2 WHERE id = $1;

-- name: DeleteSceneClue :execrows
DELETE FROM scene_clues
WHERE point_id = $1 AND id = $2;

-- name: DeleteSceneCluesOfPoint :exec
-- A point that stops being a scene has no clues. What players already
-- received stays (scene_clue_reveals keeps its own copy of the text).
DELETE FROM scene_clues
WHERE point_id = $1;

-- name: GetSceneClueInCampaign :one
-- A clue by its ID alone, if it is on a point of the campaign's maps: the one
-- the master reveals.
SELECT c.* FROM scene_clues AS c
JOIN map_points AS p ON p.id = c.point_id
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND c.id = $2;

-- name: InsertClueReveal :execrows
-- Gives a clue to a player, once: the unique index on (clue_id, user_id) turns
-- a second reveal into no row at all. It copies the clue's text, so what the
-- player received stays as it was said.
INSERT INTO scene_clue_reveals (campaign_id, clue_id, point_id, user_id, character_id, text, revealed_at)
VALUES (sqlc.arg(campaign_id), sqlc.arg(clue_id), sqlc.arg(point_id), sqlc.arg(user_id), sqlc.narg(character_id), sqlc.arg(text), sqlc.arg(now))
ON CONFLICT (clue_id, user_id) DO NOTHING;

-- name: ListClueRevealsOfPoint :many
-- Who has each clue of a point, oldest reveal first.
SELECT clue_id, character_id, revealed_at FROM scene_clue_reveals
WHERE point_id = $1 AND clue_id IS NOT NULL
ORDER BY revealed_at, id;

-- name: ListClueRevealsOfMap :many
-- Who has each clue of a map's points.
SELECT r.point_id, r.clue_id, r.character_id, r.revealed_at FROM scene_clue_reveals AS r
JOIN map_points AS p ON p.id = r.point_id
WHERE p.map_id = $1 AND r.clue_id IS NOT NULL
ORDER BY r.revealed_at, r.id;

-- name: UpsertSceneDiscovery :exec
-- The group discovered a scene (MR-030): the first time wins.
INSERT INTO scene_discoveries (campaign_id, point_id, discovered_at)
VALUES ($1, $2, $3)
ON CONFLICT (campaign_id, point_id) DO NOTHING;

-- name: ListDiscoveredScenes :many
-- The scenes the group discovered, with their current names, oldest discovery
-- first. A point that stopped being a scene is not listed.
SELECT p.id, p.name FROM scene_discoveries AS d
JOIN map_points AS p ON p.id = d.point_id
WHERE d.campaign_id = $1 AND p.kind = 'scene'
ORDER BY d.discovered_at, p.id;

-- name: ListReceivedClues :many
-- The clues revealed to a player in a campaign, newest first.
SELECT id, point_id, text, revealed_at FROM scene_clue_reveals
WHERE campaign_id = $1 AND user_id = $2
ORDER BY revealed_at DESC, id;

-- name: DeletePointReveals :exec
-- A point that stops being a trap tells nobody anything.
DELETE FROM map_point_reveals
WHERE point_id = $1;

-- name: ListPointRevealsOfPoint :many
-- Who knows one trap: the players to tell when it changes or goes away.
SELECT * FROM map_point_reveals
WHERE point_id = $1;

-- name: GetMapTreasureLocks :one
-- Whether the map holds a treasure that was found or converted: such a map
-- cannot be deleted, so a found treasure never vanishes from a session's summary.
SELECT (count(*) FILTER (WHERE treasure_found_at IS NOT NULL))::INT4 AS found,
       (count(*) FILTER (WHERE treasure_converted_award_id IS NOT NULL))::INT4 AS converted
FROM map_points
WHERE map_id = $1 AND kind = 'treasure';

-- What each player saw of a map with the fog of war on (MR-036, D6). The bytes are
-- a packed bitmap in package rules/grid's layout; the handlers size them by the
-- map's grid.

-- name: GetMapVisionMemory :one
-- One player's memory of a map; no row means they have seen nothing yet. The
-- caller compares epoch with the map's vision_epoch: an older one reads as empty.
SELECT seen, epoch FROM map_vision_memory
WHERE map_id = $1 AND user_id = $2;

-- name: ListMapVisionMemory :many
-- Every player's memory of a map (a stream's hint needs each player's view).
SELECT user_id, seen, epoch FROM map_vision_memory
WHERE map_id = $1;

-- name: UpsertMapVisionMemory :execrows
-- The caller already merged the new squares into the old bytes, so the memory
-- only grows. It writes only while the map is still in the epoch the bytes were
-- built for: a clear that happened meanwhile bumped it, and nothing is written.
INSERT INTO map_vision_memory (map_id, user_id, seen, epoch, updated_at)
SELECT m.id, sqlc.arg(user_id)::UUID, sqlc.arg(seen)::BYTEA, m.vision_epoch, sqlc.arg(updated_at)::TIMESTAMPTZ
FROM maps AS m
WHERE m.id = sqlc.arg(map_id) AND m.vision_epoch = sqlc.arg(epoch)
ON CONFLICT (map_id, user_id) DO UPDATE
SET seen = excluded.seen, epoch = excluded.epoch, updated_at = excluded.updated_at;

-- name: ClearMapVisionMemory :execrows
-- "Esquecer o que foi visto", and a new grid or image: every player forgets. The
-- epoch goes up, so a refresh that was already running cannot write the old bitmap.
UPDATE maps SET vision_epoch = vision_epoch + 1
WHERE id = $1;

-- name: DeleteMapVisionMemory :execrows
-- The rows of the cleared epochs (they already read as empty); tidying only.
DELETE FROM map_vision_memory
WHERE map_id = $1;

-- name: ImageIsOnAFogMap :one
-- Whether the image is the background of a map with the fog of war on: a player
-- never receives it, whatever else shows it (RN-10, MR-036). The image route asks
-- it for a player. A map's image is found by maps_image_id_idx.
SELECT EXISTS (
    SELECT 1 FROM maps
    WHERE campaign_id = $1 AND image_id = $2 AND fog_enabled
);

-- name: ImageIsUsedElsewhere :one
-- Whether the image is also used another way than as the background of the
-- given map: the background of any other map, or an image the campaign left with
-- the players. Turning the fog on copies such an image first, so the fog map's
-- image is its own.
SELECT (
    EXISTS (
        SELECT 1 FROM maps AS m
        WHERE m.campaign_id = sqlc.arg(campaign_id) AND m.image_id = sqlc.arg(image_id) AND m.id <> sqlc.arg(map_id)
    )
    OR EXISTS (
        SELECT 1 FROM campaign_left_images AS l
        WHERE l.campaign_id = sqlc.arg(campaign_id) AND l.image_id = sqlc.arg(image_id)
    )
)::BOOL AS used;

-- name: SetMapImageOnly :one
-- A new copy of the image becomes the map's, without touching the name or the
-- revision's guard: the map's revision still moves, since the image changed.
UPDATE maps
SET image_id = sqlc.arg(image_id), revision = revision + 1, updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id) AND id = sqlc.arg(id)
RETURNING *;

-- Gold (slice 9.11, MR-032): the session summary's "Mais tesouro encontrado".

-- name: ListTreasureFindsOfSession :many
-- One row per treasure and finder of the treasures found while the game session
-- was open. A treasure unmarked later has no session and no finders.
SELECT p.id AS point_id, COALESCE(p.treasure_value_po, 0)::INT4 AS value_po, f.character_id
FROM map_points AS p
JOIN map_treasure_finders AS f ON f.point_id = p.id
WHERE p.kind = 'treasure' AND p.treasure_found_at IS NOT NULL AND p.treasure_session_id = sqlc.arg(session_id)::UUID
ORDER BY p.id, f.character_id;

-- name: SetTrapState :one
-- The live game changes a trap's state (MR-035, slice 9.8): it fires (state
-- 'triggered', with when), the master disarms it, or an undo puts back what
-- there was. The caller locked the point first.
UPDATE map_points
SET trap_state = sqlc.arg(trap_state), trap_triggered_at = sqlc.narg(trap_triggered_at), updated_at = sqlc.arg(now)
WHERE map_id = sqlc.arg(map_id) AND id = sqlc.arg(id) AND kind = 'trap'
RETURNING *;

-- name: ListTrapsOfMap :many
-- The map's traps, oldest first: what noticing, searching and firing read.
SELECT * FROM map_points
WHERE map_id = $1 AND kind = 'trap'
ORDER BY created_at, id;

-- name: ListPointRevealCharacters :many
-- The characters that know each trap of the map, for the noticing: one row per
-- trap and character.
SELECT r.point_id, r.character_id FROM map_point_reveals AS r
JOIN map_points AS p ON p.id = r.point_id
WHERE p.map_id = $1;

-- name: GetMapPoint :one
-- One point of a map, without locking it.
SELECT * FROM map_points
WHERE map_id = $1 AND id = $2;

-- name: ListTrapNamesInCampaign :many
-- The names of traps by point ID, for the combat log: a trap that fired is
-- public, so its name may be told (MR-035). A deleted point is simply absent.
SELECT p.id, p.name FROM map_points AS p
JOIN maps AS m ON m.id = p.map_id
WHERE m.campaign_id = $1 AND p.kind = 'trap' AND p.id = ANY($2::uuid[]);
