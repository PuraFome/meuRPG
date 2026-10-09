package grid_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// picture draws a set of squares over a rows × cols window: # for a wall, the
// caster or origin as O, a square of the set as x and the rest as dots. It makes
// the shape of an area a thing a test reads at a glance.
func picture(t grid.Terrain, origin grid.Square, set []grid.Square) string {
	var b strings.Builder
	for row := range t.Grid.Rows {
		for col := range t.Grid.Columns {
			sq := grid.Square{Col: col, Row: row}
			switch {
			case t.Walls.Has(sq):
				b.WriteByte('#')
			case slices.Contains(set, sq):
				b.WriteByte('x')
			case sq == origin:
				b.WriteByte('O')
			default:
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// open is a terrain of size columns × rows with the walls the rows draw (# is a
// wall, anything else floor); a row shorter than the grid is open at its end.
func open(columns int, rows ...string) grid.Terrain {
	g := grid.Grid{Columns: columns, Rows: len(rows)}
	t := grid.Terrain{Grid: g, Walls: grid.NewLayer(g), Cover: grid.NewCoverLayer(g)}
	for row, line := range rows {
		for col, ch := range line {
			switch ch {
			case '#':
				t.Walls.Set(col, row, true)
			case 'h':
				t.Cover.Set(col, row, grid.CoverHalf)
			}
		}
	}
	return t
}

func blank(columns, rows int) grid.Terrain {
	return open(columns, slices.Repeat([]string{""}, rows)...)
}

func TestSphereIsCenteredOnItsOriginWithTheExactRadius(t *testing.T) {
	t.Parallel()
	area := grid.Area{Shape: grid.ShapeSphere, SizeFt: 20}
	origin := grid.Square{Col: 6, Row: 6}
	got := area.FromPoint(origin)
	if len(got) != 49 {
		t.Fatalf("a 20 ft sphere has %d squares, want 49 (those with dc²+dr² ≤ 16)", len(got))
	}
	for _, in := range []grid.Square{origin, {Col: 10, Row: 6}, {Col: 6, Row: 2}, {Col: 9, Row: 8}} {
		if !slices.Contains(got, in) {
			t.Errorf("%v is inside the sphere, and was left out", in)
		}
	}
	// 4,1 is 4.12 squares away: the range rounds it down to 4 (RN-21), but the area
	// is exact, so the sphere does not grow by a square.
	for _, out := range []grid.Square{{Col: 10, Row: 7}, {Col: 9, Row: 9}, {Col: 11, Row: 6}} {
		if slices.Contains(got, out) {
			t.Errorf("%v is outside the sphere, and was put in", out)
		}
	}
	if small := (grid.Area{Shape: grid.ShapeSphere, SizeFt: 5}).FromPoint(origin); len(small) != 5 {
		t.Errorf("a 5 ft sphere has %d squares, want the origin and its four neighbors", len(small))
	}
	if cyl := (grid.Area{Shape: grid.ShapeCylinder, SizeFt: 20}).FromPoint(origin); len(cyl) != 49 {
		t.Errorf("a 20 ft cylinder has %d squares, want 49: its height does not matter on a flat map", len(cyl))
	}
	if (grid.Area{Shape: grid.ShapeCone, SizeFt: 15}).FromPoint(origin) != nil {
		t.Error("a cone has no point to be centered on")
	}
}

func TestConeFromTheCasterIsAsWideAsItIsFarFromTheCaster(t *testing.T) {
	t.Parallel()
	g := blank(9, 9)
	caster := grid.Square{Col: 4, Row: 4}
	cone := grid.Area{Shape: grid.ShapeCone, SizeFt: 15}
	cases := []struct {
		name string
		dir  grid.Direction
		want string
	}{
		{"east", grid.Direction{Dx: 1}, `
.........
.........
.........
......xx.
....Oxxx.
......xx.
.........
.........
.........
`},
		{"south-east", grid.Direction{Dx: 1, Dy: 1}, `
.........
.........
.........
.........
....O....
.....xxx.
.....xx..
.....x...
.........
`},
		{"north", grid.Direction{Dy: -1}, `
.........
...xxx...
...xxx...
....x....
....O....
.........
.........
.........
.........
`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := g.OpenArea(caster, cone.FromCaster(caster, c.dir), false)
			if got := "\n" + picture(g, caster, got); got != c.want {
				t.Errorf("the cone to the %s is\n%s\nwant\n%s", c.name, got, c.want)
			}
		})
	}
}

func TestLineAndCubeFromTheCaster(t *testing.T) {
	t.Parallel()
	g := blank(30, 30)
	caster := grid.Square{Col: 2, Row: 15}
	line := grid.Area{Shape: grid.ShapeLine, SizeFt: 100}
	east := g.OpenArea(caster, line.FromCaster(caster, grid.Direction{Dx: 1}), false)
	if len(east) != 20 || east[0] != (grid.Square{Col: 3, Row: 15}) || east[19] != (grid.Square{Col: 22, Row: 15}) {
		t.Errorf("a 100 ft line to the east = %v, want the 20 squares of the row after the caster", east)
	}
	if slices.Contains(east, caster) {
		t.Error("the line has the caster's own square")
	}
	// On a diagonal the length is measured along the diagonal, not by the squares:
	// 100 ft is 20 squares, and 14 of them diagonal squares are 19.8 long.
	south := grid.Square{Col: 2, Row: 25}
	if diag := g.OpenArea(south, line.FromCaster(south, grid.Direction{Dx: 1, Dy: -1}), false); len(diag) != 14 {
		t.Errorf("a 100 ft line on the diagonal has %d squares, want 14 (14 diagonal squares are 19.8 squares long)", len(diag))
	}
	wide := grid.Area{Shape: grid.ShapeLine, SizeFt: 60, WidthFt: 10}
	if got := len(wide.FromCaster(caster, grid.Direction{Dx: 1})); got != 24 {
		t.Errorf("a 60 ft × 10 ft line has %d squares, want 12 long × 2 wide = 24", got)
	}

	cube := grid.Area{Shape: grid.ShapeCube, SizeFt: 15}
	cases := []struct {
		dir                    grid.Direction
		col0, col1, row0, row1 int
	}{
		{grid.Direction{Dx: 1}, 3, 5, 14, 16},
		{grid.Direction{Dx: -1}, -1, 1, 14, 16},
		{grid.Direction{Dy: 1}, 1, 3, 16, 18},
		{grid.Direction{Dx: 1, Dy: 1}, 3, 5, 16, 18},
		{grid.Direction{Dx: -1, Dy: -1}, -1, 1, 12, 14},
	}
	for _, c := range cases {
		got := cube.FromCaster(caster, c.dir)
		if len(got) != 9 {
			t.Errorf("cube %v has %d squares, want 9", c.dir, len(got))
			continue
		}
		for _, sq := range got {
			if sq == caster || sq.Col < c.col0 || sq.Col > c.col1 || sq.Row < c.row0 || sq.Row > c.row1 {
				t.Errorf("cube %v has %v, outside columns %d-%d rows %d-%d or the caster's own square", c.dir, sq, c.col0, c.col1, c.row0, c.row1)
			}
		}
	}
	if (grid.Area{Shape: grid.ShapeCone, SizeFt: 15}).FromCaster(caster, grid.Direction{}) != nil {
		t.Error("no direction, no area")
	}
}

func TestTowardPicksTheNearestOfTheEightDirections(t *testing.T) {
	t.Parallel()
	from := grid.Square{Col: 5, Row: 5}
	cases := []struct {
		to   grid.Square
		want grid.Direction
	}{
		{grid.Square{Col: 9, Row: 5}, grid.Direction{Dx: 1}},
		{grid.Square{Col: 9, Row: 6}, grid.Direction{Dx: 1}},        // 14°: still east
		{grid.Square{Col: 9, Row: 7}, grid.Direction{Dx: 1, Dy: 1}}, // 26°: the diagonal
		{grid.Square{Col: 7, Row: 7}, grid.Direction{Dx: 1, Dy: 1}},
		{grid.Square{Col: 5, Row: 1}, grid.Direction{Dy: -1}},
		{grid.Square{Col: 3, Row: 2}, grid.Direction{Dx: -1, Dy: -1}},
		{grid.Square{Col: 1, Row: 5}, grid.Direction{Dx: -1}},
	}
	for _, c := range cases {
		if got, ok := grid.Toward(from, c.to); !ok || got != c.want {
			t.Errorf("Toward(%v, %v) = %v, %v; want %v", from, c.to, got, ok, c.want)
		}
	}
	if _, ok := grid.Toward(from, from); ok {
		t.Error("a square has no direction to itself")
	}
}

func TestOnlyTotalCoverBlocksTheLineOfEffect(t *testing.T) {
	t.Parallel()
	// The line goes east along row 1: the crates (half cover) do not stop it, the
	// wall does.
	g := open(12,
		"............",
		"..h.....#...",
		"............",
	)
	caster := grid.Square{Col: 0, Row: 1}
	line := grid.Area{Shape: grid.ShapeLine, SizeFt: 100}
	got := g.OpenArea(caster, line.FromCaster(caster, grid.Direction{Dx: 1}), false)
	if len(got) != 7 || got[len(got)-1] != (grid.Square{Col: 7, Row: 1}) || !slices.Contains(got, grid.Square{Col: 2, Row: 1}) {
		t.Errorf("the line = %v, want squares 1 to 7 of the row: the crates are in, the wall and what is behind it are out", got)
	}
}

func TestASphereStopsAtAWallUnlessItSpreadsAroundCorners(t *testing.T) {
	t.Parallel()
	// A wall with a gap: the origin is on the left, in line with the lower part of
	// the wall, and the room beyond the gap is reached only around the corner.
	g := open(11,
		".....#.....",
		".....#.....",
		".....#.....",
		".....#.....",
		"...........",
		".....#.....",
		".....#.....",
	)
	origin := grid.Square{Col: 3, Row: 1}
	sphere := grid.Area{Shape: grid.ShapeSphere, SizeFt: 25}
	raw := sphere.FromPoint(origin)

	straight := g.OpenArea(origin, raw, false)
	around := g.OpenArea(origin, raw, true)
	behind := grid.Square{Col: 7, Row: 3} // 4,2 from the origin: inside the sphere, behind the wall
	if slices.Contains(straight, behind) {
		t.Errorf("a straight line reaches %v through the wall", behind)
	}
	if !slices.Contains(around, behind) {
		t.Errorf("a sphere that spreads around corners does not reach %v by the gap: %s", behind, picture(g, origin, around))
	}
	for _, s := range around {
		if g.Walls.Has(s) {
			t.Errorf("the area has the wall at %v", s)
		}
	}
	if len(around) <= len(straight) {
		t.Errorf("spreading around corners gives %d squares, straight lines %d: want more", len(around), len(straight))
	}
	// What is reached around the corner is still within the sphere.
	for _, s := range around {
		if dc, dr := s.Col-origin.Col, s.Row-origin.Row; dc*dc+dr*dr > 25 {
			t.Errorf("%v is outside the 5-square sphere", s)
		}
	}
	if !slices.Contains(straight, origin) || !slices.Contains(around, origin) {
		t.Error("the origin of a sphere is inside it")
	}
}

func TestFireDoesNotSqueezeBetweenTwoWallsThatTouchAtACorner(t *testing.T) {
	t.Parallel()
	g := open(5,
		"#####",
		"#.###",
		"##.##",
		"#####",
	)
	origin := grid.Square{Col: 1, Row: 1}
	across := grid.Square{Col: 2, Row: 2} // diagonal from the origin, through the corner the two walls share
	raw := (grid.Area{Shape: grid.ShapeSphere, SizeFt: 10}).FromPoint(origin)
	for _, corners := range []bool{false, true} {
		if got := g.OpenArea(origin, raw, corners); slices.Contains(got, across) || !slices.Contains(got, origin) {
			t.Errorf("corners=%v: the area is %v, want the origin alone: the fire does not slip between two walls", corners, got)
		}
	}
}

func TestAnOriginInsideAWallHasNoArea(t *testing.T) {
	t.Parallel()
	g := open(5, "..#..")
	if got := g.OpenArea(grid.Square{Col: 2, Row: 0}, (grid.Area{Shape: grid.ShapeSphere, SizeFt: 10}).FromPoint(grid.Square{Col: 2, Row: 0}), true); got != nil {
		t.Errorf("an origin on a wall has the area %v", got)
	}
	if got := g.OpenArea(grid.Square{Col: 9, Row: 0}, []grid.Square{{Col: 9, Row: 0}}, false); got != nil {
		t.Errorf("an origin off the map has the area %v", got)
	}
}

func TestAPointBehindAWallComesIntoBeingOnTheNearSide(t *testing.T) {
	t.Parallel()
	g := open(12,
		"............",
		".....#......",
		"............",
	)
	caster := grid.Square{Col: 1, Row: 1}
	if got := g.NearSide(caster, grid.Square{Col: 9, Row: 1}); got != (grid.Square{Col: 4, Row: 1}) {
		t.Errorf("NearSide behind the wall = %v, want the square before it (4,1)", got)
	}
	if got := g.NearSide(caster, grid.Square{Col: 5, Row: 1}); got != (grid.Square{Col: 4, Row: 1}) {
		t.Errorf("NearSide on the wall = %v, want the square before it (4,1)", got)
	}
	if got := g.NearSide(caster, grid.Square{Col: 6, Row: 2}); got != (grid.Square{Col: 6, Row: 2}) {
		t.Errorf("NearSide with a clear line = %v, want the point itself", got)
	}
	if got := g.NearSide(caster, caster); got != caster {
		t.Errorf("NearSide on the caster = %v", got)
	}
}
