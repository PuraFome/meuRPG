package dungeon

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The corruption corpus of check: generated dungeons, each broken one
// invariant at a time, with the exact error check returns recorded in
// testdata/check_golden.json. It freezes the text of every refusal and the
// order of the checks (the FIRST broken invariant is the one reported), so
// check can be split into small functions without changing what it says.
// "valid-" cases are untouched or consistently edited dungeons and must pass.

// Three branches of check cannot be reached by any dungeon, because an earlier
// invariant refuses first, and so have no case here: a square outside the grid
// read by the local `at` (the border ring is checked before anything reads
// beyond it), a corridor touching a room floor (the ring check of the room
// refuses it first) and a door that joins a room to itself (a room is a solid
// rectangle of floor).
const checkGoldenPath = "testdata/check_golden.json"

// chkBase names a family of generated dungeons a corruption starts from.
type chkBase string

const (
	baseFull       chkBase = "full"       // many rooms, direct doors, corridor stairs, dead ends removed.
	baseNoStairs   chkBase = "nostairs"   // no stairs: the entrance is the center of the first room.
	baseOneRoom    chkBase = "oneroom"    // a single room and no door.
	baseRoomStairs chkBase = "roomstairs" // a stair on a room corner (the fallback site).
	baseKeepEnds   chkBase = "keepends"   // dead ends kept (removal 0), so a corridor stub can be a stair.
)

// chkSeedLimit bounds the seed search for a base with a given property.
const chkSeedLimit = 3000

// chkBaseOptions finds, once, the options of a base: the first seed from 1
// whose dungeon satisfies the base's property.
func chkBaseOptions(t *testing.T, b chkBase) Options {
	t.Helper()
	mk := func(seed uint64) Options { return opts(seed, 51, 51) }
	var pred func(d *Dungeon) bool
	switch b {
	case baseFull:
		mk = func(seed uint64) Options {
			o := opts(seed, 51, 51)
			o.DeadendRemoval = 100
			return o
		}
		pred = func(d *Dungeon) bool {
			return len(d.Rooms) >= 5 && chkHasDirectDoor(d) && len(d.Stairs) == 2 &&
				d.Stairs[0].InRoom == 0 && d.Stairs[1].InRoom == 0 && len(chkSecondDoorSites(d)) > 0
		}
	case baseNoStairs:
		mk = func(seed uint64) Options {
			o := opts(seed, 51, 51)
			o.DeadendRemoval = 100
			o.Stairs = 0
			return o
		}
		pred = func(d *Dungeon) bool { return len(d.Rooms) >= 5 }
	case baseOneRoom:
		mk = func(seed uint64) Options {
			o := opts(seed, 15, 15)
			o.RoomSideMin, o.RoomSideMax = 13, 13
			o.Stairs = 0
			return o
		}
		pred = func(d *Dungeon) bool { return len(d.Rooms) == 1 }
	case baseRoomStairs:
		mk = func(seed uint64) Options {
			o := opts(seed, 15, 15)
			o.RoomSideMin, o.RoomSideMax = 13, 13
			o.Stairs = 3
			return o
		}
		pred = func(d *Dungeon) bool { return len(d.Stairs) > 0 && d.Stairs[0].InRoom != 0 }
	case baseKeepEnds:
		mk = func(seed uint64) Options {
			o := opts(seed, 51, 51)
			o.DeadendRemoval = 0
			return o
		}
		pred = func(d *Dungeon) bool { return len(d.Rooms) >= 5 && len(chkStubNextToDoor(d)) > 0 }
	}
	for seed := uint64(1); seed <= chkSeedLimit; seed++ {
		o := mk(seed)
		d, err := Generate(o)
		if err == nil && pred(d) {
			return o
		}
	}
	t.Fatalf("no seed up to %d gives a %q dungeon", chkSeedLimit, b)
	return Options{}
}

func chkHasDirectDoor(d *Dungeon) bool {
	for _, dr := range d.Doors {
		if dr.RoomB != 0 {
			return true
		}
	}
	return false
}

func (d *Dungeon) chkIdx(x, y int) int { return y*d.Width + x }

// chkDeg counts the open neighbors of a square.
func (d *Dungeon) chkDeg(x, y int) int {
	n := 0
	for dd := range 4 {
		if d.Kind(x+dx[dd], y+dy[dd]) != KindRock {
			n++
		}
	}
	return n
}

// chkStubNextToDoor lists corridor squares whose only open neighbor is a
// door (a one-square corridor stub).
func chkStubNextToDoor(d *Dungeon) []int {
	var out []int
	for i, k := range d.Kinds {
		x, y := i%d.Width, i/d.Width
		if k != KindCorridor || d.chkDeg(x, y) != 1 {
			continue
		}
		for dd := range 4 {
			if d.Kind(x+dx[dd], y+dy[dd]) == KindDoor {
				out = append(out, i)
			}
		}
	}
	return out
}

// chkSite is a wall square between two rooms where a second direct door
// could be put.
type chkSite struct{ x, y, a, b int }

// chkSecondDoorSites lists the rock wall squares between the floors of two
// rooms that already share a direct door, at least 4 squares from every door
// of both rooms on that side (the spacing check of the exits).
func chkSecondDoorSites(d *Dungeon) []chkSite {
	var out []chkSite
	for _, dr := range d.Doors {
		if dr.RoomB == 0 {
			continue
		}
		for y := 1; y < d.Height-1; y++ {
			for x := 1; x < d.Width-1; x++ {
				if x%2 == y%2 || d.Kind(x, y) != KindRock {
					continue
				}
				ax, ay := 0, 1
				if x%2 == 0 {
					ax, ay = 1, 0
				}
				ra, rb := int(d.RoomIDs[(y-ay)*d.Width+x-ax]), int(d.RoomIDs[(y+ay)*d.Width+x+ax])
				if ra != dr.RoomA || rb != dr.RoomB {
					continue
				}
				if chkFarFromDoors(d, x, y, ra) && chkFarFromDoors(d, x, y, rb) {
					out = append(out, chkSite{x, y, ra, rb})
				}
			}
		}
	}
	return out
}

func chkFarFromDoors(d *Dungeon, x, y, room int) bool {
	r, _ := d.Room(room)
	for _, e := range r.Exits {
		if abs(e.X-x)+abs(e.Y-y) < 4 {
			return false
		}
	}
	return true
}

func chkGenerate(t *testing.T, b chkBase, cache map[chkBase]Options) *Dungeon {
	t.Helper()
	o, ok := cache[b]
	if !ok {
		o = chkBaseOptions(t, b)
		cache[b] = o
	}
	d, err := Generate(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := check(d); err != nil {
		t.Fatalf("base %s is not valid: %v", b, err)
	}
	return d
}

// chkScan tries, square by square, a corruption and keeps the first one whose
// error starts with prefix. It leaves the dungeon corrupted and reports false
// when none fits. try must set the corruption and return an undo.
func chkScan(d *Dungeon, prefix string, try func(i int) (undo func(), ok bool)) bool {
	for i := range d.Kinds {
		undo, ok := try(i)
		if !ok {
			continue
		}
		if err := check(d); err != nil && strings.HasPrefix(err.Error(), prefix) {
			return true
		}
		undo()
	}
	return false
}

// chkSetKind returns an undo for a change of one square's kind.
func chkSetKind(d *Dungeon, i int, k Kind) func() {
	old := d.Kinds[i]
	d.Kinds[i] = k
	return func() { d.Kinds[i] = old }
}

// chkRoomWithTwoExits returns the index of a room with at least two exits.
func chkRoomWithTwoExits(d *Dungeon) int {
	for i, r := range d.Rooms {
		if len(r.Exits) >= 2 {
			return i
		}
	}
	return -1
}

// chkDirectExit returns the room and exit index of an exit into another room.
func chkDirectExit(d *Dungeon) (ri, ei int) {
	for i, r := range d.Rooms {
		for j, e := range r.Exits {
			if e.Other != 0 {
				return i, j
			}
		}
	}
	return -1, -1
}

// chkCorridorExit returns the room and exit index of an exit into a corridor.
func chkCorridorExit(d *Dungeon) (ri, ei int) {
	for i, r := range d.Rooms {
		for j, e := range r.Exits {
			if e.Other == 0 {
				return i, j
			}
		}
	}
	return -1, -1
}

// chkDoorIndex returns the index of the first door that is (corridor) or is
// not (direct) a door into a corridor.
func chkDoorIndex(d *Dungeon, direct bool) int {
	for i, dr := range d.Doors {
		if (dr.RoomB != 0) == direct {
			return i
		}
	}
	return -1
}

type chkCase struct {
	name string
	base chkBase
	// mut corrupts the dungeon and reports false when the base has nothing
	// to corrupt that way (a failure of the test, not a skipped case).
	mut func(d *Dungeon) bool
}

func chkCases() []chkCase {
	return []chkCase{
		// Untouched dungeons and consistent edits: the pass path.
		{"valid-full", baseFull, func(*Dungeon) bool { return true }},
		{"valid-nostairs", baseNoStairs, func(*Dungeon) bool { return true }},
		{"valid-oneroom", baseOneRoom, func(*Dungeon) bool { return true }},
		{"valid-roomstairs", baseRoomStairs, func(*Dungeon) bool { return true }},
		{"valid-keepends", baseKeepEnds, func(*Dungeon) bool { return true }},

		// 1. Bounds.
		{"bounds-width-differs-from-options", baseFull, func(d *Dungeon) bool { d.Width++; return true }},
		{"bounds-height-differs-from-options", baseFull, func(d *Dungeon) bool { d.Height--; return true }},
		{"bounds-grid-too-small", baseFull, func(d *Dungeon) bool {
			d.Width, d.Options.Width, d.Height, d.Options.Height = 13, 13, 13, 13
			return true
		}},
		{"bounds-even-width", baseFull, func(d *Dungeon) bool {
			d.Width, d.Options.Width = 50, 50
			return true
		}},
		{"bounds-even-height", baseFull, func(d *Dungeon) bool {
			d.Height, d.Options.Height = 50, 50
			return true
		}},
		{"bounds-kinds-length", baseFull, func(d *Dungeon) bool { d.Kinds = d.Kinds[:len(d.Kinds)-1]; return true }},
		{"bounds-roomids-length", baseFull, func(d *Dungeon) bool { d.RoomIDs = d.RoomIDs[1:]; return true }},
		{"bounds-mask-length", baseFull, func(d *Dungeon) bool { d.Mask = append(d.Mask, true); return true }},
		{"bounds-corridorids-length", baseFull, func(d *Dungeon) bool { d.CorridorIDs = nil; return true }},
		{"bounds-border-open-top", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(5, 0)] = KindCorridor
			return true
		}},
		{"bounds-border-open-bottom", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(7, d.Height-1)] = KindCorridor
			return true
		}},
		{"bounds-border-open-left", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(0, 9)] = KindCorridor
			return true
		}},
		{"bounds-border-open-right", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(d.Width-1, 11)] = KindCorridor
			return true
		}},

		// 4. Nothing open in a blocked square.
		{"blocked-open-square-outside-mask", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Mask[d.chkIdx(r.X, r.Y)] = false
			return true
		}},
		{"blocked-after-bounds-reports-bounds", baseFull, func(d *Dungeon) bool {
			d.Width++
			r := d.Rooms[0]
			d.Mask[d.chkIdx(r.X, r.Y)] = false
			return true
		}},
		{"rooms-none", baseFull, func(d *Dungeon) bool { d.Rooms = nil; return true }},
		{"rooms-none-after-blocked-reports-blocked", baseFull, func(d *Dungeon) bool {
			d.Rooms = nil
			r := d.Kinds
			for i, k := range r {
				if k == KindRoom {
					d.Mask[i] = false
					break
				}
			}
			return true
		}},

		// 2, 3. Rooms.
		{"rooms-ids-not-ascending", baseFull, func(d *Dungeon) bool { d.Rooms[1].ID = d.Rooms[0].ID; return true }},
		{"rooms-listed-and-discarded", baseFull, func(d *Dungeon) bool {
			d.DiscardedRooms = []int{d.Rooms[2].ID}
			return true
		}},
		{"parity-room-x-even", baseFull, func(d *Dungeon) bool { d.Rooms[0].X++; return true }},
		{"parity-room-y-even", baseFull, func(d *Dungeon) bool { d.Rooms[0].Y++; return true }},
		{"parity-room-width-even", baseFull, func(d *Dungeon) bool { d.Rooms[0].Width++; return true }},
		{"parity-room-height-even", baseFull, func(d *Dungeon) bool { d.Rooms[0].Height++; return true }},
		{"parity-side-under-min", baseFull, func(d *Dungeon) bool {
			d.Options.RoomSideMin = d.Rooms[0].Width + 2
			return true
		}},
		{"parity-side-over-max", baseFull, func(d *Dungeon) bool {
			d.Options.RoomSideMax = 1
			return true
		}},
		{"aspect-room-too-long", baseFull, func(d *Dungeon) bool {
			d.Options.AspectLimit = 1
			for _, r := range d.Rooms {
				if r.Width != r.Height {
					return true
				}
			}
			return false
		}},
		{"bounds-room-leaves-border", baseFull, func(d *Dungeon) bool { d.Rooms[0].X = d.Width - 2; return true }},
		{"overlap-room-floor-not-its-own", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.RoomIDs[d.chkIdx(r.X, r.Y)] = uint16(r.ID + 1) //nolint:gosec // G115: tiny room IDs.
			return true
		}},
		{"overlap-room-floor-not-room-kind", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Kinds[d.chkIdx(r.X+1, r.Y+1)] = KindRock
			return true
		}},
		{"blocked-room-ring-touches-blocked-square", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Mask[d.chkIdx(r.X-1, r.Y-1)] = false
			return true
		}},
		{"rooms-floor-square-without-room", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "rooms: floor square", func(i int) (func(), bool) {
				if d.Kinds[i] != KindRock || d.Mask[i] == false {
					return nil, false
				}
				return chkSetKind(d, i, KindRoom), true
			})
		}},
		{"rooms-non-floor-square-with-room-id", baseFull, func(d *Dungeon) bool {
			for i, k := range d.Kinds {
				if k == KindRock {
					d.RoomIDs[i] = 1
					return true
				}
			}
			return false
		}},
		{"overlap-floor-count-differs", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "overlap: ", func(i int) (func(), bool) {
				if d.Kinds[i] != KindRock || d.RoomIDs[i] != 0 {
					return nil, false
				}
				d.RoomIDs[i] = 1
				undo := chkSetKind(d, i, KindRoom)
				return func() { undo(); d.RoomIDs[i] = 0 }, true
			})
		}},
		{"corridor-on-the-ring-of-a-room", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			for x := r.X; x < r.X+r.Width; x++ {
				if d.Kind(x, r.Y-1) == KindRock {
					d.Kinds[d.chkIdx(x, r.Y-1)] = KindCorridor
					return true
				}
			}
			return false
		}},
		{"corridor-2x2-block", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "corridor: 2x2", func(i int) (func(), bool) {
				x, y := i%d.Width, i/d.Width
				if x+1 >= d.Width-1 || y+1 >= d.Height-1 {
					return nil, false
				}
				var undos []func()
				for _, q := range [4]int{i, i + 1, i + d.Width, i + d.Width + 1} {
					if d.Kinds[q] != KindRock {
						for _, u := range undos {
							u()
						}
						return nil, false
					}
					undos = append(undos, chkSetKind(d, q, KindCorridor))
				}
				return func() {
					for _, u := range undos {
						u()
					}
				}, true
			})
		}},

		// 6. Doors.
		{"doors-bad-id", baseFull, func(d *Dungeon) bool { d.Doors[1].ID = 9; return true }},
		{"doors-duplicate-id", baseFull, func(d *Dungeon) bool { d.Doors[1].ID = d.Doors[0].ID; return true }},
		{"doors-not-on-a-door-square", baseFull, func(d *Dungeon) bool { d.Doors[0].X = 0; return true }},
		{"doors-not-on-a-wall-line", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(2, 2)] = KindDoor
			d.Doors[0].X, d.Doors[0].Y = 2, 2
			return true
		}},
		{"doors-archway-trapped", baseFull, func(d *Dungeon) bool {
			d.Doors[0].Kind, d.Doors[0].Trapped = DoorArchway, true
			return true
		}},
		{"doors-barred-trapped", baseFull, func(d *Dungeon) bool {
			d.Doors[0].Kind, d.Doors[0].Trapped = DoorBarred, true
			return true
		}},
		{"doors-kind-out-of-range", baseFull, func(d *Dungeon) bool { d.Doors[0].Kind = 9; return true }},
		{"doors-axis-does-not-match", baseFull, func(d *Dungeon) bool {
			d.Doors[0].Axis = 1 - d.Doors[0].Axis
			return true
		}},
		{"doors-does-not-open-on-both-sides", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "doors: door", func(i int) (func(), bool) {
				if d.Kinds[i] != KindCorridor {
					return nil, false
				}
				return chkSetKind(d, i, KindRock), true
			})
		}},
		{"doors-no-rock-beside", baseFull, func(d *Dungeon) bool {
			for _, dr := range d.Doors {
				ax, ay := 0, 1
				if dr.Axis == VerticalWall {
					ax, ay = 1, 0
				}
				bx, by := dr.X-ay, dr.Y-ax
				if bx > 0 && by > 0 && bx < d.Width-1 && by < d.Height-1 {
					d.Kinds[d.chkIdx(bx, by)] = KindDoor
					return true
				}
			}
			return false
		}},
		{"doors-not-on-a-rooms-wall", baseFull, func(d *Dungeon) bool {
			// A fake door in the middle of a straight corridor.
			for i, k := range d.Kinds {
				x, y := i%d.Width, i/d.Width
				if k != KindCorridor || x%2 != 0 || y%2 != 1 {
					continue
				}
				if d.Kind(x-1, y) == KindCorridor && d.Kind(x+1, y) == KindCorridor && d.Kind(x, y-1) == KindRock && d.Kind(x, y+1) == KindRock {
					d.Kinds[i] = KindDoor
					d.Doors[0] = Door{ID: 1, X: x, Y: y, Kind: DoorClosed, Axis: VerticalWall}
					return true
				}
			}
			return false
		}},
		{"doors-direct-door-recorded-wrong-rooms", baseFull, func(d *Dungeon) bool {
			i := chkDoorIndex(d, true)
			d.Doors[i].RoomA, d.Doors[i].RoomB = d.Doors[i].RoomB, d.Doors[i].RoomA
			return true
		}},
		{"doors-corridor-door-recorded-a-second-room", baseFull, func(d *Dungeon) bool {
			i := chkDoorIndex(d, false)
			d.Doors[i].RoomB = 1
			return true
		}},

		// 12. Exits.
		{"exits-non-door", baseFull, func(d *Dungeon) bool { d.Rooms[0].Exits[0].X = 0; return true }},
		{"exits-door-id-zero", baseFull, func(d *Dungeon) bool { d.Rooms[0].Exits[0].DoorID = 0; return true }},
		{"exits-door-id-past-the-end", baseFull, func(d *Dungeon) bool {
			d.Rooms[0].Exits[0].DoorID = len(d.Doors) + 1
			return true
		}},
		{"exits-kind-differs", baseFull, func(d *Dungeon) bool {
			e := &d.Rooms[0].Exits[0]
			e.Kind = (e.Kind + 1) % (DoorSecret + 1)
			return true
		}},
		{"exits-trapped-differs", baseFull, func(d *Dungeon) bool {
			e := &d.Rooms[0].Exits[0]
			e.Trapped = !e.Trapped
			return true
		}},
		{"exits-position-differs", baseFull, func(d *Dungeon) bool {
			e := &d.Rooms[0].Exits[0]
			for _, dr := range d.Doors {
				if dr.ID != e.DoorID {
					e.X, e.Y = dr.X, dr.Y
					return true
				}
			}
			return false
		}},
		{"exits-door-of-another-room", baseFull, func(d *Dungeon) bool {
			r := &d.Rooms[0]
			for _, dr := range d.Doors {
				if dr.RoomA != r.ID && dr.RoomB != r.ID {
					r.Exits[0] = Exit{Side: r.Exits[0].Side, X: dr.X, Y: dr.Y, DoorID: dr.ID, Kind: dr.Kind, Trapped: dr.Trapped}
					return true
				}
			}
			return false
		}},
		{"exits-far-room-is-the-room-itself", baseFull, func(d *Dungeon) bool {
			ri, ei := chkDirectExit(d)
			d.Rooms[ri].Exits[ei].Other = d.Rooms[ri].ID
			return true
		}},
		{"exits-far-room-not-on-the-door", baseFull, func(d *Dungeon) bool {
			ri, ei := chkCorridorExit(d)
			d.Rooms[ri].Exits[ei].Other = 999
			return true
		}},
		{"doors-two-doors-too-close-on-a-side", baseFull, func(d *Dungeon) bool {
			d.Rooms[0].Exits = append(d.Rooms[0].Exits, d.Rooms[0].Exits[0])
			return true
		}},
		{"exits-room-has-none", baseFull, func(d *Dungeon) bool {
			d.Rooms[len(d.Rooms)-1].Exits = nil
			return true
		}},
		{"exits-count-differs-from-doors", baseFull, func(d *Dungeon) bool {
			i := chkRoomWithTwoExits(d)
			d.Rooms[i].Exits = d.Rooms[i].Exits[:1]
			return true
		}},
		{"doors-two-direct-doors-between-the-same-rooms", baseFull, func(d *Dungeon) bool {
			s := chkSecondDoorSites(d)[0]
			id := len(d.Doors) + 1
			ax, ay := 0, 1
			axis := HorizontalWall
			if s.x%2 == 0 {
				ax, ay = 1, 0
				axis = VerticalWall
			}
			_, _ = ax, ay
			d.Kinds[d.chkIdx(s.x, s.y)] = KindDoor
			d.Doors = append(d.Doors, Door{ID: id, X: s.x, Y: s.y, Kind: DoorArchway, Axis: axis, RoomA: s.a, RoomB: s.b})
			for k, rid := range [2]int{s.a, s.b} {
				i := rid - 1
				for j := range d.Rooms {
					if d.Rooms[j].ID == rid {
						i = j
					}
				}
				other := s.b
				if k == 1 {
					other = s.a
				}
				d.Rooms[i].Exits = append(d.Rooms[i].Exits, Exit{Side: chkSideOf(d.Rooms[i], s.x, s.y), X: s.x, Y: s.y, DoorID: id, Kind: DoorArchway, Other: other})
			}
			return true
		}},
		{"doors-square-without-an-entry", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "doors: ", func(i int) (func(), bool) {
				x, y := i%d.Width, i/d.Width
				if d.Kinds[i] != KindRock || x < 3 || y < 3 || x > d.Width-4 || y > d.Height-4 || d.chkDeg(x, y) != 0 {
					return nil, false
				}
				return chkSetKind(d, i, KindDoor), true
			})
		}},

		// 9, 10. Stairs and the entrance.
		{"stairs-placed-differs-from-listed", baseFull, func(d *Dungeon) bool { d.StairsPlaced++; return true }},
		{"stairs-more-than-wanted", baseFull, func(d *Dungeon) bool { d.Options.Stairs = 1; return true }},
		{"stairs-on-rock", baseFull, func(d *Dungeon) bool { d.Stairs[0].X, d.Stairs[0].Y = 0, 0; return true }},
		{"stairs-on-a-door", baseFull, func(d *Dungeon) bool {
			d.Stairs[0].X, d.Stairs[0].Y = d.Doors[0].X, d.Doors[0].Y
			return true
		}},
		{"stairs-too-close", baseFull, func(d *Dungeon) bool {
			d.Stairs[1].X, d.Stairs[1].Y = d.Stairs[0].X, d.Stairs[0].Y
			return true
		}},
		{"stairs-first-not-up", baseFull, func(d *Dungeon) bool { d.Stairs[0].Kind = StairDown; return true }},
		{"stairs-second-not-down", baseFull, func(d *Dungeon) bool { d.Stairs[1].Kind = StairUp; return true }},
		{"stairs-corridor-stair-in-a-room-square", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Stairs[0].X, d.Stairs[0].Y = r.X, r.Y
			return true
		}},
		{"stairs-corridor-stair-not-at-a-dead-end", baseFull, func(d *Dungeon) bool {
			for i, k := range d.Kinds {
				x, y := i%d.Width, i/d.Width
				if k == KindCorridor && d.chkDeg(x, y) == 2 && abs(x-d.Stairs[1].X)+abs(y-d.Stairs[1].Y) >= minStairGap {
					d.Stairs[0].X, d.Stairs[0].Y = x, y
					return true
				}
			}
			return false
		}},
		{"stairs-corridor-stair-next-to-a-door", baseKeepEnds, func(d *Dungeon) bool {
			i := chkStubNextToDoor(d)[0]
			d.Stairs[0].X, d.Stairs[0].Y = i%d.Width, i/d.Width
			return true
		}},
		{"stairs-corridor-stair-faces-the-wrong-way", baseFull, func(d *Dungeon) bool {
			d.Stairs[0].Facing = (d.Stairs[0].Facing + 1) % 4
			return true
		}},
		{"stairs-room-stair-in-an-unknown-room", baseRoomStairs, func(d *Dungeon) bool { d.Stairs[0].InRoom = 99; return true }},
		{"stairs-room-stair-on-the-floor-of-another-room", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Stairs[0].X, d.Stairs[0].Y, d.Stairs[0].InRoom = r.X, r.Y, d.Rooms[1].ID
			return true
		}},
		{"stairs-room-stair-on-a-corridor-square", baseFull, func(d *Dungeon) bool {
			d.Stairs[0].InRoom = d.Rooms[0].ID
			return true
		}},
		{"stairs-room-stair-not-at-a-corner", baseRoomStairs, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Stairs[0].X, d.Stairs[0].Y = r.X+1, r.Y+1
			return true
		}},
		{"stairs-room-stair-near-a-door", baseFull, func(d *Dungeon) bool {
			// A stair moved to a room corner within 2 squares of a door.
			for _, r := range d.Rooms {
				for _, c := range [4][2]int{{r.X, r.Y}, {r.X + r.Width - 1, r.Y}, {r.X, r.Y + r.Height - 1}, {r.X + r.Width - 1, r.Y + r.Height - 1}} {
					for _, dr := range d.Doors {
						if abs(dr.X-c[0])+abs(dr.Y-c[1]) <= 2 {
							d.Stairs[0].X, d.Stairs[0].Y, d.Stairs[0].InRoom = c[0], c[1], r.ID
							return true
						}
					}
				}
			}
			return false
		}},
		{"stairs-moved-to-a-free-corner-leaves-a-dead-end", baseFull, func(d *Dungeon) bool {
			for _, r := range d.Rooms {
				c := [2]int{r.X, r.Y}
				free := true
				for _, dr := range d.Doors {
					if abs(dr.X-c[0])+abs(dr.Y-c[1]) <= 2 {
						free = false
					}
				}
				if free && abs(c[0]-d.Stairs[1].X)+abs(c[1]-d.Stairs[1].Y) >= minStairGap {
					d.Stairs[0].X, d.Stairs[0].Y, d.Stairs[0].InRoom = c[0], c[1], r.ID
					d.Entrance.X, d.Entrance.Y = c[0], c[1]
					return true
				}
			}
			return false
		}},
		{"entrance-not-open", baseFull, func(d *Dungeon) bool { d.Entrance.X, d.Entrance.Y = 0, 0; return true }},
		{"entrance-not-on-the-first-stair", baseFull, func(d *Dungeon) bool { d.Entrance.OnStairs = false; return true }},
		{"entrance-moved-off-the-first-stair", baseFull, func(d *Dungeon) bool {
			r := d.Rooms[0]
			d.Entrance.X, d.Entrance.Y = r.X, r.Y
			return true
		}},
		{"entrance-not-the-center-of-the-first-room", baseNoStairs, func(d *Dungeon) bool { d.Entrance.X++; return true }},
		{"entrance-flagged-on-stairs-without-stairs", baseNoStairs, func(d *Dungeon) bool { d.Entrance.OnStairs = true; return true }},
		{"entrance-wrong-room", baseNoStairs, func(d *Dungeon) bool { d.Entrance.Room = 99; return true }},

		// 5. Reachability, 11. Dead ends.
		{"reachability-a-corridor-is-cut", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "reachability", func(i int) (func(), bool) {
				if d.Kinds[i] != KindCorridor {
					return nil, false
				}
				return chkSetKind(d, i, KindRock), true
			})
		}},
		{"reachability-an-island", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "reachability", func(i int) (func(), bool) {
				x, y := i%d.Width, i/d.Width
				if d.Kinds[i] != KindRock || x < 3 || y < 3 || x > d.Width-4 || y > d.Height-4 || d.chkDeg(x, y) != 0 {
					return nil, false
				}
				return chkSetKind(d, i, KindCorridor), true
			})
		}},
		{"dead-ends-a-stub-remains-with-removal-100", baseFull, func(d *Dungeon) bool {
			return chkScan(d, "dead ends", func(i int) (func(), bool) {
				x, y := i%d.Width, i/d.Width
				if d.Kinds[i] != KindRock || x < 2 || y < 2 || x > d.Width-3 || y > d.Height-3 {
					return nil, false
				}
				return chkSetKind(d, i, KindCorridor), true
			})
		}},

		// Corridors table.
		{"corridors-bad-id", baseFull, func(d *Dungeon) bool { d.Corridors[0].ID = 99; return true }},
		{"corridors-first-square-has-another-id", baseFull, func(d *Dungeon) bool {
			c := d.Corridors[0]
			d.CorridorIDs[d.chkIdx(c.FirstX, c.FirstY)] = 0
			return true
		}},
		{"corridors-square-and-id-disagree", baseFull, func(d *Dungeon) bool {
			for i, k := range d.Kinds {
				if k == KindRock {
					d.CorridorIDs[i] = 1
					return true
				}
			}
			return false
		}},
		{"corridors-corridor-square-without-id", baseFull, func(d *Dungeon) bool {
			first := map[int]bool{}
			for _, c := range d.Corridors {
				first[d.chkIdx(c.FirstX, c.FirstY)] = true
			}
			for i, k := range d.Kinds {
				if k == KindCorridor && !first[i] {
					d.CorridorIDs[i] = 0
					return true
				}
			}
			return false
		}},
		{"rooms-discarded-ids-not-ascending", baseFull, func(d *Dungeon) bool {
			d.DiscardedRooms = []int{1000, 999}
			return true
		}},
		{"rooms-discarded-ids-repeat", baseFull, func(d *Dungeon) bool {
			d.DiscardedRooms = []int{1000, 1000}
			return true
		}},
		{"valid-discarded-ids-ascending", baseFull, func(d *Dungeon) bool {
			d.DiscardedRooms = []int{998, 999, 1000}
			return true
		}},

		// Order: the first broken invariant is the one reported.
		{"order-bounds-before-border", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(5, 0)] = KindCorridor
			d.Width++
			return true
		}},
		{"order-border-before-blocked", baseFull, func(d *Dungeon) bool {
			d.Kinds[d.chkIdx(5, 0)] = KindCorridor
			d.Mask[d.chkIdx(5, 0)] = false
			return true
		}},
		{"order-rooms-before-doors", baseFull, func(d *Dungeon) bool {
			d.Rooms[0].X++
			d.Doors[0].X = 0
			return true
		}},
		{"order-doors-before-exits", baseFull, func(d *Dungeon) bool {
			d.Doors[0].Kind = 9
			d.Rooms[0].Exits[0].X = 0
			return true
		}},
		{"order-exits-before-stairs", baseFull, func(d *Dungeon) bool {
			d.Rooms[len(d.Rooms)-1].Exits = nil
			d.Stairs[0].Kind = StairDown
			return true
		}},
		{"order-stairs-before-entrance", baseFull, func(d *Dungeon) bool {
			d.StairsPlaced++
			d.Entrance.OnStairs = false
			return true
		}},
		{"order-entrance-before-reachability", baseFull, func(d *Dungeon) bool {
			d.Entrance.OnStairs = false
			return chkScan(d, "entrance", func(i int) (func(), bool) {
				x, y := i%d.Width, i/d.Width
				if d.Kinds[i] != KindRock || x < 3 || y < 3 || x > d.Width-4 || y > d.Height-4 || d.chkDeg(x, y) != 0 {
					return nil, false
				}
				return chkSetKind(d, i, KindCorridor), true
			})
		}},
		{"order-corridor-table-last", baseFull, func(d *Dungeon) bool {
			d.Corridors[0].ID = 99
			d.DiscardedRooms = []int{1000, 999}
			return true
		}},
	}
}

// chkSideOf returns the side of a room's ring a wall square sits on.
func chkSideOf(r Room, x, y int) Side {
	switch {
	case y < r.Y:
		return North
	case x >= r.X+r.Width:
		return East
	case y >= r.Y+r.Height:
		return South
	}
	return West
}

func TestCheckCorruptionCorpus(t *testing.T) {
	cache := map[chkBase]Options{}
	got := map[string]string{}
	for _, c := range chkCases() {
		d := chkGenerate(t, c.base, cache)
		if !c.mut(d) {
			t.Fatalf("%s: the %s base has nothing to corrupt that way", c.name, c.base)
		}
		msg := ""
		if err := check(d); err != nil {
			msg = err.Error()
		}
		if strings.HasPrefix(c.name, "valid-") != (msg == "") {
			t.Errorf("%s: error %q, but a valid- case must pass and the others must fail", c.name, msg)
		}
		got[c.name] = msg
	}
	if *update {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(checkGoldenPath, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(checkGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for name, msg := range got {
		if w, ok := want[name]; !ok || w != msg {
			t.Errorf("%s:\n got  %q\n want %q", name, msg, w)
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("golden has %q, the corpus does not", name)
		}
	}
}

// BenchmarkCheck measures the invariants over a big dungeon, so a split of
// check can show it did not get slower.
func BenchmarkCheck(b *testing.B) {
	d, err := Generate(opts(2026, 199, 399))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := check(d); err != nil {
			b.Fatal(err)
		}
	}
}
