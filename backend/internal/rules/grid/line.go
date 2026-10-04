package grid

// The straight line between the centers of two squares (RN-21). It is walked
// exactly, with integers, so the same two squares always give the same answer
// on every machine:
//
//   - the start square is not part of the line; the end square is;
//   - a line that passes exactly through the corner shared by four squares
//     goes from one square to the diagonally opposite one and enters neither of
//     the other two (a "corner" step);
//   - a corner step between two squares that both block (two walls that touch
//     at a corner) is a squeeze, and it is blocked: the line does not slip
//     between them.

// Step is one square the line enters.
type Step struct {
	Square Square
	// Corner is true when the line entered the square exactly through a
	// corner, from the square diagonally opposite. SideA and SideB are then the
	// two squares that touch the same corner and that the line did not enter.
	Corner       bool
	SideA, SideB Square
}

// walker walks the squares of a line. Everything is in squares counted from
// the center of the start: the line crosses the (i+1)-th vertical square edge
// at the fraction (2i+1)/(2·nc) of its length and the (j+1)-th horizontal edge
// at (2j+1)/(2·nr), so comparing the two fractions is a comparison of two
// integer products.
type walker struct {
	col, row int // the current square
	sx, sy   int // the direction, -1 or 1
	nc, nr   int // the squares to cross, across and down
	i, j     int // the edges crossed so far
}

func newWalker(from, to Square) walker {
	w := walker{col: from.Col, row: from.Row, nc: to.Col - from.Col, nr: to.Row - from.Row, sx: 1, sy: 1}
	if w.nc < 0 {
		w.nc, w.sx = -w.nc, -1
	}
	if w.nr < 0 {
		w.nr, w.sy = -w.nr, -1
	}
	return w
}

func (w *walker) done() bool { return w.i == w.nc && w.j == w.nr }

// sides are the two squares beside the corner the next step would cross, if
// it is a corner step. Read them before step, which moves the walker.
func (w *walker) sides() (a, b Square) {
	return Square{w.col + w.sx, w.row}, Square{w.col, w.row + w.sy}
}

// step moves to the next square of the line. It returns true when the move
// crossed exactly through a corner (the walker moved diagonally).
func (w *walker) step() (corner bool) {
	switch {
	case w.i == w.nc:
		w.row += w.sy
		w.j++
	case w.j == w.nr:
		w.col += w.sx
		w.i++
	default:
		across, down := (2*w.i+1)*w.nr, (2*w.j+1)*w.nc
		switch {
		case across < down:
			w.col += w.sx
			w.i++
		case across > down:
			w.row += w.sy
			w.j++
		default:
			w.col += w.sx
			w.row += w.sy
			w.i++
			w.j++
			return true
		}
	}
	return false
}

// Line is the squares the straight line from the center of one square to the
// center of another enters, in order: the start is left out and the end is
// the last one. The same square gives an empty line.
func Line(from, to Square) []Step {
	w := newWalker(from, to)
	steps := make([]Step, 0, w.nc+w.nr)
	for !w.done() {
		a, b := w.sides()
		corner := w.step()
		s := Step{Square: Square{w.col, w.row}}
		if corner {
			s.Corner, s.SideA, s.SideB = true, a, b
		}
		steps = append(steps, s)
	}
	return steps
}

// Sight answers "is there a wall between these two squares?" for a fixed set
// of walls, quickly: the fog of war asks it for every square a viewer might
// see. Squares outside the grid count as walls. Build one with NewSight; it
// copies the walls, so changing the Layer later does not change it.
type Sight struct {
	g     Grid
	width int
	solid []bool // padded by one square on every side, true = wall
	// walls[r*(cols+1)+c] is how many walls the squares before column c and row
	// r hold (a summed-area table): a line whose bounding box holds none is
	// clear without walking it, which is most of them on a map with few walls.
	walls []int32
}

// NewSight prepares the walls of a grid (nil for none) for Clear. It returns
// ErrBadGrid for an invalid grid or walls sized for another one.
func NewSight(g Grid, walls *Layer) (*Sight, error) {
	if !g.Valid() || (walls != nil && walls.Grid() != g) {
		return nil, ErrBadGrid
	}
	s := &Sight{g: g, width: g.Columns + 2, solid: make([]bool, (g.Columns+2)*(g.Rows+2))}
	for i := range s.solid {
		col, row := i%s.width-1, i/s.width-1
		s.solid[i] = !g.Contains(Square{col, row}) || walls.Get(col, row)
	}
	stride := g.Columns + 1
	s.walls = make([]int32, stride*(g.Rows+1))
	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Columns; col++ {
			n := s.walls[row*stride+col+1] + s.walls[(row+1)*stride+col] - s.walls[row*stride+col]
			if walls.Get(col, row) {
				n++
			}
			s.walls[(row+1)*stride+col+1] = n
		}
	}
	return s, nil
}

// wallsIn counts the walls in the rectangle of squares between two squares,
// both included.
func (s *Sight) wallsIn(a, b Square) int32 {
	c0, c1 := min(a.Col, b.Col), max(a.Col, b.Col)+1
	r0, r1 := min(a.Row, b.Row), max(a.Row, b.Row)+1
	stride := s.g.Columns + 1
	return s.walls[r1*stride+c1] - s.walls[r0*stride+c1] - s.walls[r1*stride+c0] + s.walls[r0*stride+c0]
}

// Wall says whether a square is a wall (or outside the grid).
func (s *Sight) Wall(sq Square) bool {
	return s.solid[(sq.Row+1)*s.width+sq.Col+1]
}

// skipChunk is the stretch, in squares of a line's longer side, below which
// Clear stops splitting and walks.
const skipChunk = 16

// progress is how many edges of the dominant axis the walker has crossed, and
// major how many there are in all.
func (w *walker) progress() int {
	if w.nc >= w.nr {
		return w.i
	}
	return w.j
}

func (w *walker) major() int { return max(w.nc, w.nr) }

// at is the walker's current square.
func (w *walker) at() Square { return Square{w.col, w.row} }

// jumpTo is the walker after it has crossed m edges of the dominant axis, along
// with every edge of the other axis that comes before the m-th (and one that
// comes exactly with it, a corner), which is where step would have left it. It
// is called on the walker as it was at the start of the line, and m must be
// between 1 and major.
func (w walker) jumpTo(m int) walker {
	if w.nc >= w.nr {
		w.i, w.j = m, ((2*m-1)*w.nr+w.nc)/(2*w.nc)
	} else {
		w.j, w.i = m, ((2*m-1)*w.nc+w.nr)/(2*w.nr)
	}
	w.col += w.sx * w.i
	w.row += w.sy * w.j
	return w
}

// Clear says whether the line between the two squares crosses no wall. The
// start and the end squares themselves are not looked at (a wall can be seen,
// and a light can stand on one), but squeezing between two walls that touch at
// a corner is blocked, even on the last step. Both squares must be inside the
// grid.
//
// A long line mostly runs through open squares, so Clear does not step on every
// one: it asks the summed-area table whether a stretch of the line has any wall
// in its bounding box (which holds every square and corner the stretch can
// touch) and jumps the whole stretch if not. A stretch with a wall in its box
// is split in two and each half asked again, down to skipChunk squares of the
// line's longer side, which are walked one at a time.
func (s *Sight) Clear(from, to Square) bool {
	if s.wallsIn(from, to) == 0 {
		return true
	}
	origin := newWalker(from, to)
	w := origin
	// The last square is the end, which is never looked at: advance to just
	// before it, then take the last step.
	if !s.span(&origin, &w, origin.major()-1) {
		return false
	}
	for !w.done() {
		if !s.stepClear(&w) {
			return false
		}
	}
	return true
}

// span moves w along the line, which began at origin, until it has crossed end
// edges of the longer side, and says whether that stretch was clear.
func (s *Sight) span(origin, w *walker, end int) bool {
	if w.progress() >= end {
		return true
	}
	if next := origin.jumpTo(end); s.wallsIn(w.at(), next.at()) == 0 {
		*w = next
		return true
	}
	if end-w.progress() <= skipChunk {
		for w.progress() < end {
			if !s.stepClear(w) {
				return false
			}
		}
		return true
	}
	return s.span(origin, w, (w.progress()+end)/2) && s.span(origin, w, end)
}

// stepClear takes one step of the line and says whether it was clear: no
// squeeze, and, unless it is the last square, no wall.
func (s *Sight) stepClear(w *walker) bool {
	a, b := w.sides()
	corner := w.step()
	if corner && s.solid[(a.Row+1)*s.width+a.Col+1] && s.solid[(b.Row+1)*s.width+b.Col+1] {
		return false
	}
	return w.done() || !s.solid[(w.row+1)*s.width+w.col+1]
}
