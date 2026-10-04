package grid_test

import (
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The cave "A caverna do Vale Seco" of the Etapa 9 artboards (24 x 16 squares):
// # wall, . floor, : rubble (difficult terrain), h crates (half cover, can be
// crossed), q a stone column (three-quarters cover, blocks movement). The
// expected numbers in the tests come from cave.py (the Etapa 9 artboards),
// whose lines are sampled; the functions here walk the line exactly.
var caveRows = []string{
	"########################",
	"########################",
	"################.......#",
	"################.......#",
	"################....q..#",
	"################.......#",
	"#......#########.......#",
	"...................h...#",
	"...................h...#",
	"#...::.#..######.......#",
	"#...::.#..##############",
	"######........##########",
	"######........##########",
	"######........##########",
	"######........##########",
	"########################",
}

var caveGrid = grid.Grid{Columns: 24, Rows: 16}

func cave() grid.Terrain {
	t := grid.Terrain{
		Grid:      caveGrid,
		Walls:     grid.NewLayer(caveGrid),
		Difficult: grid.NewLayer(caveGrid),
		Cover:     grid.NewCoverLayer(caveGrid),
	}
	for row, line := range caveRows {
		for col, ch := range line {
			switch ch {
			case '#':
				t.Walls.Set(col, row, true)
			case ':':
				t.Difficult.Set(col, row, true)
			case 'h':
				t.Cover.Set(col, row, grid.CoverHalf)
			case 'q':
				t.Cover.Set(col, row, grid.CoverThreeQuarters)
			}
		}
	}
	return t
}

// The party of the artboards, in the entrance cave, and the goblins in the
// guard room.
var (
	toren     = grid.Square{Col: 6, Row: 7}
	pensantus = grid.Square{Col: 5, Row: 8}
	brisa     = grid.Square{Col: 4, Row: 7}
	salvia    = grid.Square{Col: 3, Row: 8}
	goblins   = []grid.Square{{Col: 18, Row: 5}, {Col: 20, Row: 7}, {Col: 21, Row: 3}}
)

// partyBut is the party's squares except the mover's, as friendly Medium
// occupants.
func partyBut(mover grid.Square) grid.OccupantMap {
	m := grid.OccupantMap{}
	for _, sq := range []grid.Square{toren, pensantus, brisa, salvia} {
		if sq != mover {
			m[sq] = grid.Occupant{Size: grid.SizeMedium}
		}
	}
	return m
}

// render draws a reach the way cave.py does: @ the start, o another creature,
// x an enemy, # wall, q a column, + reachable floor, * reachable rubble, : rubble
// out of reach, . floor out of reach.
func render(t grid.Terrain, from grid.Square, occ grid.OccupantMap, reach []grid.Reachable) string {
	in := map[grid.Square]bool{}
	for _, r := range reach {
		in[r.Square] = true
	}
	var b strings.Builder
	for row := 0; row < t.Grid.Rows; row++ {
		for col := 0; col < t.Grid.Columns; col++ {
			sq := grid.Square{Col: col, Row: row}
			ch := byte('.')
			o, occupied := occ[sq]
			switch {
			case sq == from:
				ch = '@'
			case occupied && o.Hostile:
				ch = 'x'
			case occupied:
				ch = 'o'
			case t.Walls.Has(sq):
				ch = '#'
			case t.Cover.Get(col, row) == grid.CoverThreeQuarters:
				ch = 'q'
			case in[sq] && t.Difficult.Has(sq):
				ch = '*'
			case in[sq]:
				ch = '+'
			case t.Difficult.Has(sq):
				ch = ':'
			}
			b.WriteByte(ch)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestReachAgainstTheCave(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		from grid.Square
		left int
		occ  func() grid.OccupantMap
		want string
	}{
		{"from Toren, 30 ft", toren, 300, func() grid.OccupantMap { return partyBut(toren) }, `
########################
########################
################.......#
################.......#
################....q..#
################.......#
#.+++++#########.......#
.+++o+@++++++..........#
...o+o++++++...........#
#..+**+#++######.......#
#...:*+#.+##############
######+...+...##########
######+.......##########
######+.......##########
######........##########
########################
`},
		{"from (10, 8), 30 ft", grid.Square{Col: 10, Row: 8}, 300, func() grid.OccupantMap { return partyBut(grid.Square{}) }, `
########################
########################
################.......#
################.......#
################....q..#
################.......#
#......#########.......#
....o.o+++++++++.......#
...o.o++++@++++++......#
#...::.#++######.......#
#...::.#+.##############
######.+......##########
######+.......##########
######........##########
######........##########
########################
`},
		{"from (16, 8) with the goblins as enemies", grid.Square{Col: 16, Row: 8}, 300, func() grid.OccupantMap {
			m := partyBut(grid.Square{})
			for _, g := range goblins {
				m[g] = grid.Occupant{Size: grid.SizeSmall, Hostile: true}
			}
			return m
		}, `
########################
########################
################+......#
################+++..x.#
################++..q..#
################++x+++.#
#......#########+++++..#
....o.o....+++++++++x..#
...o.o....++++++@++++++#
#...::.#..######++++++.#
#...::.#..##############
######........##########
######........##########
######........##########
######........##########
########################
`},
	}
	for _, c := range cases {
		occ := c.occ()
		got := render(cave(), c.from, occ, cave().Reach(c.from, c.left, occ, grid.Mover{}))
		if want := strings.TrimPrefix(c.want, "\n"); got != want {
			t.Errorf("%s:\n got:\n%s want:\n%s", c.name, got, want)
		}
	}
}

func TestMoveCostAgainstTheCave(t *testing.T) {
	t.Parallel()
	c := cave()
	cases := []struct {
		name     string
		from, to grid.Square
		wantDFt  int // 0 with blocked
		blocked  bool
		entered  []grid.Square
	}{
		// cave.py cost 3 8 6 10: 18,03 ft and three rubble squares, 33,03 ft.
		{
			"through the rubble", salvia,
			grid.Square{Col: 6, Row: 10},
			330, false,
			[]grid.Square{{4, 8}, {4, 9}, {5, 9}, {5, 10}, {6, 10}},
		},
		// cave.py cost 10 7 13 7: three squares straight.
		{
			"three squares straight",
			grid.Square{Col: 10, Row: 7},
			grid.Square{Col: 13, Row: 7},
			150, false,
			[]grid.Square{{11, 7}, {12, 7}, {13, 7}},
		},
		// cave.py cost 10 8 9 10: through the rock.
		{"into a wall", grid.Square{Col: 10, Row: 8}, grid.Square{Col: 9, Row: 10}, 0, true, nil},
		{"one square straight", grid.Square{Col: 10, Row: 7}, grid.Square{Col: 11, Row: 7}, 50, false, []grid.Square{{11, 7}}},
		// A diagonal neighbor: 7,07 ft exactly the way the circle asks, to the tenth.
		{"one square diagonally", grid.Square{Col: 10, Row: 7}, grid.Square{Col: 11, Row: 8}, 71, false, []grid.Square{{11, 8}}},
		{"one square straight into rubble", grid.Square{Col: 3, Row: 9}, grid.Square{Col: 4, Row: 9}, 100, false, []grid.Square{{4, 9}}},
		{"half cover is crossed at no cost", grid.Square{Col: 18, Row: 7}, grid.Square{Col: 20, Row: 7}, 100, false, []grid.Square{{19, 7}, {20, 7}}},
		{"a column blocks", grid.Square{Col: 19, Row: 4}, grid.Square{Col: 21, Row: 4}, 0, true, nil},
		{"the column's square can't be entered", grid.Square{Col: 19, Row: 4}, grid.Square{Col: 20, Row: 4}, 0, true, nil},
		{"outside the grid", grid.Square{Col: 0, Row: 7}, grid.Square{Col: -1, Row: 7}, 0, true, nil},
	}
	for _, tc := range cases {
		occ := partyBut(tc.from)
		got := c.Move(tc.from, tc.to, occ, grid.Mover{})
		if got.Blocked != tc.blocked || got.CostDFt != tc.wantDFt {
			t.Errorf("%s: got blocked=%v cost=%d, want blocked=%v cost=%d", tc.name, got.Blocked, got.CostDFt, tc.blocked, tc.wantDFt)
		}
		if !tc.blocked && !equalSquares(got.Entered, tc.entered) {
			t.Errorf("%s: entered %v, want %v", tc.name, got.Entered, tc.entered)
		}
	}
}

func equalSquares(a, b []grid.Square) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
