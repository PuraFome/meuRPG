package maps

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// What the play module asks of the fog of war while a combat runs on a map
// (MR-036, RN-10, slice 9.7): who sees which square now, and the terrain a
// player knows. Like Terrain and the other seams between modules they take no
// caller and check nobody: they run after play's own authorization, and play only
// uses them to decide which combatants and which squares a player may be told.

// fogRow reads the map's row and its grid, and says whether it has the fog on.
func (s *Service) fogRow(ctx context.Context, campaignID, mapID string) (fogInput, mapsdb.Map, bool, error) {
	id, ok := parseID(mapID)
	if !ok {
		return fogInput{}, mapsdb.Map{}, false, nil
	}
	row, err := s.queries.GetMap(ctx, mapsdb.GetMapParams{CampaignID: campaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return fogInput{}, mapsdb.Map{}, false, nil // a map the master deleted: no fog
	}
	if err != nil {
		return fogInput{}, mapsdb.Map{}, false, fmt.Errorf("find the map: %w", err)
	}
	if !fogged(row) {
		return fogInput{}, row, false, nil
	}
	size, err := s.queries.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: campaignID, ID: id})
	if err != nil {
		return fogInput{}, row, false, fmt.Errorf("read the map's grid: %w", err)
	}
	return fogInputOfRow(row, size.ImageWidth, size.ImageHeight), row, true, nil
}

// combatSight is link.CombatSight over a map's scene. A player's view and known
// terrain are worked out once, the first time they are asked for.
type combatSight struct {
	s     *Service
	in    fogInput
	sg    *sight
	mu    sync.Mutex
	views map[string]*vision.View
	known map[string]grid.Terrain
}

func (c *combatSight) Users() []string { return c.sg.users() }

func (c *combatSight) view(userID string) *vision.View {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.views[userID]; ok {
		return v
	}
	v, _ := c.sg.viewOf(userID)
	c.views[userID] = v
	return v
}

// Sees counts the grey of darkvision and the light of the dark as seen, and a
// wall (seen because a seen square touches it) as nothing: no creature stands in one.
func (c *combatSight) Sees(userID string, sq grid.Square) bool {
	return c.view(userID).At(sq) >= vision.SeenGrey
}

// CanSee is a pair check, never a whole view: the light at the square for this
// viewer's senses, and one line.
func (c *combatSight) CanSee(from grid.Square, senses vision.Senses, to grid.Square) bool {
	return c.sg.entry.lit.CanSee(vision.Viewer{At: from, Senses: senses}, to)
}

// KnownTerrain is the terrain the player knows: the walls, the difficult terrain,
// the cover and the doors (as a player knows them) of the squares they see now or remember, and plain floor
// everywhere else (D1: the move is planned on what the player knows, and cut short
// by what they do not).
func (c *combatSight) KnownTerrain(ctx context.Context, tx pgx.Tx, userID string) (grid.Terrain, error) {
	c.mu.Lock()
	t, ok := c.known[userID]
	c.mu.Unlock()
	if ok {
		return t, nil
	}
	memory, err := c.s.memoryOf(ctx, tx, c.in, userID)
	if err != nil {
		return grid.Terrain{}, err
	}
	pv := c.sg.newView(userID, memory)
	set := filterLayers(*pv.layers, pv)
	t = grid.Terrain{Grid: c.in.g, Walls: set.walls, Difficult: set.terrain, Cover: set.cover, Doors: set.doors}
	c.mu.Lock()
	c.known[userID] = t
	c.mu.Unlock()
	return t, nil
}

// CombatSight returns what the players see of the map now, for a combat on it,
// or nil when the map has no fog of war (or is gone): then every player sees
// every combatant that is not hidden, as before. The party is read once, here. It
// implements play.FogSource.
func (s *Service) CombatSight(ctx context.Context, campaignID, mapID string) (link.CombatSight, error) {
	in, row, on, err := s.fogRow(ctx, campaignID, mapID)
	if err != nil || !on {
		return nil, err
	}
	points, err := s.queries.ListMapPoints(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("list the points: %w", err)
	}
	tokens, err := s.queries.ListMapTokens(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("list the tokens: %w", err)
	}
	sg, err := s.newSight(ctx, nil, in, points, tokens)
	if err != nil {
		return nil, err
	}
	return &combatSight{s: s, in: in, sg: sg, views: map[string]*vision.View{}, known: map[string]grid.Terrain{}}, nil
}
