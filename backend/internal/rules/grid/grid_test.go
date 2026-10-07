package grid_test

import (
	"errors"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

func TestRowsFor(t *testing.T) {
	t.Parallel()
	cases := []struct{ columns, w, h, want int }{
		{24, 1200, 800, 16}, // the cave: 3:2
		{10, 1000, 1000, 10},
		{10, 1000, 1049, 10}, // 10,49 rounds down
		{10, 1000, 1050, 11}, // 10,5 rounds up
		{4, 100000, 10, 1},   // a very wide image: at least 1 row
		{200, 100, 100000, 400},
		{0, 1000, 1000, 0},
		{10, 0, 1000, 0},
	}
	for _, c := range cases {
		if got := grid.RowsFor(c.columns, c.w, c.h); got != c.want {
			t.Errorf("RowsFor(%d, %d, %d) = %d, want %d", c.columns, c.w, c.h, got, c.want)
		}
	}
}

// The numbers below are what play/combat_rules.go squareOf and centerOf give
// today (the formulas are the same); the grid is the cave's 24 x 16.
func TestBasisPoints(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 24, Rows: 16}
	squares := []struct {
		x, y int
		want grid.Square
	}{
		{0, 0, grid.Square{Col: 0, Row: 0}},
		{416, 624, grid.Square{Col: 0, Row: 0}},
		{417, 625, grid.Square{Col: 1, Row: 1}},
		{5000, 5000, grid.Square{Col: 12, Row: 8}},
		{9999, 9999, grid.Square{Col: 23, Row: 15}},
		{10000, 10000, grid.Square{Col: 23, Row: 15}}, // an edge belongs to the last square
		{-5, 20000, grid.Square{Col: 0, Row: 15}},     // outside: the nearest
	}
	for _, c := range squares {
		if got := g.SquareOf(c.x, c.y); got != c.want {
			t.Errorf("SquareOf(%d, %d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	centers := []struct {
		sq   grid.Square
		x, y int
	}{
		{grid.Square{Col: 0, Row: 0}, 208, 312},
		{grid.Square{Col: 12, Row: 8}, 5208, 5312},
		{grid.Square{Col: 23, Row: 15}, 9791, 9687},
	}
	for _, c := range centers {
		if x, y := g.CenterOf(c.sq); x != c.x || y != c.y {
			t.Errorf("CenterOf(%v) = (%d, %d), want (%d, %d)", c.sq, x, y, c.x, c.y)
		}
	}
	// A token placed on the middle of a square is in that square, on every grid.
	for _, g := range []grid.Grid{{Columns: 4, Rows: 3}, {Columns: 24, Rows: 16}, {Columns: 37, Rows: 91}, {Columns: 200, Rows: 400}} {
		for row := range g.Rows {
			for col := range g.Columns {
				sq := grid.Square{Col: col, Row: row}
				x, y := g.CenterOf(sq)
				if got := g.SquareOf(x, y); got != sq {
					t.Fatalf("grid %v: SquareOf(CenterOf(%v)) = %v", g, sq, got)
				}
			}
		}
	}
}

func TestLayerEncoding(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 5, Rows: 3} // 15 squares: 2 bytes, the last with 1 used bit
	l := grid.NewLayer(g)
	l.Set(0, 0, true) // square 0: bit 0 of byte 0
	l.Set(2, 1, true) // square 7: bit 7 of byte 0
	l.Set(4, 2, true) // square 14: bit 6 of byte 1
	if got := l.Encode(); len(got) != 2 || got[0] != 0b1000_0001 || got[1] != 0b0100_0000 {
		t.Fatalf("Encode() = %08b, want [10000001 01000000]", got)
	}
	back, err := grid.DecodeLayer(g, l.Encode())
	if err != nil {
		t.Fatal(err)
	}
	for row := range g.Rows {
		for col := range g.Columns {
			if back.Get(col, row) != l.Get(col, row) {
				t.Errorf("square (%d, %d) did not survive the round trip", col, row)
			}
		}
	}
	if l.Count() != 3 || l.Get(1, 0) || l.Get(-1, 0) || l.Get(5, 0) || l.Get(0, 3) {
		t.Error("Count, Get and the squares outside the grid are wrong")
	}
	if l.Set(5, 0, true) || l.Set(0, -1, true) {
		t.Error("Set must refuse a square outside the grid")
	}
	l.Set(0, 0, false)
	if l.Get(0, 0) || l.Count() != 2 {
		t.Error("Set(false) must clear a square")
	}
	var nilLayer *grid.Layer
	if nilLayer.Get(0, 0) || nilLayer.Count() != 0 || nilLayer.Encode() != nil {
		t.Error("a nil Layer reads as empty")
	}

	for name, b := range map[string][]byte{"too short": {0}, "too long": {0, 0, 0}, "unused bit set": {0, 0b1000_0000}} {
		if _, err := grid.DecodeLayer(g, b); !errors.Is(err, grid.ErrLayerSize) {
			t.Errorf("DecodeLayer(%s) = %v, want ErrLayerSize", name, err)
		}
	}
	if _, err := grid.DecodeLayer(grid.Grid{}, nil); !errors.Is(err, grid.ErrLayerSize) {
		t.Error("DecodeLayer must refuse an invalid grid")
	}
}

func TestLightAndCoverLayerEncoding(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 3, Rows: 2} // 6 squares: 2 bytes, the last with 2 used pairs
	l := grid.NewLightLayer(g)
	l.Set(0, 0, grid.Dark)   // square 0: bits 0-1 of byte 0
	l.Set(1, 0, grid.Dim)    // square 1: bits 2-3
	l.Set(2, 0, grid.Bright) // square 2: bits 4-5
	l.Set(2, 1, grid.Bright) // square 5: bits 2-3 of byte 1
	if got := l.Encode(); len(got) != 2 || got[0] != 0b00_11_10_01 || got[1] != 0b0000_1100 {
		t.Fatalf("Encode() = %08b, want [00111001 00001100]", got)
	}
	back, err := grid.DecodeLightLayer(g, l.Encode())
	if err != nil {
		t.Fatal(err)
	}
	for row := range g.Rows {
		for col := range g.Columns {
			if back.Get(col, row) != l.Get(col, row) {
				t.Errorf("light at (%d, %d) did not survive the round trip", col, row)
			}
		}
	}
	if l.Get(0, 1) != grid.Unpainted || l.Get(9, 9) != grid.Unpainted {
		t.Error("an unpainted square and one outside the grid read Unpainted")
	}
	l.Set(0, 0, grid.Unpainted)
	if l.Get(0, 0) != grid.Unpainted || l.Set(0, 0, 4) {
		t.Error("Unpainted clears a square and 4 is not a light")
	}
	for name, b := range map[string][]byte{"too short": {0}, "too long": {0, 0, 0}, "unused bits set": {0, 0b0001_0000}} {
		if _, err := grid.DecodeLightLayer(g, b); !errors.Is(err, grid.ErrLayerSize) {
			t.Errorf("DecodeLightLayer(%s) = %v, want ErrLayerSize", name, err)
		}
	}

	c := grid.NewCoverLayer(g)
	c.Set(1, 0, grid.CoverHalf)
	c.Set(2, 1, grid.CoverThreeQuarters)
	if c.Set(0, 0, grid.CoverTotal) {
		t.Error("CoverTotal is never stored")
	}
	if got := c.Encode(); len(got) != 2 || got[0] != 0b0000_0100 || got[1] != 0b0000_1000 {
		t.Fatalf("Encode() = %08b, want [00000100 00001000]", got)
	}
	backCover, err := grid.DecodeCoverLayer(g, c.Encode())
	if err != nil || backCover.Get(1, 0) != grid.CoverHalf || backCover.Get(2, 1) != grid.CoverThreeQuarters {
		t.Errorf("cover round trip: %v %v", backCover, err)
	}
	if _, err := grid.DecodeCoverLayer(g, []byte{0b11, 0}); !errors.Is(err, grid.ErrLayerSize) {
		t.Errorf("DecodeCoverLayer must refuse a 3, got %v", err)
	}
	if _, err := grid.DecodeCoverLayer(g, []byte{0}); !errors.Is(err, grid.ErrLayerSize) {
		t.Error("DecodeCoverLayer must refuse a wrong length")
	}
}

func steps(line []grid.Step) []grid.Square {
	out := make([]grid.Square, len(line))
	for i, s := range line {
		out[i] = s.Square
	}
	return out
}

func TestLine(t *testing.T) {
	t.Parallel()
	sq := func(c, r int) grid.Square { return grid.Square{Col: c, Row: r} }
	cases := []struct {
		name     string
		from, to grid.Square
		want     []grid.Square
	}{
		{"the same square", sq(3, 3), sq(3, 3), nil},
		{"straight east", sq(0, 0), sq(3, 0), []grid.Square{sq(1, 0), sq(2, 0), sq(3, 0)}},
		{"straight north", sq(2, 5), sq(2, 3), []grid.Square{sq(2, 4), sq(2, 3)}},
		{"diagonal: through the corners", sq(0, 0), sq(3, 3), []grid.Square{sq(1, 1), sq(2, 2), sq(3, 3)}},
		{"diagonal neighbor", sq(4, 4), sq(3, 3), []grid.Square{sq(3, 3)}},
		// (0,0) to (2,1) crosses the edge between (1,0) and (1,1) at its middle.
		{"knight's move", sq(0, 0), sq(2, 1), []grid.Square{sq(1, 0), sq(1, 1), sq(2, 1)}},
		{"knight's move, mirrored", sq(2, 1), sq(0, 0), []grid.Square{sq(1, 1), sq(1, 0), sq(0, 0)}},
		// (0,0) to (3,1) passes exactly through the corner at x=2, y=1, shared by
		// (1,0) (2,0) (1,1) (2,1): it goes from (1,0) to (2,1) and enters neither
		// of the other two.
		{"through a corner", sq(0, 0), sq(3, 1), []grid.Square{sq(1, 0), sq(2, 1), sq(3, 1)}},
		{"long and shallow", sq(0, 0), sq(5, 1), []grid.Square{sq(1, 0), sq(2, 0), sq(3, 1), sq(4, 1), sq(5, 1)}},
		{"steep", sq(0, 0), sq(1, 3), []grid.Square{sq(0, 1), sq(1, 2), sq(1, 3)}},
	}
	for _, c := range cases {
		got := steps(grid.Line(c.from, c.to))
		if !equalSquares(got, c.want) {
			t.Errorf("%s: Line(%v, %v) = %v, want %v", c.name, c.from, c.to, got, c.want)
		}
	}

	// The corner step names the two squares it slipped between.
	line := grid.Line(sq(0, 0), sq(2, 2))
	if !line[0].Corner || line[0].SideA != sq(1, 0) || line[0].SideB != sq(0, 1) {
		t.Errorf("first corner step = %+v", line[0])
	}
	if c := grid.Line(sq(0, 0), sq(3, 1))[1]; !c.Corner || c.SideA != sq(2, 0) || c.SideB != sq(1, 1) {
		t.Errorf("the corner step of (0,0) to (3,1) = %+v", c)
	}
	if grid.Line(sq(0, 0), sq(2, 1))[0].Corner {
		t.Error("a line through the middle of an edge is not a corner step")
	}
}

// TestLineIsSymmetricAndConnected: whichever end it starts from, a line enters
// the same squares (but for the ends), and each step moves to a neighbor.
func TestLineIsSymmetricAndConnected(t *testing.T) {
	t.Parallel()
	rng := newRNG(1)
	for range 2000 {
		a := grid.Square{Col: rng.intn(40), Row: rng.intn(40)}
		b := grid.Square{Col: rng.intn(40), Row: rng.intn(40)}
		fwd, back := steps(grid.Line(a, b)), steps(grid.Line(b, a))
		if a == b {
			continue
		}
		if len(fwd) != len(back) {
			t.Fatalf("%v -> %v: %d squares forward, %d back", a, b, len(fwd), len(back))
		}
		prev := a
		for _, s := range fwd {
			if d := grid.RangeSquares(prev, s); d < 1 || abs(s.Col-prev.Col) > 1 || abs(s.Row-prev.Row) > 1 {
				t.Fatalf("%v -> %v: step %v to %v is not to a neighbor", a, b, prev, s)
			}
			prev = s
		}
		// Reversed, the squares are the same set, with a and b swapped at the ends.
		want := map[grid.Square]bool{a: true}
		for _, s := range fwd[:len(fwd)-1] {
			want[s] = true
		}
		for _, s := range back[:len(back)-1] {
			delete(want, s)
		}
		if len(want) != 1 || !want[a] {
			t.Fatalf("%v -> %v: the line is not the same both ways: %v and %v", a, b, fwd, back)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestSqueeze(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 5, Rows: 5}
	walls := grid.NewLayer(g)
	// Two walls touch at a corner: (2,1) and (1,2). The diagonal from (1,1) to
	// (2,2) goes through that corner.
	walls.Set(2, 1, true)
	walls.Set(1, 2, true)
	tr := grid.Terrain{Grid: g, Walls: walls}
	from, to := grid.Square{Col: 1, Row: 1}, grid.Square{Col: 2, Row: 2}
	if mv := tr.Move(from, to, nil, grid.Mover{}); !mv.Blocked || mv.By != grid.StopWall {
		t.Errorf("squeezing between two walls must be blocked, got %+v", mv)
	}
	if mustSight(t, g, walls).Clear(from, to) {
		t.Error("sight must not slip between two walls that touch at a corner")
	}
	// With one of the two a floor, the line passes.
	walls.Set(1, 2, false)
	tr.Walls = walls
	if mv := tr.Move(from, to, nil, grid.Mover{}); mv.Blocked || mv.CostDFt != 71 {
		t.Errorf("one wall only: got %+v", mv)
	}
	if !mustSight(t, g, walls).Clear(from, to) {
		t.Error("with one wall only, sight passes")
	}
	// A three-quarters column squeezes like a wall for movement, not for sight.
	cover := grid.NewCoverLayer(g)
	cover.Set(1, 2, grid.CoverThreeQuarters)
	tr.Cover = cover
	if mv := tr.Move(from, to, nil, grid.Mover{}); !mv.Blocked {
		t.Error("a wall and a column that touch at a corner squeeze movement")
	}
}

// TestSightMatchesTheLine: Sight.Clear agrees with walking Line by hand, on
// random walls.
func TestSightMatchesTheLine(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 30, Rows: 20}
	rng := newRNG(7)
	walls := grid.NewLayer(g)
	for range 120 {
		walls.Set(rng.intn(g.Columns), rng.intn(g.Rows), true)
	}
	sight := mustSight(t, g, walls)
	for range 3000 {
		a := grid.Square{Col: rng.intn(g.Columns), Row: rng.intn(g.Rows)}
		b := grid.Square{Col: rng.intn(g.Columns), Row: rng.intn(g.Rows)}
		want := true
		line := grid.Line(a, b)
		for i, s := range line {
			if s.Corner && walls.Has(s.SideA) && walls.Has(s.SideB) {
				want = false
			}
			if i < len(line)-1 && walls.Has(s.Square) {
				want = false
			}
		}
		if got := sight.Clear(a, b); got != want {
			t.Fatalf("Clear(%v, %v) = %v, want %v", a, b, got, want)
		}
	}
}

func TestRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to grid.Square
		want     int
	}{
		{grid.Square{Col: 5, Row: 5}, grid.Square{Col: 5, Row: 5}, 0},
		{grid.Square{Col: 5, Row: 5}, grid.Square{Col: 6, Row: 6}, 5}, // a diagonal neighbor: melee works on all eight sides
		{grid.Square{Col: 5, Row: 5}, grid.Square{Col: 6, Row: 5}, 5},
		{grid.Square{Col: 0, Row: 0}, grid.Square{Col: 4, Row: 4}, 25}, // 5,66 squares rounds down to 5
		{grid.Square{Col: 0, Row: 0}, grid.Square{Col: 6, Row: 0}, 30},
		{grid.Square{Col: 0, Row: 0}, grid.Square{Col: 3, Row: 4}, 25}, // exactly 5 squares
		{grid.Square{Col: 0, Row: 0}, grid.Square{Col: 2, Row: 2}, 10}, // 2,83 squares
		{grid.Square{Col: 9, Row: 9}, grid.Square{Col: 0, Row: 0}, 60}, // 12,7 squares
	}
	for _, c := range cases {
		if got := grid.RangeFt(c.from, c.to); got != c.want {
			t.Errorf("RangeFt(%v, %v) = %d, want %d", c.from, c.to, got, c.want)
		}
		if got := grid.RangeFt(c.to, c.from); got != c.want {
			t.Errorf("RangeFt(%v, %v) = %d, want %d (not symmetric)", c.to, c.from, got, c.want)
		}
	}
}

// rng is a small deterministic generator (xorshift64), so the random cases of
// a test are the same on every run and every machine.
type rng struct{ state uint64 }

func newRNG(seed uint64) *rng { return &rng{state: seed*0x9E3779B97F4A7C15 + 1} }

func (r *rng) intn(n int) int {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return int(uint32(r.state>>32)) % n
}

func mustSight(t *testing.T, g grid.Grid, walls *grid.Layer) *grid.Sight {
	t.Helper()
	s, err := grid.NewSight(g, walls)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSightOnLongLines checks the stretch-skipping of Clear against walking
// the line square by square, on grids with few walls and long lines, where the
// skipping is what runs.
func TestSightOnLongLines(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 140, Rows: 90}
	for _, perMille := range []int{0, 2, 10, 40} {
		rng := newRNG(3)
		walls := grid.NewLayer(g)
		for row := range g.Rows {
			for col := range g.Columns {
				if rng.intn(1000) < perMille {
					walls.Set(col, row, true)
				}
			}
		}
		sight := mustSight(t, g, walls)
		for range 4000 {
			a := grid.Square{Col: rng.intn(g.Columns), Row: rng.intn(g.Rows)}
			b := grid.Square{Col: rng.intn(g.Columns), Row: rng.intn(g.Rows)}
			want := true
			line := grid.Line(a, b)
			for i, s := range line {
				if s.Corner && walls.Has(s.SideA) && walls.Has(s.SideB) {
					want = false
				}
				if i < len(line)-1 && walls.Has(s.Square) {
					want = false
				}
			}
			if got := sight.Clear(a, b); got != want {
				t.Fatalf("%d per mille: Clear(%v, %v) = %v, want %v", perMille, a, b, got, want)
			}
		}
	}
}

// Untrusted input must never panic: a bad grid, and layers sized for another.
func TestBadGridsAndLayers(t *testing.T) {
	t.Parallel()
	for _, g := range []grid.Grid{{}, {Columns: -3, Rows: 4}, {Columns: 4, Rows: -1}, {Columns: 0, Rows: 5}, {Columns: 999, Rows: 999}} {
		l := grid.NewLayer(g)
		l.Set(0, 0, true)
		_ = l.Get(1, 1)
		_ = grid.NewLightLayer(g).Get(0, 0)
		_ = grid.NewCoverLayer(g).Encode()
		if x, y := g.CenterOf(grid.Square{}); g.Squares() == 0 && (x != 0 || y != 0) {
			t.Errorf("CenterOf on %v = (%d, %d)", g, x, y)
		}
		_ = g.SquareOf(5000, 5000)
		if _, err := grid.NewSight(g, nil); !g.Valid() && err == nil {
			t.Errorf("NewSight accepted the grid %v", g)
		}
		tr := grid.Terrain{Grid: g}
		if !g.Valid() {
			if tr.Validate() == nil {
				t.Errorf("Validate accepted the grid %v", g)
			}
			if mv := tr.Move(grid.Square{}, grid.Square{Col: 1}, nil, grid.Mover{}); !mv.Blocked {
				t.Error("a move on a bad grid must be blocked")
			}
			_ = tr.Reach(grid.Square{}, 300, nil, grid.Mover{})
		}
	}
	// Layers sized for another grid are refused, not misread.
	small, big := grid.Grid{Columns: 4, Rows: 4}, grid.Grid{Columns: 8, Rows: 8}
	if _, err := grid.NewSight(big, grid.NewLayer(small)); err == nil {
		t.Error("NewSight accepted walls of another grid")
	}
	for name, tr := range map[string]grid.Terrain{
		"walls":     {Grid: big, Walls: grid.NewLayer(small)},
		"difficult": {Grid: big, Difficult: grid.NewLayer(small)},
		"cover":     {Grid: big, Cover: grid.NewCoverLayer(small)},
	} {
		if tr.Validate() == nil {
			t.Errorf("Validate accepted %s of another grid", name)
		}
	}
	if err := (grid.Terrain{Grid: big, Walls: grid.NewLayer(big), Cover: grid.NewCoverLayer(big)}).Validate(); err != nil {
		t.Errorf("Validate refused a good terrain: %v", err)
	}
}
