package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// keepApartSquares is how close to a player's character a creature is not placed
// on its own: 10 ft, two squares by the distance of an attack (RN-21), so it never
// starts within the reach of one.
const keepApartSquares = 2

// halves splits a side of the grid in two: the middle of the map.
const halves = 2

// placement picks the squares of the combatants that come to a map without one
// ("Colocar no mapa", the NPCs when the combat begins). A square is allowed when a
// creature can stand on it (no wall, no column, no shut door) and nobody is on it.
// Among the allowed squares, the ones farther than keepApartSquares from every
// player's character come first, and the one nearest the middle of the fight wins;
// when none is that far, the allowed square farthest from the players wins.
type placement struct {
	terrain grid.Terrain
	taken   map[grid.Square]bool
	party   []grid.Square
	center  grid.Square
}

// newPlacement works out the squares in play: who stands where among cs, and the
// middle of the fight, the battle point when the combat came from one on this very
// map, else the middle of the grid.
func newPlacement(terrain grid.Terrain, cs []playdb.Combatant, center *grid.Square) *placement {
	p := &placement{terrain: terrain, taken: map[grid.Square]bool{}}
	for _, c := range cs {
		if !placed(c) {
			continue
		}
		sq := squareOfCombatant(c)
		p.taken[sq] = true
		if c.Kind == kindPlayer {
			p.party = append(p.party, sq)
		}
	}
	p.center = grid.Square{Col: terrain.Grid.Columns / halves, Row: terrain.Grid.Rows / halves}
	if center != nil && terrain.Grid.Contains(*center) {
		p.center = *center
	}
	return p
}

// apart is how many squares the square is from the nearest player's character,
// math.MaxInt when there is none.
func (p *placement) apart(sq grid.Square) int {
	d := math.MaxInt
	for _, o := range p.party {
		d = min(d, grid.RangeSquares(o, sq))
	}
	return d
}

// spot is an allowed square and how it ranks.
type spot struct {
	sq    grid.Square
	apart int // squares from the nearest player's character
	away  int // squared distance from the middle of the fight
}

// far says the square is out of reach of every player's character.
func (c spot) far() bool { return c.apart > keepApartSquares }

// beats says whether c is the better square: a far one over one that is not, the
// nearer the middle among the far, and the farther from the players among the rest
// (the middle breaks a tie).
func (c spot) beats(o spot) bool {
	switch {
	case c.far() != o.far():
		return c.far()
	case !c.far() && c.apart != o.apart:
		return c.apart > o.apart
	}
	return c.away < o.away
}

// next picks the square for one more combatant and takes it, or reports that the
// map has no square left that a creature can stand on. Squares are read row by
// row, so a tie goes to the first.
func (p *placement) next() (grid.Square, bool) {
	var best spot
	found := false
	for row := range p.terrain.Grid.Rows {
		for col := range p.terrain.Grid.Columns {
			sq := grid.Square{Col: col, Row: row}
			if p.taken[sq] || !p.terrain.Standable(sq) {
				continue
			}
			c := spot{sq: sq, apart: p.apart(sq), away: centerDistance(p.center, sq)}
			if !found || c.beats(best) {
				best, found = c, true
			}
		}
	}
	if found {
		p.taken[best.sq] = true
	}
	return best.sq, found
}

// centerDistance orders the squares by how close they are to the middle: the
// squared straight distance, exact in integers.
func centerDistance(a, b grid.Square) int {
	dc, dr := a.Col-b.Col, a.Row-b.Row
	return dc*dc + dr*dr
}

// placementFor reads what the placement needs in the transaction: the terrain, the
// battle point the combat came from, and who stands where in cs.
func (s *Service) placementFor(ctx context.Context, c *combatTx, cs []playdb.Combatant) (*placement, error) {
	terrain, err := s.terrainOf(ctx, c.tx, c.session.CampaignID, c.enc)
	if err != nil {
		return nil, err
	}
	var center *grid.Square
	if c.enc.MapPointID != nil && c.enc.MapID != nil {
		point, err := s.maps.BattlePoint(ctx, c.tx, c.session.CampaignID, *c.enc.MapPointID)
		switch {
		case connect.CodeOf(err) == connect.CodeNotFound: // the master took the point away: the middle of the map
		case err != nil:
			return nil, fmt.Errorf("read the battle point: %w", err)
		case point.MapID == *c.enc.MapID: // a fight on another map is not where the point is
			col, row := squareOf(link.Grid{Columns: int32(terrain.Grid.Columns), Rows: int32(terrain.Grid.Rows)}, point.XBP, point.YBP) //nolint:gosec // a grid is at most 200 by 400
			center = &grid.Square{Col: int(col), Row: int(row)}
		}
	}
	return newPlacement(terrain, cs, center), nil
}

// placeUnplaced puts every NPC of the combat that has no square on one, by the
// placement rule, so a combat on a map never starts with creatures off it. A map
// with no room left leaves the rest where they are. It returns the combatants as
// they are afterwards.
func (s *Service) placeUnplaced(ctx context.Context, c *combatTx, cs []playdb.Combatant) ([]playdb.Combatant, error) {
	if isTheatre(c.enc) || !slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.Kind == kindNPC && !placed(o) }) {
		return cs, nil
	}
	p, err := s.placementFor(ctx, c, cs)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(cs)
	for i, o := range out {
		if o.Kind != kindNPC || placed(o) {
			continue
		}
		sq, ok := p.next()
		if !ok {
			break
		}
		col, row := clamp32(sq.Col, 0, math.MaxInt32), clamp32(sq.Row, 0, math.MaxInt32)
		if err := c.q.SetCombatantMove(ctx, playdb.SetCombatantMoveParams{
			ID: o.ID, GridCol: &col, GridRow: &row,
			MovementUsedFt: o.MovementUsedFt, MovementUsedDft: o.MovementUsedDft, LastMoveDft: o.LastMoveDft, CoverMark: o.CoverMark,
		}); err != nil {
			return nil, fmt.Errorf("place a combatant: %w", err)
		}
		out[i].GridCol, out[i].GridRow = &col, &row
	}
	return out, nil
}

// errNoRoom is the refusal for a placement on a map with no square left.
func errNoRoom() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_FREE_SQUARE, "there is no free square on the map")
}

// errWall is the refusal for a master's move or placement onto a wall.
func errWall() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WALL_ON_SQUARE, "that square is a wall")
}

var errAlreadyPlaced = connect.NewError(connect.CodeInvalidArgument, errors.New("the combatant is already on the map"))
