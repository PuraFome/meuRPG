package grid

import "slices"

// Formation is the shape a group takes when it is put on the map.
type Formation int

// The shapes of a group.
const (
	// FormationLine puts the group side by side, to the right of the anchor.
	FormationLine Formation = iota
	// FormationBlock puts it together, in rows of at most two.
	FormationBlock
	// FormationArc puts it on a half circle in front of the anchor, the anchor in the middle.
	FormationArc
)

// Arrange chooses n squares for a group put on the map at anchor in the shape f: the shape's own
// squares in order, each one that is not standable or is taken (taken, or an earlier one) going to the
// nearest free standable square instead. It returns fewer than n squares when the map has no room for
// more. The first square of a shape is the anchor when it is free.
func Arrange(t Terrain, anchor Square, n int, f Formation, taken Set) []Square {
	out := make([]Square, 0, n)
	free := func(s Square) bool { return t.Standable(s) && !inSet(taken, s) && !slices.Contains(out, s) }
	for _, want := range FormationSquares(anchor, n, f) {
		if free(want) {
			out = append(out, want)
			continue
		}
		if s, ok := nearestFree(t, want, free); ok {
			out = append(out, s)
		}
	}
	return out
}

// FormationSquares is the squares of a formation of n at anchor, ignoring the terrain and
// whoever stands there: the shape Arrange starts from.
func FormationSquares(anchor Square, n int, f Formation) []Square {
	out := make([]Square, 0, n)
	for i := range n {
		switch f {
		case FormationBlock:
			out = append(out, Square{Col: anchor.Col + i%2, Row: anchor.Row + i/2})
		case FormationArc:
			out = append(out, arcSquare(anchor, i))
		default:
			out = append(out, Square{Col: anchor.Col + i, Row: anchor.Row})
		}
	}
	return out
}

// arcSquare is the i-th square of an arc: the anchor first, then the squares on both sides of it, each
// pair one square further back.
func arcSquare(anchor Square, i int) Square {
	if i == 0 {
		return anchor
	}
	side := 1
	if i%2 == 0 {
		side = -1
	}
	k := (i + 1) / 2
	return Square{Col: anchor.Col + side*k, Row: anchor.Row + (k+1)/2}
}

// nearestFree is the free square closest to want, found by rings of growing distance; ties go to the
// first in row-major order, so the answer does not depend on map order.
func nearestFree(t Terrain, want Square, free func(Square) bool) (Square, bool) {
	limit := max(t.Grid.Columns, t.Grid.Rows)
	for d := 1; d <= limit; d++ {
		for dr := -d; dr <= d; dr++ {
			for dc := -d; dc <= d; dc++ {
				if max(abs(dc), abs(dr)) != d {
					continue
				}
				if s := (Square{Col: want.Col + dc, Row: want.Row + dr}); t.Grid.Contains(s) && free(s) {
					return s, true
				}
			}
		}
	}
	return Square{}, false
}
