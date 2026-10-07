package dungeon

// keepChance is the chance that a walk first tries to keep its direction
// (spec 2).
func (g *gen) keepChance() int {
	switch g.o.CorridorStyle {
	case StyleTwisty:
		return 0
	case StyleLongRuns:
		return 85
	}
	return 40
}

// walkFrame is one node of the maze walk's explicit stack (no recursion: a
// 199 by 399 grid has about 20 000 nodes).
type walkFrame struct {
	x, y  int
	order [4]uint8
	next  uint8
}

// frameAt builds the frame of a node a walk has just reached: the direction
// order is drawn here and kept until the walk backs up to the node (spec 3.5,
// 4.5). prev is the arrival direction, or -1 at the start of a walk. forced
// is true for the first node of a lane walk, whose preferred direction (away
// from the door) is not a chance.
func (g *gen) frameAt(r *rng, x, y, prev int, forced bool) walkFrame {
	f := walkFrame{x: x, y: y, order: [4]uint8{0, 1, 2, 3}}
	keep := false
	if prev >= 0 {
		keep = forced || r.chance(g.keepChance()) // DRAW: keep direction (none at 0 or 100, none for a lane start)
	}
	shuffle(r, f.order[:]) // DRAW: the four directions, Fisher-Yates
	if keep {
		// The arrival direction goes to the front, the others keep their order.
		pos := 0
		for k, d := range f.order {
			if int(d) == prev {
				pos = k
			}
		}
		copy(f.order[1:pos+1], f.order[:pos])
		f.order[0] = uint8(prev) //nolint:gosec // G115: a direction, 0 to 3
	}
	return f
}

// canStep: a walk may step from a node to the node 2 squares away in
// direction d when the wall-line square between them is rock and not blocked
// and the destination is a free usable node, or a door lane that no walk has
// joined yet (the exception of spec 3.5: without it the first walk fills its
// region and every other lane in it stays a one-square stub). A room ring can
// never be the wall-line square: a ring square between two nodes always has
// a floor node on one side, so the destination test already refuses it.
func (g *gen) canStep(x, y, d int) bool {
	vx, vy := x+2*dx[d], y+2*dy[d]
	mi := g.idx(x+dx[d], y+dy[d])
	if g.kind[mi] != KindRock || g.blocked[mi] {
		return false
	}
	if g.freeNode(vx, vy) {
		return true
	}
	vi := g.idx(vx, vy)
	return g.inside(vx, vy) && g.lane[vi] && !g.joined[vi]
}

// walk runs one randomized depth-first walk from a node that is already
// carved.
func (g *gen) walk(r *rng, stack []walkFrame, start walkFrame) []walkFrame {
	stack = append(stack[:0], start)
	for len(stack) > 0 {
		top := len(stack) - 1
		advanced := false
		for stack[top].next < 4 {
			d := int(stack[top].order[stack[top].next])
			stack[top].next++
			x, y := stack[top].x, stack[top].y
			if !g.canStep(x, y, d) {
				continue
			}
			g.kind[g.idx(x+dx[d], y+dy[d])] = KindCorridor
			vx, vy := x+2*dx[d], y+2*dy[d]
			g.kind[g.idx(vx, vy)] = KindCorridor
			g.joined[g.idx(vx, vy)] = true
			stack = append(stack, g.frameAt(r, vx, vy, d, false))
			advanced = true
			break
		}
		if !advanced {
			stack = stack[:top]
		}
	}
	return stack
}

// carveCorridors is spec 3.5: the maze fills every free lattice node. The
// squares beyond doors were carved in 3.4 and start walks first, in lane
// order; then the row-major scan starts a walk at every free node.
func (g *gen) carveCorridors() {
	r := stream(g.o.Seed, phaseCorridor)
	var stack []walkFrame
	for _, l := range g.lanes {
		if g.joined[l.idx] {
			continue // a walk already stepped onto this lane and exhausted it
		}
		g.joined[l.idx] = true
		x, y := l.idx%g.w, l.idx/g.w
		stack = g.walk(r, stack, g.frameAt(r, x, y, l.away, true))
	}
	for j := range g.ny {
		for i := range g.nx {
			x, y := 2*i+1, 2*j+1
			if !g.freeNode(x, y) {
				continue
			}
			g.kind[g.idx(x, y)] = KindCorridor
			stack = g.walk(r, stack, g.frameAt(r, x, y, -1, false))
		}
	}
}
