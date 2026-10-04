package grid

import (
	"errors"
	"math"
	"slices"
)

// Set is a set of squares, such as the squares creatures stand on. A Layer is
// one; so is Squares.
type Set interface {
	Has(Square) bool
}

// Squares is a Set made of a short list of squares.
type Squares []Square

// Has says whether the square is in the list.
func (s Squares) Has(sq Square) bool { return slices.Contains(s, sq) }

func inSet(s Set, sq Square) bool { return s != nil && s.Has(sq) }

// Size is a creature's size, in the SRD's order. The zero value means "not
// said" and counts as Medium.
type Size uint8

// The sizes of the SRD, smallest to largest.
const (
	SizeTiny Size = iota + 1
	SizeSmall
	SizeMedium
	SizeLarge
	SizeHuge
	SizeGargantuan
)

// rank is the position of the size in the order, 1 (Tiny) to 6 (Gargantuan).
func (s Size) rank() int {
	if s < SizeTiny || s > SizeGargantuan {
		return int(SizeMedium)
	}
	return int(s)
}

// Occupant is who stands in a square, as far as moving through it goes: the
// largest creature there, and whether any of them is hostile to the mover.
type Occupant struct {
	Size    Size
	Hostile bool
}

// Occupants says who stands where. The caller builds it for one mover: it
// leaves the mover out, says who is hostile to it, and leaves out the
// creatures the mover does not see when it plans a move.
type Occupants interface {
	// At is the occupant of a square, and false for an empty one.
	At(Square) (Occupant, bool)
}

// OccupantMap is an Occupants made of a map.
type OccupantMap map[Square]Occupant

// At is the occupant of the square.
func (m OccupantMap) At(sq Square) (Occupant, bool) {
	o, ok := m[sq]
	return o, ok
}

func occupantAt(o Occupants, sq Square) (Occupant, bool) {
	if o == nil {
		return Occupant{}, false
	}
	return o.At(sq)
}

// Terrain is what a move runs over: the grid and its layers. Any layer may be
// nil (none painted). A square outside the grid is a wall.
type Terrain struct {
	Grid Grid
	// Walls block movement, sight and light.
	Walls *Layer
	// Difficult is the terrain that costs 5 ft more for each square entered.
	Difficult *Layer
	// Cover is the painted cover: three-quarters squares block movement
	// like a wall (but not sight or light), half-cover squares can be crossed.
	Cover *CoverLayer
}

// ErrBadGrid is returned for a grid a map cannot have, or a layer sized for
// another grid than the one it is used with: the bytes of a layer mean nothing
// on a different grid, so it is refused instead of being misread.
var ErrBadGrid = errors.New("grid: the grid is not valid, or a layer is sized for another grid")

// Validate checks that the grid is valid and that every layer present is sized
// for it. Call it once when the terrain is put together from stored layers;
// the methods below assume it passed.
func (t Terrain) Validate() error {
	if !t.Grid.Valid() {
		return ErrBadGrid
	}
	if (t.Walls != nil && t.Walls.Grid() != t.Grid) || (t.Difficult != nil && t.Difficult.Grid() != t.Grid) || (t.Cover != nil && t.Cover.Grid() != t.Grid) {
		return ErrBadGrid
	}
	return nil
}

// wall says whether a square is a wall: it blocks sight, light and movement.
func (t Terrain) wall(sq Square) bool {
	return !t.Grid.Contains(sq) || t.Walls.Has(sq)
}

// solid says whether a square blocks movement: a wall, or a square of
// three-quarters cover.
func (t Terrain) solid(sq Square) bool {
	return t.wall(sq) || t.Cover.Get(sq.Col, sq.Row) == CoverThreeQuarters
}

// Mover says how a creature moves. A flier ignores difficult terrain (walls
// stop everyone, and another creature's space still costs). Size decides
// whether it may pass a hostile creature and whether it may share a square.
type Mover struct {
	Flier bool
	Size  Size
}

// passes says whether the mover can move through an occupied square: always
// through a creature that is not hostile, and through a hostile one only when
// the two are at least two sizes apart, either way (SRD).
func (m Mover) passes(o Occupant) bool {
	d := m.Size.rank() - o.Size.rank()
	return !o.Hostile || d >= 2 || d <= -2
}

// canEnd says whether the mover may end its move in an occupied square: never,
// except a Tiny creature in a square that holds only Tiny creatures it can
// pass (SRD: four Tiny creatures fit in a square).
func (m Mover) canEnd(o Occupant) bool {
	return m.Size.rank() == int(SizeTiny) && o.Size.rank() == int(SizeTiny) && m.passes(o)
}

// lengthDFt is the distance between the centers of two squares in tenths of a
// foot, exact to the nearest tenth: 5 ft a square, so a diagonal neighbor is
// 70,7 tenths of ft... that is 7,07 ft, 71 here. The square root of an integer
// times 50 is never exactly half a tenth, so the rounding has no ties.
func lengthDFt(from, to Square) int {
	dc, dr := to.Col-from.Col, to.Row-from.Row
	return int(math.Round(math.Sqrt(float64(dc*dc+dr*dr)) * DFtPerSquare))
}

// blocked says whether a step cannot be entered: its square blocks movement,
// or the line squeezes between two squares that do and touch at a corner.
func (t Terrain) blocked(s Step) bool {
	return t.solid(s.Square) || (s.Corner && t.solid(s.SideA) && t.solid(s.SideB))
}

// Move is the result of one straight move.
type Move struct {
	// Blocked is true when the move cannot be made. Nothing else is
	// meaningful then.
	Blocked bool
	// By says what blocked it: StopWall for a wall, a square of
	// three-quarters cover or a squeeze, StopOccupied for a creature in the
	// way.
	By StopReason
	// CostDFt is the cost in tenths of a foot: the length of the line plus 50
	// for each square of difficult terrain or holding another creature it
	// enters, the destination included.
	CostDFt int
	// Entered are the squares the line enters, in order, the destination last.
	Entered []Square
}

// Move is the cost of a straight move from one square to another (D1): the
// distance between the centers, exact to the nearest tenth of a foot, plus
// 5 ft for each square the line enters that is difficult terrain or holds a
// creature (the SRD counts another creature's space as difficult terrain,
// hostile or not). A flier pays no extra for difficult terrain.
//
// The move is blocked by a wall, a square of three-quarters cover, a squeeze
// between two of them, a hostile creature the mover cannot pass (it can only
// when the two are two sizes apart), and by ending in a square another
// creature holds (a Tiny creature may share a square with another Tiny one).
func (t Terrain) Move(from, to Square, occ Occupants, m Mover) Move {
	return t.walk(from, to, occ, m, false)
}

// Jump is the cost of a long jump between two squares: the length of the line,
// with no extra for difficult terrain or creatures on the way (the jumper
// clears them). It cannot cross a wall or a square of three-quarters cover,
// and it cannot end in a square another creature holds.
func (t Terrain) Jump(from, to Square, occ Occupants, m Mover) Move {
	return t.walk(from, to, occ, m, true)
}

func (t Terrain) walk(from, to Square, occ Occupants, m Mover, clears bool) Move {
	if !t.Grid.Valid() || !t.Grid.Contains(from) || !t.Grid.Contains(to) {
		return Move{Blocked: true, By: StopWall}
	}
	line := Line(from, to)
	res := Move{CostDFt: lengthDFt(from, to), Entered: make([]Square, 0, len(line))}
	for _, s := range line {
		if t.blocked(s) {
			return Move{Blocked: true, By: StopWall}
		}
		res.Entered = append(res.Entered, s.Square)
		o, occupied := occupantAt(occ, s.Square)
		// Difficult terrain costs 5 ft once, even if several things in the square
		// count as it (SRD): rubble and a creature standing on it.
		if !clears && ((!m.Flier && t.Difficult.Has(s.Square)) || occupied) {
			res.CostDFt += DFtPerSquare
		}
		if occupied {
			// A jump clears the creatures on the way, and only the landing
			// square matters; a walk has to get through each one.
			switch {
			case s.Square == to && !m.canEnd(o), s.Square != to && !clears && !m.passes(o):
				return Move{Blocked: true, By: StopOccupied}
			}
		}
	}
	return res
}

// StopReason says why a move was blocked or ended early.
type StopReason string

// The reasons a move ends early.
const (
	// StopNone: the mover reached the destination.
	StopNone StopReason = ""
	// StopWall: the next square is a wall, a square of three-quarters cover or
	// a squeeze between two of them.
	StopWall StopReason = "wall"
	// StopMarked: the mover entered a square of the stop set (a trap's area)
	// and stops there.
	StopMarked StopReason = "marked"
	// StopOccupied: a creature is in the way and cannot be passed, or the mover
	// cannot end in the square one holds, so it stops before.
	StopOccupied StopReason = "occupied"
	// StopMovement: the next square would cost more than the movement left. The
	// player planned the move on what the character sees, so the server stops
	// the mover here instead of refusing, which would tell what is hidden.
	StopMovement StopReason = "movement"
)

// Stopped is where a move really ended.
type Stopped struct {
	// Reached is the last square the mover really got to and ends in: the
	// start when it could not take a single step.
	Reached Square
	// CostDFt is what the move really cost up to Reached, in tenths of a foot,
	// and never more than the movement left.
	CostDFt int
	// Reason is why it ended before the destination, or StopNone.
	Reason StopReason
	// Entered are the squares it really entered up to Reached, Reached last.
	Entered []Square
	// Marked says the mover entered a square of the stop set (a trap's area)
	// and MarkedAt is the first one: the trap fires there, even when the mover
	// cannot end its move in that square (a creature stands in it) and so
	// Reached is an earlier square.
	Marked   bool
	MarkedAt Square
}

// MoveUntil is the move as it really happens when the mover may not know all
// that is on the way (the fog of war, D1, and a trap's area, D5). The player
// plans the move with what the character sees (Move on what it sees), and the
// server runs the plan here with what is really there: the move follows the
// line and ends
//
//   - before the first wall, square of three-quarters cover or squeeze;
//   - before a hostile creature it cannot pass;
//   - before the first square it cannot afford with the movement left (left,
//     in tenths of a foot; pass math.MaxInt for no limit, as for the master),
//     because hidden rubble may have made the real move dearer than the plan;
//   - at the first square of stopAt, which it enters (the trap fires there);
//   - on the last square it may end in, when the destination (or a stopAt
//     square) holds a creature it may not share the square with. Marked and
//     MarkedAt still say the trap square was entered, so the trap fires.
//
// The cost is from the start to the center of Reached plus 5 ft for each
// square of difficult terrain (not for a flier) or holding a creature it
// entered, once per square. It never goes down along the line, so what the
// mover can afford is a stretch from the start. A start or a destination
// outside the grid gives a move that never leaves the start.
func (t Terrain) MoveUntil(from, to Square, occ Occupants, m Mover, stopAt Set, left int) Stopped {
	res := Stopped{Reached: from}
	if !t.Grid.Valid() || !t.Grid.Contains(from) || !t.Grid.Contains(to) {
		res.Reason = StopWall
		return res
	}
	line := Line(from, to)
	entered := make([]Square, 0, len(line))
	costs := make([]int, 0, len(line)) // the cost of the move up to each entered square
	reason := StopNone
	extras, floor := 0, 0
walk:
	for _, s := range line {
		o, occupied := occupantAt(occ, s.Square)
		switch {
		case t.blocked(s):
			reason = StopWall
			break walk
		case occupied && !m.passes(o):
			reason = StopOccupied
			break walk
		}
		if (!m.Flier && t.Difficult.Has(s.Square)) || occupied {
			extras += DFtPerSquare
		}
		cost := max(floor, lengthDFt(from, s.Square)+extras)
		if cost > left {
			reason = StopMovement
			break walk
		}
		floor = cost
		entered = append(entered, s.Square)
		costs = append(costs, cost)
		if !res.Marked && inSet(stopAt, s.Square) {
			res.Marked, res.MarkedAt = true, s.Square
			if s.Square != to {
				reason = StopMarked
			}
			break
		}
	}
	// The mover never ends in a square it may not share: back up to the last
	// square it may end in.
	for len(entered) > 0 {
		o, ok := occupantAt(occ, entered[len(entered)-1])
		if !ok || m.canEnd(o) {
			break
		}
		entered, costs = entered[:len(entered)-1], costs[:len(costs)-1]
		if !res.Marked {
			reason = StopOccupied
		}
	}
	if res.Marked && reason == StopNone {
		reason = StopMarked // the destination is in the area too
	}
	if len(entered) > 0 {
		res.Reached = entered[len(entered)-1]
		res.CostDFt = costs[len(costs)-1]
	}
	res.Reason = reason
	res.Entered = entered
	return res
}

// Reachable is a square a mover can get to in one straight move, and what the
// move costs.
type Reachable struct {
	Square  Square
	CostDFt int
}

// Reach is every square the mover can get to from one square with movement
// left tenths of a foot, in one straight move (D1): a circle, shrunk by
// difficult terrain and creatures on the way, and cut by walls, columns and
// hostile creatures. The start is not in the list, and neither is a square
// the mover may not end in. The list is in reading order (row by row), so it
// is the same every time.
func (t Terrain) Reach(from Square, left int, occ Occupants, m Mover) []Reachable {
	if left <= 0 || !t.Grid.Valid() || !t.Grid.Contains(from) {
		return nil
	}
	// The cost is never below the length, so no square further than the
	// movement left is worth a line.
	radius := left/DFtPerSquare + 1
	var out []Reachable
	for row := max(from.Row-radius, 0); row <= min(from.Row+radius, t.Grid.Rows-1); row++ {
		for col := max(from.Col-radius, 0); col <= min(from.Col+radius, t.Grid.Columns-1); col++ {
			sq := Square{col, row}
			if sq == from || lengthDFt(from, sq) > left {
				continue
			}
			if mv := t.Move(from, sq, occ, m); !mv.Blocked && mv.CostDFt <= left {
				out = append(out, Reachable{Square: sq, CostDFt: mv.CostDFt})
			}
		}
	}
	return out
}

// RangeSquares is the distance between two squares for an attack or a spell:
// the straight distance between the centers, counted in squares and rounded
// down. A diagonal neighbor is 1 square away, so a reach of 5 ft works on all
// eight sides; 4 squares diagonally is 5 squares. Only movement uses the exact
// line.
func RangeSquares(from, to Square) int {
	dc, dr := to.Col-from.Col, to.Row-from.Row
	n := dc*dc + dr*dr
	r := int(math.Sqrt(float64(n)))
	// Sqrt of a float is exact for the sizes of a grid, but settle the
	// boundary with integers anyway.
	for r*r > n {
		r--
	}
	for (r+1)*(r+1) <= n {
		r++
	}
	return r
}

// RangeFt is RangeSquares in feet: what an attack's reach or a spell's range
// is compared with.
func RangeFt(from, to Square) int {
	return RangeSquares(from, to) * FeetPerSquare
}
