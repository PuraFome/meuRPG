package vision_test

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// A lit room behind a door: a 12 x 5 map with a wall across column 5 and a door
// in it at (5, 2). The viewer stands in the dark corridor to the west with
// darkvision of 60 ft (a dark room is seen in grey, so what the door hides is
// what is tested); the torch is in the room at (8, 2).
func doorScene(door grid.Door) vision.Scene {
	g := grid.Grid{Columns: 12, Rows: 5}
	walls := grid.NewLayer(g)
	for row := range g.Rows {
		walls.Set(5, row, true)
	}
	walls.Set(5, 2, false) // the doorway
	doors := grid.NewDoorLayer(g)
	doors.Set(5, 2, door)
	return vision.Scene{
		Grid: g, Walls: walls, Doors: doors, Base: grid.Dark,
		Sources: []vision.Source{{At: grid.Square{Col: 8, Row: 2}, BrightFt: 20, DimFt: 20}},
	}
}

var (
	inCorridor = vision.Viewer{At: grid.Square{Col: 2, Row: 2}, Senses: vision.Senses{DarkvisionFt: 60}}
	inRoom     = grid.Square{Col: 8, Row: 2}
)

func TestADoorHidesWhatIsBehindIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		door       grid.Door
		seesRoom   bool
		roomIsLit  bool // the torch's light reaches the corridor side
		doorIsWall bool
	}{
		{grid.DoorNone, true, true, false},
		{grid.DoorOpen, true, true, false},
		{grid.DoorBarred, true, true, false}, // bars: sight and light pass
		{grid.DoorClosed, false, false, true},
		{grid.DoorLocked, false, false, true},
		{grid.DoorSecret, false, false, true},
	}
	for _, c := range cases {
		lit, err := vision.Compile(doorScene(c.door))
		if err != nil {
			t.Fatalf("door %d: %v", c.door, err)
		}
		v := lit.See(inCorridor)
		if got := v.Seen(inRoom) && v.At(inRoom) == vision.SeenBright; got != c.seesRoom {
			t.Errorf("door %d: sees the lit room = %v, want %v", c.door, got, c.seesRoom)
		}
		// The light stops at a closed door: the square on the corridor's side of
		// it gets none of the torch (it is 3 squares from it, in its bright radius).
		west := grid.Square{Col: 4, Row: 2}
		if got := lit.LightAt(west) > grid.Dark; got != c.roomIsLit {
			t.Errorf("door %d: the corridor side is lit = %v, want %v", c.door, got, c.roomIsLit)
		}
		if got := v.At(grid.Square{Col: 5, Row: 2}) == vision.SeenWall; got != c.doorIsWall {
			t.Errorf("door %d: the door square reads as a wall = %v, want %v", c.door, got, c.doorIsWall)
		}
	}
}

func TestOpeningADoorShowsTheRoom(t *testing.T) {
	t.Parallel()
	closed, err := vision.Compile(doorScene(grid.DoorClosed))
	if err != nil {
		t.Fatal(err)
	}
	open, err := vision.Compile(doorScene(grid.DoorOpen))
	if err != nil {
		t.Fatal(err)
	}
	if closed.See(inCorridor).Seen(inRoom) {
		t.Error("a closed door shows the room")
	}
	if !open.See(inCorridor).Seen(inRoom) {
		t.Error("an open door hides the room")
	}
}

func TestDoorsOfAnotherGridAreRefused(t *testing.T) {
	t.Parallel()
	s := doorScene(grid.DoorClosed)
	s.Doors = grid.NewDoorLayer(grid.Grid{Columns: 3, Rows: 3})
	if _, err := vision.Compile(s); err == nil {
		t.Error("Compile accepted doors sized for another grid")
	}
}
