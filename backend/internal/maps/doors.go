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

// The doors a move opens (MR-010, RN-26). The doors layer is painted by the
// master (PaintMapCells), and a creature that walks into a closed door opens it:
// play asks for that here, inside the move's transaction, so the door and the
// move stand or fall together. Like Terrain and the other seams between modules
// these take no caller and check nobody: they run after play's own
// authorization.

// OpenDoors opens, inside tx, the doors at the squares of the map that are still
// closed, and returns the squares it opened, one for each door (a square whose
// door is no longer closed, because the master changed it since the move was
// planned, is left out; on a calibrated map the whole drawn square opens, see below).
// It holds the map's row lock, as painting does, so the two take turns, and it
// raises the layers' revision once when it opened any. It implements
// play.DoorKeeper. The map must be the campaign's, or the answer is a
// `not_found` Connect error.
func (s *Service) OpenDoors(ctx context.Context, tx pgx.Tx, campaignID, mapID string, squares []grid.Square) ([]grid.Square, error) {
	id, ok := uid.Canonical(mapID)
	if !ok {
		return nil, errMapNotFound()
	}
	q := queriesIn(s.queries, tx)
	if _, err := q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: campaignID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
		return nil, errMapNotFound()
	} else if err != nil {
		return nil, fmt.Errorf("lock the map: %w", err)
	}
	size, err := q.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: campaignID, ID: id})
	if err != nil {
		return nil, fmt.Errorf("read the map's grid: %w", err)
	}
	g := gridOf(size.GridColumns, size.GridFactor, size.ImageWidth, size.ImageHeight)
	if !g.Valid() {
		return nil, nil
	}
	set, found, err := storedDoorLayers(ctx, q, id, g)
	if err != nil || !found {
		return nil, err
	}
	opened := openClosedDoors(set, squares, max(int(size.GridFactor), 1))
	if len(opened) == 0 {
		return nil, nil
	}
	if err := storeLayers(ctx, q, id, set, s.now()); err != nil {
		return nil, err
	}
	if _, err := q.BumpMapLayersRevision(ctx, id); err != nil {
		return nil, fmt.Errorf("bump the layers' revision: %w", err)
	}
	return opened, nil
}

// storedDoorLayers reads the layers a map has stored; found is false when nothing was
// ever painted on it, so there is no door.
func storedDoorLayers(ctx context.Context, q *mapsdb.Queries, mapID string, g grid.Grid) (set layerSet, found bool, err error) {
	stored, err := q.GetMapLayers(ctx, mapID)
	if errors.Is(err, pgx.ErrNoRows) {
		return layerSet{}, false, nil // nothing painted: no door
	}
	if err != nil {
		return layerSet{}, false, fmt.Errorf("read the layers: %w", err)
	}
	return loadLayers(stored, g), true, nil
}

// openClosedDoors opens the closed doors at the squares and returns the squares it
// opened. A door is a whole square of the drawing (MR-025): on a calibrated map it is a
// block of factor x factor squares, and crossing one opens every closed square of its
// block, once: one square comes back for the block, so play writes one event.
func openClosedDoors(set layerSet, squares []grid.Square, factor int) []grid.Square {
	var opened []grid.Square
	for _, sq := range squares {
		if set.doors.At(sq) != grid.DoorClosed {
			continue
		}
		bc, br := sq.Col/factor*factor, sq.Row/factor*factor
		for r := br; r < br+factor; r++ {
			for c := bc; c < bc+factor; c++ {
				if set.doors.Get(c, r) == grid.DoorClosed {
					set.doors.Set(c, r, grid.DoorOpen)
				}
			}
		}
		opened = append(opened, sq)
	}
	return opened
}

// DoorsChanged tells the watchers that doors of the map were opened, after the
// commit of the move that opened them: the screens read the layers again, and on
// a fog map what each player sees and remembers is worked out again (the open
// door lets sight through). It never fails the move. It implements
// play.DoorKeeper.
func (s *Service) DoorsChanged(ctx context.Context, campaignID, mapID string) {
	id, ok := uid.Canonical(mapID)
	if !ok {
		return
	}
	row, err := s.queries.GetMap(ctx, mapsdb.GetMapParams{CampaignID: campaignID, ID: id})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.ErrorContext(ctx, "maps: read the map of the doors that were opened", "error", err)
		}
		return
	}
	s.layersChanged(ctx, campaignID, row, true, true) // an open door lets sight through: the players read it
}
