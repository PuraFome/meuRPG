package play

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// A zone as the engine reads it: the row, its squares as a set and the turn each creature was last
// hit on. Everything that changes a zone goes through the helpers of this file, so the squares
// and the origin stay in step.

// zone is a spell_zones row with its parts read.
type zone struct {
	row     playdb.SpellZone
	def     zoneDef
	squares []grid.Square
	in      map[grid.Square]bool
	marks   map[string]string
}

func (z *zone) origin() grid.Square {
	return grid.Square{Col: int(z.row.OriginCol), Row: int(z.row.OriginRow)}
}

// has says whether the zone covers the square.
func (z *zone) has(sq grid.Square) bool { return z.in[sq] }

// readZone reads a row; ok is false for a zone of a spell the catalog no longer knows.
func readZone(row playdb.SpellZone) (*zone, bool) {
	def, ok := zoneDefOf(row.SpellKey)
	if !ok {
		return nil, false
	}
	z := &zone{row: row, def: def, marks: map[string]string{}}
	z.setSquares(squaresOfJSON(row.Squares))
	_ = json.Unmarshal(row.HitMarks, &z.marks) // a value this package wrote
	return z, true
}

func (z *zone) setSquares(sq []grid.Square) {
	z.squares, z.in = sq, make(map[grid.Square]bool, len(sq))
	for _, s := range sq {
		z.in[s] = true
	}
}

// squaresOfJSON reads the squares a row keeps, [[col,row],...].
func squaresOfJSON(raw []byte) []grid.Square {
	var pairs [][2]int
	_ = json.Unmarshal(raw, &pairs) // a value this package wrote
	out := make([]grid.Square, len(pairs))
	for i, p := range pairs {
		out[i] = grid.Square{Col: p[0], Row: p[1]}
	}
	return out
}

func squaresToJSON(sq []grid.Square) []byte {
	pairs := make([][2]int, len(sq))
	for i, s := range sq {
		pairs[i] = [2]int{s.Col, s.Row}
	}
	body, _ := json.Marshal(pairs) // ints cannot fail to encode
	return body
}

// listZones reads the zones of the combat, oldest first.
func listZones(ctx context.Context, c *combatTx) ([]*zone, error) {
	return readZonesOf(ctx, c.q, c.enc.ID)
}

// readZonesOf reads the zones of a combat.
func readZonesOf(ctx context.Context, q *playdb.Queries, encounterID string) ([]*zone, error) {
	rows, err := q.ListSpellZones(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("list the spell zones: %w", err)
	}
	return zonesOfRows(rows), nil
}

// zonesOfRows reads rows into zones, leaving out the ones of a spell the catalog does not know.
func zonesOfRows(rows []playdb.SpellZone) []*zone {
	out := make([]*zone, 0, len(rows))
	for _, row := range rows {
		if z, ok := readZone(row); ok {
			out = append(out, z)
		}
	}
	return out
}

// place writes where the zone stands now: its origin and the squares it covers there.
func (z *zone) place(ctx context.Context, c *combatTx, origin grid.Square, squares []grid.Square) error {
	z.row.OriginCol, z.row.OriginRow = clamp32(origin.Col, 0, math.MaxInt32), clamp32(origin.Row, 0, math.MaxInt32)
	z.setSquares(squares)
	z.row.Squares = squaresToJSON(squares)
	if err := c.q.SetSpellZonePlace(ctx, playdb.SetSpellZonePlaceParams{
		ID: z.row.ID, OriginCol: z.row.OriginCol, OriginRow: z.row.OriginRow, Squares: z.row.Squares,
	}); err != nil {
		return fmt.Errorf("move the spell zone: %w", err)
	}
	return nil
}

// turnMark names a turn: the round and the first combatant of the group on turn. A creature is hit
// once by a zone for each turn mark.
func turnMark(round int32, leader string) string { return strconv.Itoa(int(round)) + ":" + leader }

// hitOn says whether the zone already caught the creature on the turn.
func (z *zone) hitOn(who, mark string) bool { return z.marks[who] == mark }

// markHit writes that the zone caught the creature on the turn.
func (z *zone) markHit(ctx context.Context, c *combatTx, who, mark string) error {
	z.marks[who] = mark
	body, _ := json.Marshal(z.marks) // strings cannot fail to encode
	if err := c.q.SetSpellZoneHitMarks(ctx, playdb.SetSpellZoneHitMarksParams{ID: z.row.ID, HitMarks: body}); err != nil {
		return fmt.Errorf("mark the creature the zone hit: %w", err)
	}
	return nil
}

// owned are the zones of the combatant.
func owned(zs []*zone, owner string) []*zone {
	return slices.DeleteFunc(slices.Clone(zs), func(z *zone) bool { return z.row.OwnerID != owner })
}

// roundsLeft are the rounds a zone still has at the round: 0 for one with no round count.
func (z *zone) roundsLeft(round int32) int32 {
	if z.row.EndsRound == nil {
		return 0
	}
	return max(*z.row.EndsRound-round, 0)
}
