package maps

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Terrain is what a combat on the map runs over (MR-034, RN-21, D2): the grid
// and the walls, the difficult terrain, the cover, the doors and the surfaces (water, climbable) the master painted
// (as painted: the truth, locked and secret doors included). It
// implements play.TerrainSource. Like the other seams between modules it takes
// no caller and checks nobody: it runs after play's own authorization, and
// play never sends a layer to a player (it only walks the squares). The map
// must be the campaign's, or the answer is a `not_found` Connect error. A map
// with no grid has an empty Terrain (the zero Grid), and one with no layers
// row is open floor. The light layer is not part of it. It reads inside tx when
// the caller has one (a combat's change does; nil: the pool).
func (s *Service) Terrain(ctx context.Context, tx pgx.Tx, campaignID, mapID string) (grid.Terrain, error) {
	id, ok := uid.Canonical(mapID)
	if !ok {
		return grid.Terrain{}, errMapNotFound()
	}
	q := queriesIn(s.queries, tx)
	row, err := q.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: campaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return grid.Terrain{}, errMapNotFound()
	}
	if err != nil {
		return grid.Terrain{}, fmt.Errorf("read the map: %w", err)
	}
	g := gridOf(row.GridColumns, row.GridFactor, row.ImageWidth, row.ImageHeight)
	if !g.Valid() {
		return grid.Terrain{}, nil
	}
	stored, err := q.GetMapLayers(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return grid.Terrain{}, fmt.Errorf("read the layers: %w", err)
	}
	set := loadLayers(stored, g)
	return grid.Terrain{Grid: g, Walls: set.walls, Difficult: set.terrain, Cover: set.cover, Doors: set.doors, NoPass: set.noPass, Surface: set.surface}, nil
}

// Surface is the map's surface layer alone (water, climbable), as the grid g reads it: one indexed read of one column, for the
// counter of a combat that only needs what a combatant stands on (#819). A map with no layers row, or one whose layer does not
// fit g, is all floor. It implements play.SurfaceSource, and like Terrain it checks nobody: play decides who sees what.
func (s *Service) Surface(ctx context.Context, tx pgx.Tx, mapID string, g grid.Grid) (*grid.SurfaceLayer, error) {
	id, ok := uid.Canonical(mapID)
	if !ok {
		return nil, errMapNotFound()
	}
	raw, err := queriesIn(s.queries, tx).GetMapSurface(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return grid.NewSurfaceLayer(g), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the surface layer: %w", err)
	}
	l, err := grid.DecodeSurfaceLayer(g, raw)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, grid.ErrLayerSize) {
		return nil, fmt.Errorf("read the surface layer: %w", err)
	}
	return grid.NewSurfaceLayer(g), nil // bytes of another grid read as nothing painted, as loadLayers does
}
