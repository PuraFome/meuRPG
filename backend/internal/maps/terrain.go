package maps

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Terrain is what a combat on the map runs over (MR-034, RN-21, D2): the grid
// and the walls, the difficult terrain and the cover the master painted. It
// implements play.TerrainSource. Like the other seams between modules it takes
// no caller and checks nobody: it runs after play's own authorization, and
// play never sends a layer to a player (it only walks the squares). The map
// must be the campaign's, or the answer is a `not_found` Connect error. A map
// with no grid has an empty Terrain (the zero Grid), and one with no layers
// row is open floor. The light layer is not part of it.
func (s *Service) Terrain(ctx context.Context, campaignID, mapID string) (grid.Terrain, error) {
	id, ok := parseID(mapID)
	if !ok {
		return grid.Terrain{}, errMapNotFound()
	}
	cm, err := s.loadMaps(ctx, campaignID)
	if err != nil {
		return grid.Terrain{}, fmt.Errorf("read the map: %w", err)
	}
	row, ok := cm.byID[id]
	if !ok {
		return grid.Terrain{}, errMapNotFound()
	}
	g := gridOf(row.GridColumns, row.ImageWidth, row.ImageHeight)
	if !g.Valid() {
		return grid.Terrain{}, nil
	}
	stored, err := s.queries.GetMapLayers(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return grid.Terrain{}, fmt.Errorf("read the layers: %w", err)
	}
	set := loadLayers(stored, g)
	return grid.Terrain{Grid: g, Walls: set.walls, Difficult: set.terrain, Cover: set.cover}, nil
}
