package dungeon

import (
	"fmt"
	"slices"
)

// dx and dy are the four directions in the fixed order north, east, south,
// west, the order of Side.
var (
	dx = [4]int{0, 1, 0, -1}
	dy = [4]int{-1, 0, 1, 0}
)

// roomCap is the most rooms one level holds (spec 3.3, section 6).
const roomCap = 500

type room struct {
	id               int
	x, y, w, h       int
	discarded        bool
	centerX, centerY int
}

// gen is the working state of one generation. It is private: a Dungeon is
// the result.
type gen struct {
	o      Options
	aspect int // AspectLimit in hundredths
	w, h   int
	nx, ny int // lattice nodes across and down

	kind    []Kind
	rid     []uint16
	blocked []bool
	// lane marks the first square beyond a door when it is a corridor.
	lane []bool
	// protected marks the squares dead-end removal must keep (stairs and the
	// square behind each).
	protected []bool
	// joined marks the lanes a walk has started from or stepped onto (3.5).
	joined []bool
	// kindRNG is the "door kinds" stream: every kind and trapped draw.
	kindRNG  *rng
	doorKind []DoorKind
	doorTrap []bool

	rooms    []*room // in ID order, discards removed
	roomByID []*room // indexed by ID; nil for a discarded or unused ID
	// directPairs is the set of room pairs that already share a direct door
	// (keyed by the pair, never scanned: spec 6).
	directPairs map[uint32]struct{}
	// lanes are the squares beyond doors that start the maze walks, in order.
	lanes []laneStart

	discarded []int
	stairs    []Stair

	// scratch for the breadth-first searches
	stamp  []int32
	parent []int32
	dead   []int32 // epoch at which a free node was found unable to reach the main component
	queue  []int32
	curSt  int32
	epoch  int32 // bumped whenever the main component grows

	anyBlocked bool

	hook func(stage string, g *gen)
	// onDiscard is a test probe: called for each discarded group.
	// Test probes: onLater is called when a deferred group links on a later
	// pass (with the number of groups discarded so far); passes is the number
	// of passes the connectivity phase ran; onDeadEndDraw sees every chance
	// draw of the dead-end phase.
	onLater       func(discardsSoFar int)
	onDeadEndDraw func(node int, ok bool)
	passes        int
}

type laneStart struct {
	idx  int
	away int // direction directly away from the door
}

func (g *gen) idx(x, y int) int { return y*g.w + x }

func (g *gen) inside(x, y int) bool { return x >= 0 && y >= 0 && x < g.w && y < g.h }

// interior: inside the border ring.
func (g *gen) interior(x, y int) bool { return x >= 1 && y >= 1 && x <= g.w-2 && y <= g.h-2 }

func (g *gen) isOpen(i int) bool { return g.kind[i] != KindRock }

// openDeg counts the open 4-neighbors of a square.
func (g *gen) openDeg(i int) int {
	x, y := i%g.w, i/g.w
	n := 0
	for d := 0; d < 4; d++ {
		nx, ny := x+dx[d], y+dy[d]
		if g.inside(nx, ny) && g.isOpen(g.idx(nx, ny)) {
			n++
		}
	}
	return n
}

// nodeOK: a lattice node is usable when its own square and the four around
// it are not blocked (spec 3.2).
func (g *gen) nodeOK(x, y int) bool {
	if !g.interior(x, y) {
		return false
	}
	if !g.anyBlocked {
		return true
	}
	if g.blocked[g.idx(x, y)] {
		return false
	}
	for d := 0; d < 4; d++ {
		if g.blocked[g.idx(x+dx[d], y+dy[d])] {
			return false
		}
	}
	return true
}

// freeNode: a usable node that is rock (not a room, not a corridor).
func (g *gen) freeNode(x, y int) bool {
	return g.nodeOK(x, y) && g.kind[g.idx(x, y)] == KindRock
}

func pairKey(a, b int) uint32 {
	if a > b {
		a, b = b, a
	}
	return uint32(a)<<16 | uint32(b) //nolint:gosec // G115: room IDs are at most 500
}

// Generate turns (options, seed) into a dungeon level (spec 1 to 6). It is
// pure and deterministic: the same Options always give a byte-identical
// result.
func Generate(opts Options) (*Dungeon, error) { return generate(opts, nil) }

func generate(opts Options, hook func(stage string, g *gen)) (*Dungeon, error) {
	o, err := normalize(opts)
	if err != nil {
		return nil, err
	}
	g := newGen(o)
	g.hook = hook
	g.stage("grid")
	g.buildMask()
	g.stage("mask")
	if err := g.placeRooms(); err != nil {
		return nil, err
	}
	g.stage("rooms")
	g.placeDoors()
	g.stage("doors")
	g.carveCorridors()
	g.stage("corridors")
	g.connect()
	g.stage("connect")
	g.placeStairs()
	g.stage("stairs")
	g.removeDeadEnds()
	g.stage("deadends")
	d, err := g.finish()
	if err != nil {
		return nil, err
	}
	if o.SelfCheck {
		if err := check(d); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSelfCheck, err)
		}
	}
	return d, nil
}

// newGen allocates the working state for normalised options.
func newGen(o Options) *gen {
	n := o.Width * o.Height
	g := &gen{
		o: o, aspect: aspectX100(o.AspectLimit),
		w: o.Width, h: o.Height, nx: (o.Width - 1) / 2, ny: (o.Height - 1) / 2,
		kind: make([]Kind, n), rid: make([]uint16, n), blocked: make([]bool, n),
		lane: make([]bool, n), protected: make([]bool, n), joined: make([]bool, n),
		doorKind: make([]DoorKind, n), doorTrap: make([]bool, n),
		roomByID: make([]*room, 1, 32), directPairs: make(map[uint32]struct{}),
	}
	g.kindRNG = stream(o.Seed, phaseKinds)
	return g
}

func (g *gen) stage(name string) {
	if g.hook != nil {
		g.hook(name, g)
	}
}

// addRoom registers a room and paints its floor.
func (g *gen) addRoom(x, y, w, h int) *room {
	r := &room{id: len(g.roomByID), x: x, y: y, w: w, h: h}
	g.roomByID = append(g.roomByID, r)
	g.rooms = append(g.rooms, r)
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			i := g.idx(xx, yy)
			g.kind[i] = KindRoom
			g.rid[i] = uint16(r.id) //nolint:gosec // G115: room IDs are at most 500
		}
	}
	r.centerX, r.centerY = x+w/2, y+h/2
	return r
}

// compact removes the discarded rooms from the ordered list.
func (g *gen) compact() {
	g.rooms = slices.DeleteFunc(g.rooms, func(r *room) bool { return r.discarded })
}
