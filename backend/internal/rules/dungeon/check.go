package dungeon

import (
	"fmt"
	"slices"
)

// check runs every invariant of spec 5 on a dungeon and returns the first
// one that is broken. The generator's self-check (Options.SelfCheck) and the
// property tests call it; it is linear in the size of the grid.
func check(d *Dungeon) error {
	w, h := d.Width, d.Height
	o := d.Options
	n := w * h
	// 1. Bounds.
	if w != o.Width || h != o.Height {
		return fmt.Errorf("bounds: grid %dx%d but the effective options say %dx%d", w, h, o.Width, o.Height)
	}
	if w < 15 || h < 15 || w%2 == 0 || h%2 == 0 || len(d.Kinds) != n || len(d.RoomIDs) != n || len(d.Mask) != n || len(d.CorridorIDs) != n {
		return fmt.Errorf("bounds: bad grid %dx%d with %d squares", w, h, len(d.Kinds))
	}
	in := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h }
	at := func(x, y int) Kind {
		if !in(x, y) {
			return KindRock
		}
		return d.Kinds[y*w+x]
	}
	for x := range w {
		if at(x, 0) != KindRock || at(x, h-1) != KindRock {
			return fmt.Errorf("bounds: the border ring is open at column %d", x)
		}
	}
	for y := range h {
		if at(0, y) != KindRock || at(w-1, y) != KindRock {
			return fmt.Errorf("bounds: the border ring is open at row %d", y)
		}
	}
	// 4. Nothing open in a blocked square.
	for i, k := range d.Kinds {
		if k != KindRock && !d.Mask[i] {
			return fmt.Errorf("blocked: square (%d,%d) is open but outside the mask", i%w, i/w)
		}
	}
	if len(d.Rooms) == 0 {
		return fmt.Errorf("rooms: none")
	}
	// 2, 3. Rooms: parity, sizes, aspect, floors painted and disjoint.
	aspect := aspectX100(o.AspectLimit)
	floor := 0
	prev := 0
	for _, r := range d.Rooms {
		if r.ID <= prev {
			return fmt.Errorf("rooms: IDs not ascending at %d", r.ID)
		}
		prev = r.ID
		if slices.Contains(d.DiscardedRooms, r.ID) {
			return fmt.Errorf("rooms: room %d is listed and discarded", r.ID)
		}
		if r.X%2 != 1 || r.Y%2 != 1 || r.Width%2 != 1 || r.Height%2 != 1 {
			return fmt.Errorf("parity: room %d at (%d,%d) %dx%d", r.ID, r.X, r.Y, r.Width, r.Height)
		}
		for _, s := range [2]int{r.Width, r.Height} {
			if s < o.RoomSideMin || s > o.RoomSideMax {
				return fmt.Errorf("parity: room %d side %d outside %d..%d", r.ID, s, o.RoomSideMin, o.RoomSideMax)
			}
		}
		if max(r.Width, r.Height)*100 > aspect*min(r.Width, r.Height) {
			return fmt.Errorf("aspect: room %d is %dx%d, limit %d/100", r.ID, r.Width, r.Height, aspect)
		}
		if r.X+r.Width-1 > w-2 || r.Y+r.Height-1 > h-2 {
			return fmt.Errorf("bounds: room %d leaves the border", r.ID)
		}
		for y := r.Y; y < r.Y+r.Height; y++ {
			for x := r.X; x < r.X+r.Width; x++ {
				if at(x, y) != KindRoom || int(d.RoomIDs[y*w+x]) != r.ID {
					return fmt.Errorf("overlap: room %d floor square (%d,%d) is not its own", r.ID, x, y)
				}
			}
		}
		for y := r.Y - 1; y <= r.Y+r.Height; y++ {
			for x := r.X - 1; x <= r.X+r.Width; x++ {
				if !d.Mask[y*w+x] {
					return fmt.Errorf("blocked: room %d ring touches a blocked square", r.ID)
				}
			}
		}
		floor += r.Area()
	}
	count := 0
	for i, k := range d.Kinds {
		if k == KindRoom {
			count++
			if d.RoomIDs[i] == 0 {
				return fmt.Errorf("rooms: floor square (%d,%d) has no room", i%w, i/w)
			}
		} else if d.RoomIDs[i] != 0 {
			return fmt.Errorf("rooms: non-floor square (%d,%d) has room ID %d", i%w, i/w, d.RoomIDs[i])
		}
	}
	if count != floor {
		return fmt.Errorf("overlap: %d floor squares for rooms of %d", count, floor)
	}
	// Ring squares per room (7).
	for _, r := range d.Rooms {
		for y := r.Y - 1; y <= r.Y+r.Height; y++ {
			for x := r.X - 1; x <= r.X+r.Width; x++ {
				if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
					continue
				}
				if at(x, y) == KindCorridor {
					return fmt.Errorf("corridor: square (%d,%d) is on the ring of room %d", x, y, r.ID)
				}
			}
		}
	}
	// 7, 8. Corridors never touch a floor, never a 2 by 2 block.
	for i, k := range d.Kinds {
		if k != KindCorridor {
			continue
		}
		x, y := i%w, i/w
		for dd := range 4 {
			if at(x+dx[dd], y+dy[dd]) == KindRoom {
				return fmt.Errorf("corridor: square (%d,%d) touches a room floor", x, y)
			}
		}
		if at(x+1, y) == KindCorridor && at(x, y+1) == KindCorridor && at(x+1, y+1) == KindCorridor {
			return fmt.Errorf("corridor: 2x2 block at (%d,%d)", x, y)
		}
	}
	// 6. Doors.
	doorIDs := map[int]bool{}
	for k, dr := range d.Doors {
		if dr.ID != k+1 || doorIDs[dr.ID] {
			return fmt.Errorf("doors: bad ID %d at index %d", dr.ID, k)
		}
		doorIDs[dr.ID] = true
		if at(dr.X, dr.Y) != KindDoor {
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
		ax, ay := 0, 1 // crossing axis
		if dr.Axis == VerticalWall {
			ax, ay = 1, 0
		}
		if (dr.X%2 == 0) != (dr.Axis == VerticalWall) {
			return fmt.Errorf("doors: door %d axis does not match its square", dr.ID)
		}
		if at(dr.X-ax, dr.Y-ay) == KindRock || at(dr.X+ax, dr.Y+ay) == KindRock {
			return fmt.Errorf("doors: door %d (%d,%d) does not open on both sides", dr.ID, dr.X, dr.Y)
		}
		if at(dr.X-ay, dr.Y-ax) != KindRock || at(dr.X+ay, dr.Y+ax) != KindRock {
			return fmt.Errorf("doors: door %d (%d,%d) has no rock beside it", dr.ID, dr.X, dr.Y)
		}
		ra, rb := int(d.RoomIDs[(dr.Y-ay)*w+dr.X-ax]), int(d.RoomIDs[(dr.Y+ay)*w+dr.X+ax])
		if ra == 0 && rb == 0 {
			return fmt.Errorf("doors: door %d is not on a room's wall", dr.ID)
		}
		if ra != 0 && ra == rb {
			return fmt.Errorf("doors: door %d joins room %d to itself", dr.ID, ra)
		}
		if ra != 0 && rb != 0 && (dr.RoomA != ra || dr.RoomB != rb) || (ra == 0 || rb == 0) && dr.RoomB != 0 {
			return fmt.Errorf("doors: door %d joins %d,%d but is recorded %d,%d", dr.ID, ra, rb, dr.RoomA, dr.RoomB)
		}
	}
	// 12. Exits are exactly the doors on each ring, in both rooms of a direct
	// door, and two doors on one side are at least 4 squares apart.
	exitCount := 0
	for _, r := range d.Rooms {
		perSide := [4][]int{}
		for _, e := range r.Exits {
			exitCount++
			if at(e.X, e.Y) != KindDoor {
				return fmt.Errorf("exits: room %d lists a non-door (%d,%d)", r.ID, e.X, e.Y)
			}
			if e.DoorID < 1 || e.DoorID > len(d.Doors) {
				return fmt.Errorf("exits: room %d lists door %d", r.ID, e.DoorID)
			}
			dr := d.Doors[e.DoorID-1]
			if dr.X != e.X || dr.Y != e.Y || dr.Kind != e.Kind || dr.Trapped != e.Trapped {
				return fmt.Errorf("exits: room %d exit does not match door %d", r.ID, e.DoorID)
			}
			if dr.RoomA != r.ID && dr.RoomB != r.ID {
				return fmt.Errorf("exits: door %d is not a door of room %d", dr.ID, r.ID)
			}
			if e.Other != 0 {
				if e.Other == r.ID || (dr.RoomA != e.Other && dr.RoomB != e.Other) {
					return fmt.Errorf("exits: door %d far room %d", dr.ID, e.Other)
				}
			}
			p := e.X
			if e.Side == East || e.Side == West {
				p = e.Y
			}
			perSide[e.Side] = append(perSide[e.Side], p)
		}
		for s := range perSide {
			slices.Sort(perSide[s])
			for k := 1; k < len(perSide[s]); k++ {
				if perSide[s][k]-perSide[s][k-1] < 4 {
					return fmt.Errorf("doors: room %d has two doors %d apart on its %s side", r.ID, perSide[s][k]-perSide[s][k-1], Side(s))
				}
			}
		}
		if len(d.Rooms) > 1 && len(r.Exits) == 0 {
			return fmt.Errorf("exits: room %d has none", r.ID)
		}
	}
	directDoors := 0
	for _, dr := range d.Doors {
		if dr.RoomB != 0 {
			directDoors++
		}
	}
	// each door is an exit of one room (corridor door) or two (direct)
	if want := len(d.Doors) + directDoors; exitCount != want {
		return fmt.Errorf("exits: %d exits for %d doors (%d direct)", exitCount, len(d.Doors), directDoors)
	}
	pairs := map[[2]int]bool{}
	for _, dr := range d.Doors {
		if dr.RoomB != 0 {
			p := [2]int{min(dr.RoomA, dr.RoomB), max(dr.RoomA, dr.RoomB)}
			if pairs[p] {
				return fmt.Errorf("doors: rooms %d and %d share two direct doors", p[0], p[1])
			}
			pairs[p] = true
		}
	}
	// Every door square in the grid is in the list.
	doorSquares := 0
	for _, k := range d.Kinds {
		if k == KindDoor {
			doorSquares++
		}
	}
	if doorSquares != len(d.Doors) {
		return fmt.Errorf("doors: %d door squares, %d doors", doorSquares, len(d.Doors))
	}
	// 9, 10. Stairs and the entrance.
	if d.StairsPlaced != len(d.Stairs) || len(d.Stairs) > o.Stairs {
		return fmt.Errorf("stairs: placed %d, listed %d, wanted %d", d.StairsPlaced, len(d.Stairs), o.Stairs)
	}
	protected := map[int]bool{}
	for k, s := range d.Stairs {
		if at(s.X, s.Y) == KindRock || at(s.X, s.Y) == KindDoor {
			return fmt.Errorf("stairs: stair %d is not on open floor", k)
		}
		for j := range k {
			if abs(d.Stairs[j].X-s.X)+abs(d.Stairs[j].Y-s.Y) < minStairGap {
				return fmt.Errorf("stairs: stairs %d and %d are closer than %d", j, k, minStairGap)
			}
		}
		if k == 0 && s.Kind != StairUp {
			return fmt.Errorf("stairs: the first stair is not up")
		}
		if k == 1 && s.Kind != StairDown {
			return fmt.Errorf("stairs: the second stair is not down")
		}
		protected[s.Y*w+s.X] = true
		if s.InRoom == 0 {
			if at(s.X, s.Y) != KindCorridor {
				return fmt.Errorf("stairs: stair %d is not in a corridor", k)
			}
			deg, side := 0, -1
			for dd := range 4 {
				if at(s.X+dx[dd], s.Y+dy[dd]) != KindRock {
					deg++
					side = dd
				}
			}
			if deg != 1 || at(s.X+dx[side], s.Y+dy[side]) != KindCorridor {
				return fmt.Errorf("stairs: stair %d is not at a dead end", k)
			}
			if Side((side+2)%4) != s.Facing {
				return fmt.Errorf("stairs: stair %d faces the wrong way", k)
			}
			protected[(s.Y+dy[side])*w+s.X+dx[side]] = true
		} else {
			r, ok := d.Room(s.InRoom)
			if !ok || at(s.X, s.Y) != KindRoom || int(d.RoomIDs[s.Y*w+s.X]) != s.InRoom {
				return fmt.Errorf("stairs: stair %d is not in room %d", k, s.InRoom)
			}
			if (s.X != r.X && s.X != r.X+r.Width-1) || (s.Y != r.Y && s.Y != r.Y+r.Height-1) {
				return fmt.Errorf("stairs: stair %d is not at a corner of room %d", k, r.ID)
			}
			for _, dr := range d.Doors {
				if abs(dr.X-s.X)+abs(dr.Y-s.Y) <= 2 {
					return fmt.Errorf("stairs: room stair %d has door %d within 2 squares", k, dr.ID)
				}
			}
		}
	}
	e := d.Entrance
	if at(e.X, e.Y) == KindRock {
		return fmt.Errorf("entrance: (%d,%d) is not open", e.X, e.Y)
	}
	if len(d.Stairs) > 0 {
		if s := d.Stairs[0]; e.X != s.X || e.Y != s.Y || !e.OnStairs {
			return fmt.Errorf("entrance: not on the first stair")
		}
	} else {
		cx, cy := d.Rooms[0].Center()
		if e.X != cx || e.Y != cy || e.OnStairs || e.Room != d.Rooms[0].ID {
			return fmt.Errorf("entrance: not the center of room %d", d.Rooms[0].ID)
		}
	}
	// 5. Reachability from the entrance.
	seen := make([]bool, n)
	stack := []int{e.Y*w + e.X}
	seen[stack[0]] = true
	reached := 0
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		reached++
		x, y := u%w, u/w
		for dd := range 4 {
			if at(x+dx[dd], y+dy[dd]) != KindRock {
				v := (y+dy[dd])*w + x + dx[dd]
				if !seen[v] {
					seen[v] = true
					stack = append(stack, v)
				}
			}
		}
	}
	open := 0
	for _, k := range d.Kinds {
		if k != KindRock {
			open++
		}
	}
	if reached != open {
		return fmt.Errorf("reachability: %d of %d open squares reached from the entrance", reached, open)
	}
	// 11. Dead ends.
	if o.DeadendRemoval == 100 {
		for i, k := range d.Kinds {
			if k != KindCorridor || protected[i] {
				continue
			}
			deg := 0
			for dd := range 4 {
				if at(i%w+dx[dd], i/w+dy[dd]) != KindRock {
					deg++
				}
			}
			if deg == 1 {
				return fmt.Errorf("dead ends: square (%d,%d) remains with removal 100", i%w, i/w)
			}
		}
	}
	// Corridors table.
	for k, c := range d.Corridors {
		if c.ID != k+1 || d.CorridorIDs[c.FirstY*w+c.FirstX] != uint16(c.ID) { //nolint:gosec // G115: corridor IDs fit uint16 (at most ~20 000 nodes)
			return fmt.Errorf("corridors: bad corridor %d", c.ID)
		}
	}
	for i, k := range d.Kinds {
		if (k == KindCorridor) != (d.CorridorIDs[i] != 0) {
			return fmt.Errorf("corridors: square (%d,%d) and its corridor ID disagree", i%w, i/w)
		}
	}
	for k := 1; k < len(d.DiscardedRooms); k++ {
		if d.DiscardedRooms[k] <= d.DiscardedRooms[k-1] {
			return fmt.Errorf("rooms: discarded IDs not ascending")
		}
	}
	return nil
}
