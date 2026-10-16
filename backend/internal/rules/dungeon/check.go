package dungeon

import (
	"fmt"
	"slices"
)

const (
	// minGridSide is the smallest side of a grid (spec 2).
	minGridSide = 15
	// minDoorGap is the least distance between two doors on one side of a
	// room (invariant 12).
	minDoorGap = 4
	// stairDoorClearance is how far a room stair keeps from every door.
	stairDoorClearance = 2
	// allDeadendsRemoved is the DeadendRemoval at which no dead end may
	// remain (invariant 11).
	allDeadendsRemoved = 100
)

// checker holds what the invariants of check share: the dungeon and what an
// earlier invariant computes for a later one.
type checker struct {
	d    *Dungeon
	o    Options
	w, h int
	// floor is the total area of the listed rooms (rooms), compared with the
	// floor squares in the grid (roomSquares).
	floor int
	// protected marks the squares the stairs depend on (stairs); the dead
	// end scan (deadEnds) leaves them alone.
	protected map[int]bool
}

// check runs every invariant of spec 5 on a dungeon and returns the first
// one that is broken. The generator's self-check (Options.SelfCheck) and the
// property tests call it; it is linear in the size of the grid. The order of
// the steps is part of its contract: the error is always the first of them
// that fails.
func check(d *Dungeon) error {
	c := &checker{d: d, o: d.Options, w: d.Width, h: d.Height, protected: map[int]bool{}}
	steps := [...]func() error{
		c.gridSize, c.border, c.mask, c.rooms, c.roomSquares, c.roomRings, c.corridorSquares,
		c.doors, c.exits, c.directPairs, c.doorSquares, c.stairs, c.entrance,
		c.reachability, c.deadEnds, c.corridorTable, c.discardedRooms,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// at is the kind of a square; rock outside the grid.
func (c *checker) at(x, y int) Kind {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return KindRock
	}
	return c.d.Kinds[y*c.w+x]
}

// degree counts the open neighbors of a square.
func (c *checker) degree(x, y int) int {
	deg := 0
	for dd := range 4 {
		if c.at(x+dx[dd], y+dy[dd]) != KindRock {
			deg++
		}
	}
	return deg
}

// 1. Bounds.

func (c *checker) gridSize() error {
	if c.w != c.o.Width || c.h != c.o.Height {
		return fmt.Errorf("bounds: grid %dx%d but the effective options say %dx%d", c.w, c.h, c.o.Width, c.o.Height)
	}
	if !c.gridIsWellFormed() {
		return fmt.Errorf("bounds: bad grid %dx%d with %d squares", c.w, c.h, len(c.d.Kinds))
	}
	return nil
}

// gridIsWellFormed: odd sides of at least minGridSide and one entry per
// square in every layer.
func (c *checker) gridIsWellFormed() bool {
	n := c.w * c.h
	d := c.d
	sides := c.w >= minGridSide && c.h >= minGridSide && c.w%2 == 1 && c.h%2 == 1
	return sides && len(d.Kinds) == n && len(d.RoomIDs) == n && len(d.Mask) == n && len(d.CorridorIDs) == n
}

func (c *checker) border() error {
	for x := range c.w {
		if c.at(x, 0) != KindRock || c.at(x, c.h-1) != KindRock {
			return fmt.Errorf("bounds: the border ring is open at column %d", x)
		}
	}
	for y := range c.h {
		if c.at(0, y) != KindRock || c.at(c.w-1, y) != KindRock {
			return fmt.Errorf("bounds: the border ring is open at row %d", y)
		}
	}
	return nil
}

// 4. Nothing open in a blocked square.
func (c *checker) mask() error {
	for i, k := range c.d.Kinds {
		if k != KindRock && !c.d.Mask[i] {
			return fmt.Errorf("blocked: square (%d,%d) is open but outside the mask", i%c.w, i/c.w)
		}
	}
	return nil
}

// 2, 3. Rooms: parity, sizes, aspect, floors painted and disjoint.

func (c *checker) rooms() error {
	if len(c.d.Rooms) == 0 {
		return fmt.Errorf("rooms: none")
	}
	prev := 0
	for _, r := range c.d.Rooms {
		if err := c.room(r, prev); err != nil {
			return err
		}
		prev = r.ID
		c.floor += r.Area()
	}
	return nil
}

func (c *checker) room(r Room, prev int) error {
	if err := c.roomListing(r, prev); err != nil {
		return err
	}
	if err := c.roomSize(r); err != nil {
		return err
	}
	if err := c.roomPlace(r); err != nil {
		return err
	}
	if err := c.roomFloor(r); err != nil {
		return err
	}
	return c.roomSurround(r)
}

func (c *checker) roomListing(r Room, prev int) error {
	if r.ID <= prev {
		return fmt.Errorf("rooms: IDs not ascending at %d", r.ID)
	}
	if slices.Contains(c.d.DiscardedRooms, r.ID) {
		return fmt.Errorf("rooms: room %d is listed and discarded", r.ID)
	}
	return nil
}

// roomSize: odd position and sides, sides within the options.
func (c *checker) roomSize(r Room) error {
	if r.X%2 != 1 || r.Y%2 != 1 || r.Width%2 != 1 || r.Height%2 != 1 {
		return fmt.Errorf("parity: room %d at (%d,%d) %dx%d", r.ID, r.X, r.Y, r.Width, r.Height)
	}
	for _, s := range [2]int{r.Width, r.Height} {
		if s < c.o.RoomSideMin || s > c.o.RoomSideMax {
			return fmt.Errorf("parity: room %d side %d outside %d..%d", r.ID, s, c.o.RoomSideMin, c.o.RoomSideMax)
		}
	}
	return nil
}

// roomPlace: the aspect limit and the room inside the border.
func (c *checker) roomPlace(r Room) error {
	aspect := aspectX100(c.o.AspectLimit)
	if max(r.Width, r.Height)*100 > aspect*min(r.Width, r.Height) {
		return fmt.Errorf("aspect: room %d is %dx%d, limit %d/100", r.ID, r.Width, r.Height, aspect)
	}
	if r.X+r.Width-1 > c.w-2 || r.Y+r.Height-1 > c.h-2 {
		return fmt.Errorf("bounds: room %d leaves the border", r.ID)
	}
	return nil
}

// roomFloor: every square of the rectangle is floor of this room.
func (c *checker) roomFloor(r Room) error {
	for y := r.Y; y < r.Y+r.Height; y++ {
		for x := r.X; x < r.X+r.Width; x++ {
			if c.at(x, y) != KindRoom || int(c.d.RoomIDs[y*c.w+x]) != r.ID {
				return fmt.Errorf("overlap: room %d floor square (%d,%d) is not its own", r.ID, x, y)
			}
		}
	}
	return nil
}

// roomSurround: the ring around the room is inside the mask.
func (c *checker) roomSurround(r Room) error {
	for y := r.Y - 1; y <= r.Y+r.Height; y++ {
		for x := r.X - 1; x <= r.X+r.Width; x++ {
			if !c.d.Mask[y*c.w+x] {
				return fmt.Errorf("blocked: room %d ring touches a blocked square", r.ID)
			}
		}
	}
	return nil
}

// roomSquares: the room floors of the grid are exactly the listed rooms'.
func (c *checker) roomSquares() error {
	count := 0
	for i, k := range c.d.Kinds {
		if k == KindRoom {
			count++
			if c.d.RoomIDs[i] == 0 {
				return fmt.Errorf("rooms: floor square (%d,%d) has no room", i%c.w, i/c.w)
			}
		} else if c.d.RoomIDs[i] != 0 {
			return fmt.Errorf("rooms: non-floor square (%d,%d) has room ID %d", i%c.w, i/c.w, c.d.RoomIDs[i])
		}
	}
	if count != c.floor {
		return fmt.Errorf("overlap: %d floor squares for rooms of %d", count, c.floor)
	}
	return nil
}

// roomRings: no corridor square on the ring of a room (7).
func (c *checker) roomRings() error {
	for _, r := range c.d.Rooms {
		for y := r.Y - 1; y <= r.Y+r.Height; y++ {
			for x := r.X - 1; x <= r.X+r.Width; x++ {
				if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
					continue
				}
				if c.at(x, y) == KindCorridor {
					return fmt.Errorf("corridor: square (%d,%d) is on the ring of room %d", x, y, r.ID)
				}
			}
		}
	}
	return nil
}

// 7, 8. Corridors never touch a floor, never a 2 by 2 block.
func (c *checker) corridorSquares() error {
	for i, k := range c.d.Kinds {
		if k != KindCorridor {
			continue
		}
		x, y := i%c.w, i/c.w
		for dd := range 4 {
			if c.at(x+dx[dd], y+dy[dd]) == KindRoom {
				return fmt.Errorf("corridor: square (%d,%d) touches a room floor", x, y)
			}
		}
		if c.at(x+1, y) == KindCorridor && c.at(x, y+1) == KindCorridor && c.at(x+1, y+1) == KindCorridor {
			return fmt.Errorf("corridor: 2x2 block at (%d,%d)", x, y)
		}
	}
	return nil
}

// 6. Doors.

func (c *checker) doors() error {
	doorIDs := map[int]bool{}
	for k, dr := range c.d.Doors {
		if err := c.door(k, dr, doorIDs); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) door(k int, dr Door, doorIDs map[int]bool) error {
	if dr.ID != k+1 || doorIDs[dr.ID] {
		return fmt.Errorf("doors: bad ID %d at index %d", dr.ID, k)
	}
	doorIDs[dr.ID] = true
	if err := c.doorSquare(dr); err != nil {
		return err
	}
	ax, ay := doorCrossing(dr)
	if (dr.X%2 == 0) != (dr.Axis == VerticalWall) {
		return fmt.Errorf("doors: door %d axis does not match its square", dr.ID)
	}
	if err := c.doorSides(dr, ax, ay); err != nil {
		return err
	}
	return c.doorRooms(dr, ax, ay)
}

// doorSquare: the door is on a door square on a wall line, with a kind that
// may carry what it carries.
func (c *checker) doorSquare(dr Door) error {
	if c.at(dr.X, dr.Y) != KindDoor {
		return fmt.Errorf("doors: door %d is not on a door square", dr.ID)
	}
	if dr.X%2 == dr.Y%2 { // exactly one coordinate is even: (odd, even) horizontal wall, (even, odd) vertical
		return fmt.Errorf("doors: door %d at (%d,%d) is not on a wall line", dr.ID, dr.X, dr.Y)
	}
	if dr.Trapped && !dr.Kind.canBeTrapped() {
		return fmt.Errorf("doors: %s door %d is trapped", dr.Kind, dr.ID)
	}
	if dr.Kind > DoorSecret {
		return fmt.Errorf("doors: door %d has kind %d", dr.ID, dr.Kind)
	}
	return nil
}

// doorCrossing is the step of the axis a door is crossed along.
func doorCrossing(dr Door) (ax, ay int) {
	if dr.Axis == VerticalWall {
		return 1, 0
	}
	return 0, 1
}

// doorSides: open on both ends, rock on both sides of the wall line.
func (c *checker) doorSides(dr Door, ax, ay int) error {
	if c.at(dr.X-ax, dr.Y-ay) == KindRock || c.at(dr.X+ax, dr.Y+ay) == KindRock {
		return fmt.Errorf("doors: door %d (%d,%d) does not open on both sides", dr.ID, dr.X, dr.Y)
	}
	if c.at(dr.X-ay, dr.Y-ax) != KindRock || c.at(dr.X+ay, dr.Y+ax) != KindRock {
		return fmt.Errorf("doors: door %d (%d,%d) has no rock beside it", dr.ID, dr.X, dr.Y)
	}
	return nil
}

// doorRooms: the rooms at the ends of the door match the ones recorded.
func (c *checker) doorRooms(dr Door, ax, ay int) error {
	ra, rb := int(c.d.RoomIDs[(dr.Y-ay)*c.w+dr.X-ax]), int(c.d.RoomIDs[(dr.Y+ay)*c.w+dr.X+ax])
	if ra == 0 && rb == 0 {
		return fmt.Errorf("doors: door %d is not on a room's wall", dr.ID)
	}
	if ra != 0 && ra == rb {
		return fmt.Errorf("doors: door %d joins room %d to itself", dr.ID, ra)
	}
	if doorRecordedWrong(dr, ra, rb) {
		return fmt.Errorf("doors: door %d joins %d,%d but is recorded %d,%d", dr.ID, ra, rb, dr.RoomA, dr.RoomB)
	}
	return nil
}

// doorRecordedWrong: a direct door records both rooms, a corridor door only
// RoomA.
func doorRecordedWrong(dr Door, ra, rb int) bool {
	direct := ra != 0 && rb != 0
	if direct {
		return dr.RoomA != ra || dr.RoomB != rb
	}
	return dr.RoomB != 0
}

// 12. Exits are exactly the doors on each ring, in both rooms of a direct
// door, and two doors on one side are at least minDoorGap squares apart.

func (c *checker) exits() error {
	exitCount := 0
	for _, r := range c.d.Rooms {
		exitCount += len(r.Exits)
		if err := c.roomExits(r); err != nil {
			return err
		}
	}
	directDoors := 0
	for _, dr := range c.d.Doors {
		if dr.RoomB != 0 {
			directDoors++
		}
	}
	// each door is an exit of one room (corridor door) or two (direct)
	if want := len(c.d.Doors) + directDoors; exitCount != want {
		return fmt.Errorf("exits: %d exits for %d doors (%d direct)", exitCount, len(c.d.Doors), directDoors)
	}
	return nil
}

func (c *checker) roomExits(r Room) error {
	perSide := [4][]int{}
	for _, e := range r.Exits {
		if err := c.exit(r, e); err != nil {
			return err
		}
		p := e.X
		if e.Side == East || e.Side == West {
			p = e.Y
		}
		perSide[e.Side] = append(perSide[e.Side], p)
	}
	if err := exitSpacing(r, perSide); err != nil {
		return err
	}
	if len(c.d.Rooms) > 1 && len(r.Exits) == 0 {
		return fmt.Errorf("exits: room %d has none", r.ID)
	}
	return nil
}

// exit: the exit is its door, seen from this room.
func (c *checker) exit(r Room, e Exit) error {
	if c.at(e.X, e.Y) != KindDoor {
		return fmt.Errorf("exits: room %d lists a non-door (%d,%d)", r.ID, e.X, e.Y)
	}
	if e.DoorID < 1 || e.DoorID > len(c.d.Doors) {
		return fmt.Errorf("exits: room %d lists door %d", r.ID, e.DoorID)
	}
	dr := c.d.Doors[e.DoorID-1]
	if dr.X != e.X || dr.Y != e.Y || dr.Kind != e.Kind || dr.Trapped != e.Trapped {
		return fmt.Errorf("exits: room %d exit does not match door %d", r.ID, e.DoorID)
	}
	if dr.RoomA != r.ID && dr.RoomB != r.ID {
		return fmt.Errorf("exits: door %d is not a door of room %d", dr.ID, r.ID)
	}
	if e.Other != 0 && (e.Other == r.ID || (dr.RoomA != e.Other && dr.RoomB != e.Other)) {
		return fmt.Errorf("exits: door %d far room %d", dr.ID, e.Other)
	}
	return nil
}

// exitSpacing: the doors of one side of a room are far enough apart.
func exitSpacing(r Room, perSide [4][]int) error {
	for s := range perSide {
		slices.Sort(perSide[s])
		for k := 1; k < len(perSide[s]); k++ {
			if perSide[s][k]-perSide[s][k-1] < minDoorGap {
				return fmt.Errorf("doors: room %d has two doors %d apart on its %s side", r.ID, perSide[s][k]-perSide[s][k-1], Side(s))
			}
		}
	}
	return nil
}

// directPairs: two rooms share at most one direct door.
func (c *checker) directPairs() error {
	pairs := map[[2]int]bool{}
	for _, dr := range c.d.Doors {
		if dr.RoomB != 0 {
			p := [2]int{min(dr.RoomA, dr.RoomB), max(dr.RoomA, dr.RoomB)}
			if pairs[p] {
				return fmt.Errorf("doors: rooms %d and %d share two direct doors", p[0], p[1])
			}
			pairs[p] = true
		}
	}
	return nil
}

// doorSquares: every door square in the grid is in the list.
func (c *checker) doorSquares() error {
	doorSquares := 0
	for _, k := range c.d.Kinds {
		if k == KindDoor {
			doorSquares++
		}
	}
	if doorSquares != len(c.d.Doors) {
		return fmt.Errorf("doors: %d door squares, %d doors", doorSquares, len(c.d.Doors))
	}
	return nil
}

// 9, 10. Stairs and the entrance.

func (c *checker) stairs() error {
	d := c.d
	if d.StairsPlaced != len(d.Stairs) || len(d.Stairs) > c.o.Stairs {
		return fmt.Errorf("stairs: placed %d, listed %d, wanted %d", d.StairsPlaced, len(d.Stairs), c.o.Stairs)
	}
	for k, s := range d.Stairs {
		if err := c.stair(k, s); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) stair(k int, s Stair) error {
	if err := c.stairPlace(k, s); err != nil {
		return err
	}
	c.protected[s.Y*c.w+s.X] = true
	if s.InRoom == 0 {
		return c.corridorStair(k, s)
	}
	return c.roomStair(k, s)
}

// stairPlace: on open floor, away from the earlier stairs, up first and down
// second.
func (c *checker) stairPlace(k int, s Stair) error {
	if c.at(s.X, s.Y) == KindRock || c.at(s.X, s.Y) == KindDoor {
		return fmt.Errorf("stairs: stair %d is not on open floor", k)
	}
	for j := range k {
		if abs(c.d.Stairs[j].X-s.X)+abs(c.d.Stairs[j].Y-s.Y) < minStairGap {
			return fmt.Errorf("stairs: stairs %d and %d are closer than %d", j, k, minStairGap)
		}
	}
	if k == 0 && s.Kind != StairUp {
		return fmt.Errorf("stairs: the first stair is not up")
	}
	if k == 1 && s.Kind != StairDown {
		return fmt.Errorf("stairs: the second stair is not down")
	}
	return nil
}

// corridorStair: at a dead end, facing away from the corridor; the square
// the corridor continues on is protected too.
func (c *checker) corridorStair(k int, s Stair) error {
	if c.at(s.X, s.Y) != KindCorridor {
		return fmt.Errorf("stairs: stair %d is not in a corridor", k)
	}
	deg, side := 0, -1
	for dd := range 4 {
		if c.at(s.X+dx[dd], s.Y+dy[dd]) != KindRock {
			deg++
			side = dd
		}
	}
	if deg != 1 || c.at(s.X+dx[side], s.Y+dy[side]) != KindCorridor {
		return fmt.Errorf("stairs: stair %d is not at a dead end", k)
	}
	if Side((side+2)%4) != s.Facing {
		return fmt.Errorf("stairs: stair %d faces the wrong way", k)
	}
	c.protected[(s.Y+dy[side])*c.w+s.X+dx[side]] = true
	return nil
}

// roomStair: on a corner of its room, away from every door.
func (c *checker) roomStair(k int, s Stair) error {
	r, ok := c.d.Room(s.InRoom)
	if !ok || c.at(s.X, s.Y) != KindRoom || int(c.d.RoomIDs[s.Y*c.w+s.X]) != s.InRoom {
		return fmt.Errorf("stairs: stair %d is not in room %d", k, s.InRoom)
	}
	if (s.X != r.X && s.X != r.X+r.Width-1) || (s.Y != r.Y && s.Y != r.Y+r.Height-1) {
		return fmt.Errorf("stairs: stair %d is not at a corner of room %d", k, r.ID)
	}
	for _, dr := range c.d.Doors {
		if abs(dr.X-s.X)+abs(dr.Y-s.Y) <= stairDoorClearance {
			return fmt.Errorf("stairs: room stair %d has door %d within %d squares", k, dr.ID, stairDoorClearance)
		}
	}
	return nil
}

func (c *checker) entrance() error {
	e := c.d.Entrance
	if c.at(e.X, e.Y) == KindRock {
		return fmt.Errorf("entrance: (%d,%d) is not open", e.X, e.Y)
	}
	if len(c.d.Stairs) > 0 {
		if s := c.d.Stairs[0]; e.X != s.X || e.Y != s.Y || !e.OnStairs {
			return fmt.Errorf("entrance: not on the first stair")
		}
		return nil
	}
	first := c.d.Rooms[0]
	cx, cy := first.Center()
	if e.X != cx || e.Y != cy || e.OnStairs || e.Room != first.ID {
		return fmt.Errorf("entrance: not the center of room %d", first.ID)
	}
	return nil
}

// 5. Reachability from the entrance.
func (c *checker) reachability() error {
	reached := c.flood()
	open := 0
	for _, k := range c.d.Kinds {
		if k != KindRock {
			open++
		}
	}
	if reached != open {
		return fmt.Errorf("reachability: %d of %d open squares reached from the entrance", reached, open)
	}
	return nil
}

// flood counts the open squares reachable from the entrance.
func (c *checker) flood() int {
	e := c.d.Entrance
	seen := make([]bool, c.w*c.h)
	stack := []int{e.Y*c.w + e.X}
	seen[stack[0]] = true
	reached := 0
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		reached++
		x, y := u%c.w, u/c.w
		for dd := range 4 {
			if c.at(x+dx[dd], y+dy[dd]) == KindRock {
				continue
			}
			v := (y+dy[dd])*c.w + x + dx[dd]
			if !seen[v] {
				seen[v] = true
				stack = append(stack, v)
			}
		}
	}
	return reached
}

// 11. Dead ends.
func (c *checker) deadEnds() error {
	if c.o.DeadendRemoval != allDeadendsRemoved {
		return nil
	}
	for i, k := range c.d.Kinds {
		if k != KindCorridor || c.protected[i] {
			continue
		}
		if c.degree(i%c.w, i/c.w) == 1 {
			return fmt.Errorf("dead ends: square (%d,%d) remains with removal %d", i%c.w, i/c.w, allDeadendsRemoved)
		}
	}
	return nil
}

// corridorTable: the corridors list and the grid's corridor IDs agree.
func (c *checker) corridorTable() error {
	d := c.d
	for k, cor := range d.Corridors {
		if cor.ID != k+1 || d.CorridorIDs[cor.FirstY*c.w+cor.FirstX] != uint16(cor.ID) { //nolint:gosec // G115: corridor IDs fit uint16 (at most ~20 000 nodes)
			return fmt.Errorf("corridors: bad corridor %d", cor.ID)
		}
	}
	for i, k := range d.Kinds {
		if (k == KindCorridor) != (d.CorridorIDs[i] != 0) {
			return fmt.Errorf("corridors: square (%d,%d) and its corridor ID disagree", i%c.w, i/c.w)
		}
	}
	return nil
}

func (c *checker) discardedRooms() error {
	ids := c.d.DiscardedRooms
	for k := 1; k < len(ids); k++ {
		if ids[k] <= ids[k-1] {
			return fmt.Errorf("rooms: discarded IDs not ascending")
		}
	}
	return nil
}
