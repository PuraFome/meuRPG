package grid_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

func TestDoorLayerByteLayout(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 5, Rows: 3} // 15 squares: 8 bytes, the last holds one
	l := grid.NewDoorLayer(g)
	if got := len(l.Encode()); got != grid.DoorLayerSize(g) || got != 8 {
		t.Fatalf("size = %d, want 8", got)
	}
	// Square 0 is the low nibble of byte 0, square 1 the high one.
	l.Set(0, 0, grid.DoorClosed)
	l.Set(1, 0, grid.DoorSecret)
	l.Set(4, 2, grid.DoorBarred) // square 14: the low nibble of byte 7
	b := l.Encode()
	if b[0] != 0x52 || b[7] != 0x04 {
		t.Errorf("bytes = % x, want byte 0 = 0x52 and byte 7 = 0x04", b)
	}
	if l.Count() != 3 {
		t.Errorf("Count = %d, want 3", l.Count())
	}
	back, err := grid.DecodeDoorLayer(g, b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, c := range []struct {
		col, row int
		want     grid.Door
	}{{0, 0, grid.DoorClosed}, {1, 0, grid.DoorSecret}, {4, 2, grid.DoorBarred}, {2, 1, grid.DoorNone}} {
		if got := back.Get(c.col, c.row); got != c.want {
			t.Errorf("Get(%d, %d) = %d, want %d", c.col, c.row, got, c.want)
		}
	}
	// Painting back to none clears it, a value that is no state and a square
	// outside the grid change nothing.
	l.Set(0, 0, grid.DoorNone)
	if l.Set(0, 0, 6) || l.Set(5, 0, grid.DoorOpen) || l.Set(0, -1, grid.DoorOpen) || l.Get(0, 0) != grid.DoorNone {
		t.Error("a bad value or a square outside the grid was written")
	}
	// Every state survives the round trip, in every position of a byte.
	all := grid.NewDoorLayer(grid.Grid{Columns: 6, Rows: 1})
	for col := 0; col < 6; col++ {
		all.Set(col, 0, grid.Door(col))
	}
	again, err := grid.DecodeDoorLayer(grid.Grid{Columns: 6, Rows: 1}, all.Encode())
	if err != nil || !slices.Equal(again.Encode(), all.Encode()) {
		t.Errorf("round trip: %v", err)
	}
	var nilLayer *grid.DoorLayer
	if nilLayer.Get(0, 0) != grid.DoorNone || nilLayer.Encode() != nil || nilLayer.Count() != 0 || nilLayer.Set(0, 0, grid.DoorOpen) {
		t.Error("a nil layer reads as no door and refuses writes")
	}
}

func TestDecodeDoorLayerRefusesBadBytes(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 5, Rows: 3}
	good := grid.NewDoorLayer(g).Encode()
	cases := map[string]func([]byte) []byte{
		"too short":          func(b []byte) []byte { return b[:len(b)-1] },
		"too long":           func(b []byte) []byte { return append(b, 0) },
		"an unused nibble":   func(b []byte) []byte { b[7] = 0x10; return b },
		"a value above five": func(b []byte) []byte { b[0] = 0x06; return b },
		"a value of fifteen": func(b []byte) []byte { b[3] = 0xf0; return b },
	}
	for name, mutate := range cases {
		if _, err := grid.DecodeDoorLayer(g, mutate(slices.Clone(good))); !errors.Is(err, grid.ErrLayerSize) {
			t.Errorf("%s: error = %v, want ErrLayerSize", name, err)
		}
	}
	if _, err := grid.DecodeDoorLayer(grid.Grid{}, nil); !errors.Is(err, grid.ErrLayerSize) {
		t.Errorf("an invalid grid: error = %v", err)
	}
}

// corridor is a 9 x 3 map: a corridor along row 1 between walls, with a door at
// (4, 1). Walls above and below, and at the two ends.
func corridor(door grid.Door) grid.Terrain {
	g := grid.Grid{Columns: 9, Rows: 3}
	t := grid.Terrain{Grid: g, Walls: grid.NewLayer(g), Doors: grid.NewDoorLayer(g)}
	for col := 0; col < 9; col++ {
		t.Walls.Set(col, 0, true)
		t.Walls.Set(col, 2, true)
	}
	t.Doors.Set(4, 1, door)
	return t
}

var doorSq = grid.Square{Col: 4, Row: 1}

func TestDoorsOnAStraightMove(t *testing.T) {
	t.Parallel()
	from, to := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}
	mover := grid.Mover{Size: grid.SizeMedium}
	cases := []struct {
		door    grid.Door
		blocked bool
		by      grid.StopReason
		opens   []grid.Square
	}{
		{grid.DoorNone, false, "", nil},
		{grid.DoorOpen, false, "", nil},
		{grid.DoorClosed, false, "", []grid.Square{doorSq}}, // passable for the planner, and reported
		{grid.DoorLocked, true, grid.StopLocked, nil},
		{grid.DoorBarred, true, grid.StopWall, nil},
		{grid.DoorSecret, true, grid.StopWall, nil},
	}
	for _, c := range cases {
		mv := corridor(c.door).Move(from, to, nil, mover)
		if mv.Blocked != c.blocked || mv.By != c.by || !slices.Equal(mv.Opens, c.opens) {
			t.Errorf("door %d: Blocked %v By %q Opens %v, want %v %q %v", c.door, mv.Blocked, mv.By, mv.Opens, c.blocked, c.by, c.opens)
		}
		if !c.blocked && mv.CostDFt != 6*grid.DFtPerSquare {
			t.Errorf("door %d: cost %d, want 300 (a door costs nothing extra)", c.door, mv.CostDFt)
		}
	}
	// A door that is the destination opens too.
	if mv := corridor(grid.DoorClosed).Move(from, doorSq, nil, mover); mv.Blocked || !slices.Equal(mv.Opens, []grid.Square{doorSq}) {
		t.Errorf("a move to a closed door: %+v", mv)
	}
	// A jumper cannot open a door on the way: only an open door (or none) is cleared.
	for door, wantBlocked := range map[grid.Door]bool{grid.DoorNone: false, grid.DoorOpen: false, grid.DoorClosed: true, grid.DoorLocked: true, grid.DoorBarred: true, grid.DoorSecret: true} {
		if got := corridor(door).Jump(from, to, nil, mover).Blocked; got != wantBlocked {
			t.Errorf("jump over door %d: blocked = %v, want %v", door, got, wantBlocked)
		}
	}
}

func TestMoveUntilAtDoors(t *testing.T) {
	t.Parallel()
	from, to := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}
	mover := grid.Mover{Size: grid.SizeMedium}
	all := 1 << 30
	cases := []struct {
		name    string
		door    grid.Door
		left    int
		reached grid.Square
		reason  grid.StopReason
		cost    int
		opens   []grid.Square
	}{
		{"closed: the move opens it and goes on", grid.DoorClosed, all, to, grid.StopNone, 300, []grid.Square{doorSq}},
		{"open: nothing happens", grid.DoorOpen, all, to, grid.StopNone, 300, nil},
		{"locked: it stops before, having walked what it walked", grid.DoorLocked, all, grid.Square{Col: 3, Row: 1}, grid.StopLocked, 100, nil},
		{"barred: it stops before", grid.DoorBarred, all, grid.Square{Col: 3, Row: 1}, grid.StopWall, 100, nil},
		{"secret: a wall", grid.DoorSecret, all, grid.Square{Col: 3, Row: 1}, grid.StopWall, 100, nil},
		{"closed, with too little movement: it never gets there, so it never opens it", grid.DoorClosed, 120, grid.Square{Col: 3, Row: 1}, grid.StopMovement, 100, nil},
		{"closed, with just enough movement to enter it", grid.DoorClosed, 150, doorSq, grid.StopMovement, 150, []grid.Square{doorSq}},
	}
	for _, c := range cases {
		st := corridor(c.door).MoveUntil(from, to, nil, mover, nil, c.left)
		if st.Reached != c.reached || st.Reason != c.reason || st.CostDFt != c.cost || !slices.Equal(st.Opens, c.opens) {
			t.Errorf("%s: reached %v reason %q cost %d opens %v; want %v %q %d %v", c.name, st.Reached, st.Reason, st.CostDFt, st.Opens, c.reached, c.reason, c.cost, c.opens)
		}
		if st.Reason == grid.StopLocked && st.LockedAt != doorSq {
			t.Errorf("%s: LockedAt = %v, want %v", c.name, st.LockedAt, doorSq)
		}
	}
	// The locked door is the very first square: no step is taken.
	st := corridor(grid.DoorLocked).MoveUntil(grid.Square{Col: 3, Row: 1}, to, nil, mover, nil, all)
	if st.Reached != (grid.Square{Col: 3, Row: 1}) || st.Reason != grid.StopLocked || len(st.Entered) != 0 || st.CostDFt != 0 {
		t.Errorf("a locked door right ahead: %+v", st)
	}
	// A door painted on a square that is also a wall is a wall, not a lock.
	t2 := corridor(grid.DoorLocked)
	t2.Walls.Set(4, 1, true)
	if st := t2.MoveUntil(from, to, nil, mover, nil, all); st.Reason != grid.StopWall {
		t.Errorf("a locked door in a wall: reason %q, want wall", st.Reason)
	}
}

func TestReachThroughDoors(t *testing.T) {
	t.Parallel()
	from := grid.Square{Col: 1, Row: 1}
	mover := grid.Mover{Size: grid.SizeMedium}
	reaches := func(door grid.Door, sq grid.Square) bool {
		for _, r := range corridor(door).Reach(from, 400, nil, mover) {
			if r.Square == sq {
				return true
			}
		}
		return false
	}
	beyond := grid.Square{Col: 6, Row: 1}
	for door, want := range map[grid.Door]bool{grid.DoorNone: true, grid.DoorOpen: true, grid.DoorClosed: true, grid.DoorLocked: false, grid.DoorBarred: false, grid.DoorSecret: false} {
		if got := reaches(door, beyond); got != want {
			t.Errorf("door %d: reaches the room behind = %v, want %v", door, got, want)
		}
	}
	// The door's own square is reachable when it opens by walking into it.
	if !reaches(grid.DoorClosed, doorSq) || reaches(grid.DoorLocked, doorSq) {
		t.Error("a closed door can be entered, a locked one cannot")
	}
}

func TestSightAndLightThroughDoors(t *testing.T) {
	t.Parallel()
	a, b := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}
	for door, wantClear := range map[grid.Door]bool{grid.DoorNone: true, grid.DoorOpen: true, grid.DoorBarred: true, grid.DoorClosed: false, grid.DoorLocked: false, grid.DoorSecret: false} {
		tr := corridor(door)
		sight, err := grid.NewSight(tr.Grid, tr.SightWalls())
		if err != nil {
			t.Fatal(err)
		}
		if got := sight.Clear(a, b); got != wantClear {
			t.Errorf("door %d: the line is clear = %v, want %v", door, got, wantClear)
		}
	}
	// SightWalls does not touch the walls it was given.
	tr := corridor(grid.DoorClosed)
	before := tr.Walls.Count()
	if got := tr.SightWalls().Count(); got != before+1 || tr.Walls.Count() != before {
		t.Errorf("SightWalls counts %d (walls %d)", got, tr.Walls.Count())
	}
	if corridor(grid.DoorOpen).SightWalls() == nil {
		t.Error("with no blocking door, the walls themselves come back")
	}
}

func TestCoverFromDoors(t *testing.T) {
	t.Parallel()
	a, b := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}
	for door, want := range map[grid.Door]grid.Cover{
		grid.DoorNone: grid.CoverNone, grid.DoorOpen: grid.CoverNone, grid.DoorBarred: grid.CoverNone, // bars give no cover
		grid.DoorClosed: grid.CoverTotal, grid.DoorLocked: grid.CoverTotal, grid.DoorSecret: grid.CoverTotal,
	} {
		if got := corridor(door).CoverBetween(a, b, nil); got != want {
			t.Errorf("door %d: cover %d, want %d", door, got, want)
		}
	}
}

func TestDoorsAsAPlayerKnowsThem(t *testing.T) {
	t.Parallel()
	for door, want := range map[grid.Door]struct {
		door grid.Door
		wall bool
	}{
		grid.DoorNone: {grid.DoorNone, false}, grid.DoorOpen: {grid.DoorOpen, false}, grid.DoorClosed: {grid.DoorClosed, false},
		grid.DoorLocked: {grid.DoorClosed, false}, grid.DoorBarred: {grid.DoorBarred, false}, grid.DoorSecret: {grid.DoorNone, true},
	} {
		if d, w := door.AsPlayerKnows(); d != want.door || w != want.wall {
			t.Errorf("door %d: AsPlayerKnows = %d, %v; want %d, %v", door, d, w, want.door, want.wall)
		}
	}
	// A player plans a move through a locked door (it looks closed: they will try
	// it) and never through a secret one (a wall to them); the real terrain is
	// not changed.
	from, to := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}
	mover := grid.Mover{Size: grid.SizeMedium}
	locked := corridor(grid.DoorLocked)
	if mv := locked.ForPlayers().Move(from, to, nil, mover); mv.Blocked || !slices.Equal(mv.Opens, []grid.Square{doorSq}) {
		t.Errorf("the planning through a locked door: %+v", mv)
	}
	if !locked.Move(from, to, nil, mover).Blocked || locked.Doors.Get(4, 1) != grid.DoorLocked {
		t.Error("ForPlayers changed the real terrain")
	}
	secret := corridor(grid.DoorSecret)
	known := secret.ForPlayers()
	if !known.Move(from, to, nil, mover).Blocked || !known.Walls.Get(4, 1) || known.Doors.Get(4, 1) != grid.DoorNone {
		t.Error("a secret door is a wall to the player, and no door")
	}
	if secret.Walls.Get(4, 1) {
		t.Error("ForPlayers wrote a wall into the real terrain")
	}
}

func TestTerrainValidateChecksTheDoors(t *testing.T) {
	t.Parallel()
	tr := corridor(grid.DoorClosed)
	if err := tr.Validate(); err != nil {
		t.Fatal(err)
	}
	tr.Doors = grid.NewDoorLayer(grid.Grid{Columns: 4, Rows: 4})
	if err := tr.Validate(); !errors.Is(err, grid.ErrBadGrid) {
		t.Errorf("doors of another grid: %v", err)
	}
}

// A move that slips between two things that close a corner is blocked, as the
// line of sight is (RN-26): a closed door counts on a side of the corner like a
// wall, an open door does not.
func TestADoorClosesACornerLikeAWall(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 4, Rows: 4}
	build := func(wall bool, a, b grid.Door) grid.Terrain {
		tr := grid.Terrain{Grid: g, Walls: grid.NewLayer(g), Doors: grid.NewDoorLayer(g)}
		if wall {
			tr.Walls.Set(2, 1, true)
		}
		tr.Doors.Set(2, 1, a) // one side of the corner from (1, 1) to (2, 2)
		tr.Doors.Set(1, 2, b) // the other
		return tr
	}
	from, to := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 2, Row: 2}
	cases := []struct {
		name    string
		wall    bool
		a, b    grid.Door
		blocked bool
	}{
		{"a wall and a closed door", true, grid.DoorNone, grid.DoorClosed, true},
		{"two closed doors", false, grid.DoorClosed, grid.DoorClosed, true},
		{"a wall and a locked door", true, grid.DoorNone, grid.DoorLocked, true},
		{"a wall and a barred door", true, grid.DoorNone, grid.DoorBarred, true},
		{"a wall and a secret door", true, grid.DoorNone, grid.DoorSecret, true},
		{"a wall and an open door", true, grid.DoorNone, grid.DoorOpen, false},
		{"one closed door only", false, grid.DoorNone, grid.DoorClosed, false},
	}
	for _, c := range cases {
		tr := build(c.wall, c.a, c.b)
		got := tr.Move(from, to, nil, grid.Mover{Size: grid.SizeMedium}).Blocked
		if got != c.blocked {
			t.Errorf("%s: Move blocked = %v, want %v", c.name, got, c.blocked)
		}
		if st := tr.MoveUntil(from, to, nil, grid.Mover{Size: grid.SizeMedium}, nil, 1000); (st.Reached != to) != c.blocked {
			t.Errorf("%s: MoveUntil reached %v, want blocked = %v", c.name, st.Reached, c.blocked)
		}
		if sight, err := grid.NewSight(g, tr.SightWalls()); err != nil || sight.Clear(from, to) == c.blocked && c.name != "a wall and a barred door" {
			t.Errorf("%s: the line of sight agrees with the move? clear = %v (%v)", c.name, sight.Clear(from, to), err)
		}
	}
}

// A creature that cannot end its move before a locked door does not hide the lock:
// the move still says it was the door.
func TestAnAllyBeforeALockedDoorKeepsTheLockAsTheReason(t *testing.T) {
	t.Parallel()
	occ := grid.OccupantMap{{Col: 3, Row: 1}: {Size: grid.SizeMedium}} // an ally, just before the door at (4, 1)
	st := corridor(grid.DoorLocked).MoveUntil(grid.Square{Col: 1, Row: 1}, grid.Square{Col: 7, Row: 1}, occ, grid.Mover{Size: grid.SizeMedium}, nil, 1<<30)
	if st.Reason != grid.StopLocked || st.Reached != (grid.Square{Col: 2, Row: 1}) {
		t.Errorf("reason %q reached %v; want locked, backed up to (2, 1)", st.Reason, st.Reached)
	}
}

func TestDoorsOfAnotherGridFailClosedForPlayers(t *testing.T) {
	t.Parallel()
	walls := grid.NewLayer(grid.Grid{Columns: 9, Rows: 3})
	doors := grid.NewDoorLayer(grid.Grid{Columns: 4, Rows: 4})
	doors.Set(1, 1, grid.DoorLocked)
	w, d := doors.ForPlayers(walls)
	if d != nil || w != walls {
		t.Errorf("ForPlayers on a mismatch gave doors %v and walls %v, want no doors", d, w)
	}
}
