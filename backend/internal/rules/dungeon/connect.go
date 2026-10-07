package dungeon

import "slices"

// components labels the connected components of open squares (4-neighbor
// adjacency; a door is a normal open square, spec 3.6.1). Component c is
// order[starts[c]:starts[c+1]]; comp is -1 on rock.
type components struct {
	comp   []int32
	order  []int32
	starts []int // one more than the number of components
}

func (c *components) squares(id int32) []int32 { return c.order[c.starts[id]:c.starts[id+1]] }

func (g *gen) labelComponents() *components {
	n, openSq := g.w*g.h, 0
	for _, k := range g.kind {
		if k != KindRock {
			openSq++
		}
	}
	c := &components{comp: make([]int32, n), order: make([]int32, 0, openSq)}
	for i := range c.comp {
		c.comp[i] = -1
	}
	for i := range n {
		if g.kind[i] == KindRock || c.comp[i] >= 0 {
			continue
		}
		id := int32(len(c.starts)) //nolint:gosec // G115: at most one component per square
		c.starts = append(c.starts, len(c.order))
		c.comp[i] = id
		c.order = append(c.order, int32(i))
		for head := c.starts[id]; head < len(c.order); head++ {
			u := int(c.order[head])
			x, y := u%g.w, u/g.w
			for d := range 4 {
				nx, ny := x+dx[d], y+dy[d]
				if !g.inside(nx, ny) {
					continue
				}
				v := g.idx(nx, ny)
				if g.kind[v] != KindRock && c.comp[v] < 0 {
					c.comp[v] = id
					c.order = append(c.order, int32(v)) //nolint:gosec // G115: a square index of a grid under 80 000
				}
			}
		}
	}
	c.starts = append(c.starts, len(c.order))
	return c
}

// link is a candidate door that would join an orphan group to the rest.
type link struct {
	room *room
	side Side
	t    threshold
	// fromMain: a door on a main-component room into the group (3.6.3 (b)).
	fromMain bool
}

// connect is spec 3.6: drop sealed corridors, link every orphan room group
// to the main component (or discard it), then the optional extra loops.
func (g *gen) connect() {
	r := stream(g.o.Seed, phaseConnect)
	c := g.labelComponents()
	ncomp := len(c.starts) - 1
	main := c.comp[g.idx(g.rooms[0].centerX, g.rooms[0].centerY)]

	roomsOf := make([][]*room, ncomp)
	for _, rm := range g.rooms {
		id := c.comp[g.idx(rm.centerX, rm.centerY)]
		roomsOf[id] = append(roomsOf[id], rm)
	}
	// 3.6.2: a component with no room is a sealed corridor tree: erase it.
	for id := range int32(ncomp) { //nolint:gosec // G115: at most one component per square
		if len(roomsOf[id]) == 0 {
			for _, sq := range c.squares(id) {
				g.kind[sq] = KindRock
				g.lane[sq] = false
			}
		}
	}
	// 3.6.3: the groups in order of their lowest room ID. A group that cannot
	// link yet is deferred to the back of the queue, unchanged; when a full
	// pass links nothing, only the front group is discarded and another pass
	// runs. Each attempt gathers its candidates again (the main component has
	// grown) and shuffles them: one DRAW per attempt.
	var cands []link
	var queue []int32
	seen := make([]bool, ncomp)
	for _, rm := range g.rooms {
		id := c.comp[g.idx(rm.centerX, rm.centerY)]
		if id != main && !seen[id] {
			seen[id] = true
			queue = append(queue, id)
		}
	}
	mainRooms := roomsOf[main]
	discards := 0
	for pass := 0; len(queue) > 0; pass++ {
		var deferred []int32
		for _, id := range queue {
			cands = g.gatherLinks(cands[:0], roomsOf[id], mainRooms, c, main, id)
			shuffle(r, cands) // DRAW: the candidate list, once per attempt
			linked := false
			for _, cd := range cands {
				if g.tryLink(cd, c, main) {
					linked = true
					break
				}
			}
			if !linked {
				deferred = append(deferred, id)
				continue
			}
			if pass > 0 && g.onLater != nil {
				g.onLater(discards)
			}
			for _, sq := range c.squares(id) {
				c.comp[sq] = main
			}
			mainRooms = append(mainRooms, roomsOf[id]...)
			slices.SortFunc(mainRooms, func(a, b *room) int { return a.id - b.id })
		}
		if len(deferred) == len(queue) {
			// A fruitless pass: discard only the front group (lowest room ID);
			// its erased squares may open paths for the others.
			id := deferred[0]
			g.discardGroup(roomsOf[id], c.squares(id))
			discards++
			deferred = deferred[1:]
		}
		queue = deferred
		g.passes = pass + 1
	}
	g.compact()
	if g.o.ExtraLoops > 0 {
		g.extraLoops(r)
	}
}

// gatherLinks lists the candidate links of a group (spec 3.6.3 step 1), in the
// fixed order: first every (a), then every (b); within each, by room ID, side
// (north, east, south, west) and position. (a) are the thresholds of the
// group's rooms whose far side is a main-component open square or a free
// node; (b) are the thresholds of the main component's rooms whose far side is
// an open square of the group.
func (g *gen) gatherLinks(out []link, rooms, mainRooms []*room, c *components, main, group int32) []link {
	for _, rm := range rooms {
		for side := North; side <= West; side++ {
			for p := 0; p < rm.sideLen(side); p++ {
				t, ok := g.candidateOK(rm, side, p)
				if !ok {
					continue
				}
				bi := g.idx(t.beyondX, t.beyondY)
				if g.kind[bi] == KindRock {
					if !g.freeNode(t.beyondX, t.beyondY) {
						continue
					}
				} else if c.comp[bi] != main {
					continue
				}
				out = append(out, link{room: rm, side: side, t: t})
			}
		}
	}
	for _, rm := range mainRooms {
		for side := North; side <= West; side++ {
			for p := 0; p < rm.sideLen(side); p++ {
				t, ok := g.candidateOK(rm, side, p)
				if !ok {
					continue
				}
				bi := g.idx(t.beyondX, t.beyondY)
				if g.kind[bi] != KindRock && c.comp[bi] == group {
					out = append(out, link{room: rm, side: side, t: t, fromMain: true})
				}
			}
		}
	}
	return out
}

// tryLink opens one candidate if it works: directly onto main-component
// floor, or through the shortest path of free nodes (breadth-first, steps of
// 2, directions north, east, south, west, so ties are deterministic).
func (g *gen) tryLink(l link, c *components, main int32) bool {
	bi := g.idx(l.t.beyondX, l.t.beyondY)
	if g.kind[bi] != KindRock {
		g.epoch++ // the group joins the main component
		g.drawKind(g.openDoor(l.room, l.side, l.t, false))
		return true
	}
	path, ok := g.shortestPath(bi, c, main)
	if !ok {
		return false
	}
	g.epoch++
	for _, sq := range path {
		g.kind[sq] = KindCorridor
		c.comp[sq] = main
	}
	g.drawKind(g.openDoor(l.room, l.side, l.t, false))
	return true
}

// shortestPath finds the squares to carve (nodes and the wall-line squares
// between them, from the start node on) to reach a main-component corridor.
func (g *gen) shortestPath(start int, c *components, main int32) ([]int, bool) {
	n := g.w * g.h
	if g.stamp == nil {
		g.stamp = make([]int32, n)
		g.parent = make([]int32, n)
		g.dead = make([]int32, n)
	}
	if g.dead[start] == g.epoch+1 {
		return nil, false
	}
	g.curSt++
	cur := g.curSt
	g.queue = append(g.queue[:0], int32(start)) //nolint:gosec // G115: a square index of a grid under 80 000
	g.stamp[start] = cur
	g.parent[start] = -1
	for head := 0; head < len(g.queue); head++ {
		u := int(g.queue[head])
		x, y := u%g.w, u/g.w
		for d := range 4 {
			mi := g.idx(x+dx[d], y+dy[d])
			if !g.inside(x+2*dx[d], y+2*dy[d]) || g.kind[mi] != KindRock || g.blocked[mi] {
				continue
			}
			vx, vy := x+2*dx[d], y+2*dy[d]
			v := g.idx(vx, vy)
			if g.kind[v] == KindCorridor && c.comp[v] == main {
				path := []int{mi}
				for q := u; q >= 0; q = int(g.parent[q]) {
					path = append(path, q)
					if p := int(g.parent[q]); p >= 0 {
						path = append(path, (p+q)/2)
					}
				}
				return path, true
			}
			if g.freeNode(vx, vy) && g.stamp[v] != cur && g.dead[v] != g.epoch+1 {
				g.stamp[v] = cur
				g.parent[v] = int32(u)              //nolint:gosec // G115: a square index of a grid under 80 000
				g.queue = append(g.queue, int32(v)) //nolint:gosec // G115: a square index of a grid under 80 000
			}
		}
	}
	// Nothing in this free region touches the main component: remember it, until
	// the main component grows. (dead holds epoch+1 so that 0 means "unknown".)
	for _, v := range g.queue {
		g.dead[v] = g.epoch + 1
	}
	return nil, false
}

// discardGroup erases a group that could not be linked: its rooms' floors,
// rings and doors (and the corridors of its component) become rock; the room
// IDs stay unused.
func (g *gen) discardGroup(rooms []*room, squares []int32) {
	g.epoch++ // erased squares are free nodes again
	for _, sq := range squares {
		g.kind[sq] = KindRock
		g.rid[sq] = 0
		g.lane[sq] = false
		g.doorKind[sq] = DoorArchway
		g.doorTrap[sq] = false
	}
	for _, rm := range rooms {
		rm.discarded = true
		g.roomByID[rm.id] = nil
		g.discarded = append(g.discarded, rm.id)
	}
}

// extraLoops is spec 3.6.4: open some of the wall-line squares that separate
// two corridors, or a corridor and a room's threshold, to give the level
// cycles. The candidates are listed in a fixed order (corridor pairs in
// row-major order, then room thresholds by room, side and position), shuffled
// once, and each is opened with probability extra_loops percent if it still
// is valid when its turn comes (a door keeps its 4-square spacing).
func (g *gen) extraLoops(r *rng) {
	type cand struct {
		sq   int // corridor-corridor: the square to carve; room threshold: its door square
		room *room
		side Side
		pos  int
	}
	var cs []cand
	for y := 1; y < g.h-1; y++ {
		for x := 1; x < g.w-1; x++ {
			i := g.idx(x, y)
			if g.kind[i] != KindRock || g.blocked[i] {
				continue
			}
			// a midpoint: one coordinate even, the other odd
			var a, b int
			switch {
			case x%2 == 0 && y%2 == 1:
				a, b = g.idx(x-1, y), g.idx(x+1, y)
			case x%2 == 1 && y%2 == 0:
				a, b = g.idx(x, y-1), g.idx(x, y+1)
			default:
				continue
			}
			if g.kind[a] == KindCorridor && g.kind[b] == KindCorridor {
				cs = append(cs, cand{sq: i})
			}
		}
	}
	for _, rm := range g.rooms {
		for side := North; side <= West; side++ {
			for p := 0; p < rm.sideLen(side); p++ {
				if t, ok := g.candidateOK(rm, side, p); ok && g.kind[g.idx(t.beyondX, t.beyondY)] == KindCorridor {
					cs = append(cs, cand{sq: g.idx(t.doorX, t.doorY), room: rm, side: side, pos: p})
				}
			}
		}
	}
	// row-major order of the square (stable: a shared wall's two thresholds keep room order)
	slices.SortStableFunc(cs, func(a, b cand) int { return a.sq - b.sq })
	shuffle(r, cs) // DRAW: the loop candidates, once
	for _, cd := range cs {
		if cd.room == nil {
			if g.kind[cd.sq] != KindRock {
				continue
			}
		} else if _, ok := g.candidateOK(cd.room, cd.side, cd.pos); !ok {
			continue
		}
		if !r.chance(g.o.ExtraLoops) { // DRAW: open this one (none at 100)
			continue
		}
		if cd.room == nil {
			g.kind[cd.sq] = KindCorridor
		} else {
			t, _ := g.candidateOK(cd.room, cd.side, cd.pos)
			g.drawKind(g.openDoor(cd.room, cd.side, t, false))
		}
	}
}
