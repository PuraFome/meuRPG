package grid

import "errors"

// The calibration of a map (MR-025, RN-25). The rules count in squares of 5 ft
// (1,5 m) on every map; a map drawn with bigger squares says how many rules
// squares fit in each square of the drawing, the factor: 1 is 1,5 m (the
// default), 2 is 3 m, 3 is 4,5 m. The grid the engine uses, and the one every
// layer is sized by, is the drawn columns and rows times the factor.

// MaxFactor is the largest calibration: 20 squares of 1,5 m (30 m) to a square
// of the drawing. A drawing never has fewer than 4 columns, so the engine's
// 200 columns cap it at 50 anyway; this bound only keeps the number sane.
const MaxFactor = 20

// ErrGridLimits is returned for a calibration whose engine grid would pass
// MaxColumns x MaxRows.
var ErrGridLimits = errors.New("grid: the engine grid would pass its limits")

// EngineGrid is the grid the rules use for a drawing of drawnColumns columns on
// an image of width x height pixels, each drawn square worth factor squares of
// 1,5 m. The drawn rows follow the image's proportions (RowsFor) and the engine's
// are exactly factor times them, so a drawn square is a whole block of engine
// squares. It returns ErrGridLimits when that passes 200 x 400, and the zero
// Grid for no columns or no image.
func EngineGrid(drawnColumns, factor, width, height int) (Grid, error) {
	rows := RowsFor(drawnColumns, width, height)
	if rows == 0 {
		return Grid{}, nil
	}
	factor = max(factor, 1)
	g := Grid{Columns: drawnColumns * factor, Rows: rows * factor}
	if g.Columns > MaxColumns || g.Rows > MaxRows {
		return Grid{}, ErrGridLimits
	}
	return g, nil
}

// ScaleStep is the factor a change of calibration multiplies every painted
// square by, when that loses nothing: from factor from to factor to, with the
// same drawing, when to is a multiple of from. A square of the old grid becomes
// a block of step x step squares. ok is false for any other change (a smaller
// factor, or one that is not a multiple), which cannot be done without
// guessing, so the layers are cleared instead.
func ScaleStep(from, to int) (step int, ok bool) {
	if from < 1 || to < from || to%from != 0 {
		return 0, false
	}
	return to / from, true
}

// scaled returns the grid k times bigger in each direction.
func (g Grid) scaled(k int) Grid { return Grid{Columns: g.Columns * k, Rows: g.Rows * k} }

// Scaled is the layer on the grid k times bigger in each direction: every set
// square becomes a block of k x k set squares. k below 1 reads as 1.
func (l *Layer) Scaled(k int) *Layer {
	k = max(k, 1)
	if l == nil {
		return nil
	}
	out := NewLayer(l.g.scaled(k))
	for row := 0; row < l.g.Rows; row++ {
		for col := 0; col < l.g.Columns; col++ {
			if l.Get(col, row) {
				fill(col, row, k, func(c, r int) { out.Set(c, r, true) })
			}
		}
	}
	return out
}

// Scaled is the layer on the grid k times bigger: each painted square becomes
// a block of k x k squares of the same light.
func (l *LightLayer) Scaled(k int) *LightLayer {
	k = max(k, 1)
	if l == nil {
		return nil
	}
	out := NewLightLayer(l.c.g.scaled(k))
	for row := 0; row < l.c.g.Rows; row++ {
		for col := 0; col < l.c.g.Columns; col++ {
			if v := l.Get(col, row); v != Unpainted {
				fill(col, row, k, func(c, r int) { out.Set(c, r, v) })
			}
		}
	}
	return out
}

// Scaled is the layer on the grid k times bigger: each square of cover becomes
// a block of k x k squares of the same cover.
func (l *CoverLayer) Scaled(k int) *CoverLayer {
	k = max(k, 1)
	if l == nil {
		return nil
	}
	out := NewCoverLayer(l.c.g.scaled(k))
	for row := 0; row < l.c.g.Rows; row++ {
		for col := 0; col < l.c.g.Columns; col++ {
			if v := l.Get(col, row); v != CoverNone {
				fill(col, row, k, func(c, r int) { out.Set(c, r, v) })
			}
		}
	}
	return out
}

// Scaled is the layer on the grid k times bigger: each door becomes a block of
// k x k squares of the same state, because the door is the whole square of the
// drawing. A door in a wall line therefore becomes a doorway k squares wide,
// as wide as the drawing's own square was; walking through it opens (or stops
// at) the same door, and every other rule of the layer holds, since the layer
// never required floor on either side of a door.
func (l *DoorLayer) Scaled(k int) *DoorLayer {
	k = max(k, 1)
	if l == nil {
		return nil
	}
	out := NewDoorLayer(l.g.scaled(k))
	for row := 0; row < l.g.Rows; row++ {
		for col := 0; col < l.g.Columns; col++ {
			if v := l.Get(col, row); v != DoorNone {
				fill(col, row, k, func(c, r int) { out.Set(c, r, v) })
			}
		}
	}
	return out
}

// fill calls set for every square of the k x k block a square of the old grid
// becomes.
func fill(col, row, k int, set func(c, r int)) {
	for dr := 0; dr < k; dr++ {
		for dc := 0; dc < k; dc++ {
			set(col*k+dc, row*k+dr)
		}
	}
}
