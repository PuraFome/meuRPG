package grid_test

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The dungeon of the Etapa 10 artboards (E10-05), 31 x 21 squares. # rock, . a
// room, , a corridor, o an opening (a doorway, no door), D a closed door, T a
// closed door with a trap, L a locked door, B a barred one (a portcullis), S a
// secret door, < the entrance, > the stairs down. The expected rooms come from
// the artboards' data (dungeon.txt): with every door open all nine are
// reachable; for the players, closed and trapped doors open by walking and
// locked, barred and secret ones block, so only room 9 stays out of reach.
var dungeonRows = []string{
	"###############################",
	"#.......###.......###.........#",
	"#.<.....#,D.......D,L.........#",
	"#.......o,#.......###.........#",
	"#.......####T########.........#",
	"#.......####,########.........#",
	"####D#######,########.........#",
	"####,#######,########.........#",
	"####o#######D########D###B#####",
	"#.....###.......#####,###,#####",
	"#.....###.......#####o###,#####",
	"#.....D,o.......o,D.....#,#...#",
	"#.....###.......###.....#,#...#",
	"#.....###.......###.....o,D...#",
	"###D###############.....###...#",
	"###,###############.....###...#",
	"###o#######################...#",
	"###.......###.....#########...#",
	"###.......o,S.....#########...#",
	"###.....>.###.....#########...#",
	"###############################",
}

// dungeonRooms are the floors of the nine rooms: x, y, w, h.
var dungeonRooms = [][4]int{
	{1, 1, 7, 5},
	{11, 1, 7, 3},
	{21, 1, 9, 7},
	{1, 9, 5, 5},
	{9, 9, 7, 5},
	{19, 11, 5, 5},
	{27, 11, 3, 9},
	{3, 17, 7, 3},
	{13, 17, 5, 3},
}

func dungeon(open bool) grid.Terrain {
	g := grid.Grid{Columns: 31, Rows: 21}
	t := grid.Terrain{Grid: g, Walls: grid.NewLayer(g), Doors: grid.NewDoorLayer(g)}
	for row, line := range dungeonRows {
		for col, ch := range line {
			door := grid.DoorNone
			switch ch {
			case '#':
				t.Walls.Set(col, row, true)
			case 'o':
				door = grid.DoorOpen
			case 'D', 'T':
				door = grid.DoorClosed
			case 'L':
				door = grid.DoorLocked
			case 'B':
				door = grid.DoorBarred
			case 'S':
				door = grid.DoorSecret
			}
			if open && door != grid.DoorNone {
				door = grid.DoorOpen
			}
			t.Doors.Set(col, row, door)
		}
	}
	return t
}

// reachableRooms walks the dungeon one square at a time (eight neighbors, each
// step a Move of the terrain) from the entrance and lists the rooms it gets a
// floor square of.
func reachableRooms(t grid.Terrain, start grid.Square) []int {
	seen := map[grid.Square]bool{start: true}
	queue := []grid.Square{start}
	mover := grid.Mover{Size: grid.SizeMedium}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				next := grid.Square{Col: cur.Col + dc, Row: cur.Row + dr}
				if (dc == 0 && dr == 0) || seen[next] || !t.Grid.Contains(next) {
					continue
				}
				if mv := t.Move(cur, next, nil, mover); !mv.Blocked {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	var rooms []int
	for i, r := range dungeonRooms {
		if seen[grid.Square{Col: r[0], Row: r[1]}] {
			rooms = append(rooms, i+1)
		}
	}
	return rooms
}

func TestTheDungeonsDoors(t *testing.T) {
	t.Parallel()
	entrance := grid.Square{Col: 2, Row: 2}
	cases := []struct {
		name string
		open bool
		want []int
	}{
		{"every door open", true, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"as painted: locked, barred and secret doors block", false, []int{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, c := range cases {
		if got := reachableRooms(dungeon(c.open), entrance); !slices.Equal(got, c.want) {
			t.Errorf("%s: rooms %v, want %v", c.name, got, c.want)
		}
	}
	// What blocks, one door at a time: the throne room is still reachable without
	// the locked door (through the closed one at (21, 8)), and the secret chamber
	// only through the secret door.
	for _, c := range []struct {
		sq   grid.Square
		want grid.Door
	}{{grid.Square{Col: 20, Row: 2}, grid.DoorLocked}, {grid.Square{Col: 25, Row: 8}, grid.DoorBarred}, {grid.Square{Col: 12, Row: 18}, grid.DoorSecret}} {
		if got := dungeon(false).Doors.At(c.sq); got != c.want {
			t.Errorf("the door at %v is %d, want %d", c.sq, got, c.want)
		}
	}
	// A player's planning treats the locked door as closed and the secret one as
	// a wall: the same rooms come out, and the lock only shows on the real walk.
	known := dungeon(false).ForPlayers()
	if got := reachableRooms(known, entrance); !slices.Equal(got, []int{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Errorf("what a player plans on: rooms %v", got)
	}
	// ... and the real walk through the first door of the party's way.
	st := dungeon(false).MoveUntil(grid.Square{Col: 19, Row: 2}, grid.Square{Col: 22, Row: 2}, nil, grid.Mover{Size: grid.SizeMedium}, nil, 1000)
	if st.Reason != grid.StopLocked || st.LockedAt != (grid.Square{Col: 20, Row: 2}) || st.Reached != (grid.Square{Col: 19, Row: 2}) {
		t.Errorf("the throne room's locked door: %+v", st)
	}
}
