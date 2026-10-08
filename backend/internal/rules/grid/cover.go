package grid

// CoverBetween is the cover the target has against an attack from another
// square (D4, Q70). The line between the two centers is walked; the best cover
// among the squares it enters counts, leaving out the attacker's own square
// and the target's: a wall on the line is total cover (and so is squeezing
// between two walls), a square of painted cover gives its degree, and a square
// holding another creature gives half. Degrees do not add: the most protective
// one applies (SRD). creatures are the squares of the other creatures, not the
// attacker or the target.
func (t Terrain) CoverBetween(attacker, target Square, creatures Set) Cover {
	best := CoverNone
	for _, s := range Line(attacker, target) {
		// The corner pass of the line enters neither side square; only a squeeze
		// between two walls stops it.
		if s.Corner && t.wall(s.SideA) && t.wall(s.SideB) {
			return CoverTotal
		}
		if s.Square == target {
			continue
		}
		if t.wall(s.Square) {
			return CoverTotal
		}
		best = max(best, t.Cover.Get(s.Square.Col, s.Square.Row))
		if inSet(creatures, s.Square) {
			best = max(best, CoverHalf)
		}
	}
	return best
}

// Reaction is what an opportunity attack needs to know about a move (SRD): a
// creature that leaves a hostile creature's reach lets it attack.
type Reaction struct {
	// Leaves is true when the mover was inside the reactor's reach at some point
	// of the move, the start included, and ends the move outside it.
	Leaves bool
	// LastInReach is the last square of the move inside the reach, where the
	// mover stays if the attack drops it to 0 hit points. Only meaningful when
	// Leaves is true.
	LastInReach Square
}

// LeavesReach says whether a straight move from one square to another leaves
// the reach of a creature standing on reactor, and the last square of the move
// inside it. The whole line is walked, the start included, so a move that
// passes through the reach (from one side of the reactor to the other) leaves
// it too. The reach is measured as attacks are, in squares between centers
// rounded down times 5 ft (RangeFt), so 5 ft covers all eight neighbors. A move
// that never touches the reach does not trigger, and one that ends inside it
// does not either.
func LeavesReach(from, to, reactor Square, reachFt int) Reaction {
	inside := func(s Square) bool { return RangeFt(reactor, s) <= reachFt }
	if inside(to) {
		return Reaction{}
	}
	// A square inside the reach is at most reachFt/FeetPerSquare columns and rows
	// from the reactor, and the line stays within the box of its two ends: a
	// reactor farther than that from the box is never touched, and the line
	// (the cost of a move's whole length) is not walked for it.
	if reach := reachFt / FeetPerSquare; reactor.Col < min(from.Col, to.Col)-reach || reactor.Col > max(from.Col, to.Col)+reach ||
		reactor.Row < min(from.Row, to.Row)-reach || reactor.Row > max(from.Row, to.Row)+reach {
		return Reaction{}
	}
	var last Square
	found := inside(from)
	if found {
		last = from
	}
	for _, s := range Line(from, to) {
		if inside(s.Square) {
			last, found = s.Square, true
		}
	}
	return Reaction{Leaves: found, LastInReach: last}
}
