package maps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The calibration of a map (MR-025, RN-25): each square of the drawing is worth
// `factor` squares of 1,5 m, and the grid the rules use is the drawing's times
// the factor. Changing the factor to a multiple of the old one, over the same
// drawing, keeps what the master painted and what the players saw: every square
// becomes a block of step x step squares (rules/grid, Scaled). Any other change
// clears it (clearLayers), because a layer only fits the grid it was made on.
//
//   - The painted layers (walls, terrain, cover, light, doors) are scaled square by
//     square. A door is the whole drawn square, so it becomes a doorway as wide as
//     the old square: every cell of the block holds the same door, and walking
//     through opens them as it did one by one (the door rules never asked for floor
//     beside a door).
//   - The players' memory (map_vision_memory) is a bitmap of the old grid: it is
//     scaled the same way, so what they saw stays seen.
//   - Points and tokens are positions in basis points of the image: they do not
//     move, and fall in the right block of squares by themselves. A trap's
//     area_size is in squares of 1,5 m and stays: a 2-square trap of a 3 m drawing
//     covers the same squares, a smaller part of the new, bigger-looking square.
//   - The layers' revision goes up when a layers row exists (the caller then tells
//     the players, and forgets the caches that depended on the grid).

// scaleLayers rewrites a map's layers and the players' memory for a grid step
// times bigger in each direction. old is the grid they are sized for, epoch the
// map's vision epoch: a memory of an older epoch is already empty and is left
// alone, and the scaled ones are written in the next epoch, which it starts. It reads and writes through q (a transaction's).
func scaleLayers(ctx context.Context, q *mapsdb.Queries, mapID string, epoch int32, old grid.Grid, step int, now time.Time) error {
	stored, err := q.GetMapLayers(ctx, mapID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// nothing painted: nothing to scale, no revision to bump
	case err != nil:
		return fmt.Errorf("read the layers: %w", err)
	default:
		set := loadLayers(stored, old)
		err = q.UpsertMapLayers(ctx, mapsdb.UpsertMapLayersParams{
			MapID: mapID, DifficultTerrain: nilIfBlank(set.terrain.Scaled(step).Encode()), Walls: nilIfBlank(set.walls.Scaled(step).Encode()),
			Cover: nilIfBlank(set.cover.Scaled(step).Encode()), Light: nilIfBlank(set.light.Scaled(step).Encode()),
			Doors: nilIfBlank(set.doors.Scaled(step).Encode()), Now: now,
		})
		if err != nil {
			return fmt.Errorf("write the scaled layers: %w", err)
		}
		if _, err := q.BumpMapLayersRevision(ctx, mapID); err != nil {
			return fmt.Errorf("bump the layers' revision: %w", err)
		}
	}
	// The memory is a new generation: a refresh that read the map before this change
	// still holds the old grid's bitmap and must not write it back over the scaled
	// one (UpsertMapVisionMemory writes only in the epoch it was built for).
	if _, err := q.ClearMapVisionMemory(ctx, mapID); err != nil {
		return fmt.Errorf("start a new generation of what the players saw: %w", err)
	}
	memories, err := q.ListMapVisionMemory(ctx, mapID)
	if err != nil {
		return fmt.Errorf("read what the players saw: %w", err)
	}
	for _, mem := range memories {
		if mem.Epoch != epoch {
			continue
		}
		// Bytes that do not fit the old grid are damaged: the player forgets them
		// rather than the map holding a bitmap of the wrong size.
		seen := grid.NewLayer(grid.Grid{Columns: old.Columns * step, Rows: old.Rows * step})
		if l, err := grid.DecodeLayer(old, mem.Seen); err == nil {
			seen = l.Scaled(step)
		}
		if err := q.SetMapVisionMemorySeen(ctx, mapsdb.SetMapVisionMemorySeenParams{
			MapID: mapID, UserID: mem.UserID, Seen: seen.Encode(), Epoch: epoch + 1, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("scale what a player saw: %w", err)
		}
	}
	return nil
}
