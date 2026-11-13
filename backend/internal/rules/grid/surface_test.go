package grid_test

import (
	"errors"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// surfaceTerrain is a 10 x 4 floor whose squares of columns 2 to 5 of row 0 are water and of row 1 are climbed.
func surfaceTerrain() grid.Terrain {
	g := grid.Grid{Columns: 10, Rows: 4}
	sf := grid.NewSurfaceLayer(g)
	for col := 2; col <= 5; col++ {
		sf.Set(col, 0, grid.SurfaceWater)
		sf.Set(col, 1, grid.SurfaceClimb)
	}
	return grid.Terrain{Grid: g, Surface: sf}
}

func TestSurfaceLayerRoundTrip(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 7, Rows: 3}
	l := grid.NewSurfaceLayer(g)
	l.Set(1, 1, grid.SurfaceWater)
	l.Set(6, 2, grid.SurfaceClimb)
	if l.Set(0, 0, 3) || l.Set(7, 0, grid.SurfaceWater) {
		t.Fatal("a value that is no surface, or a square off the grid, was painted")
	}
	back, err := grid.DecodeSurfaceLayer(g, l.Encode())
	if err != nil || back.Get(1, 1) != grid.SurfaceWater || back.Get(6, 2) != grid.SurfaceClimb || back.Get(0, 0) != grid.SurfaceFloor {
		t.Fatalf("round trip: %v, %v %v", err, back.Get(1, 1), back.Get(6, 2))
	}
	if _, err := grid.DecodeSurfaceLayer(g, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); !errors.Is(err, grid.ErrLayerSize) {
		t.Fatalf("a square holding 3 was accepted: %v", err)
	}
	if _, err := grid.DecodeSurfaceLayer(g, []byte{0}); !errors.Is(err, grid.ErrLayerSize) {
		t.Fatalf("a short layer was accepted: %v", err)
	}
	var none *grid.SurfaceLayer
	if none.Get(0, 0) != grid.SurfaceFloor || none.Encode() != nil || none.Scaled(2) != nil {
		t.Fatal("a nil layer must read as empty")
	}
}

func TestSurfaceLayerScaled(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 2, Rows: 2}
	l := grid.NewSurfaceLayer(g)
	l.Set(1, 0, grid.SurfaceClimb)
	s := l.Scaled(2)
	for _, sq := range []grid.Square{{Col: 2, Row: 0}, {Col: 3, Row: 0}, {Col: 2, Row: 1}, {Col: 3, Row: 1}} {
		if s.Get(sq.Col, sq.Row) != grid.SurfaceClimb {
			t.Errorf("%v is not climbed after scaling", sq)
		}
	}
	if s.Get(0, 0) != grid.SurfaceFloor || s.Grid().Columns != 4 {
		t.Error("scaling painted the wrong squares or kept the grid")
	}
}

func TestSurfaceCost(t *testing.T) {
	t.Parallel()
	tr := surfaceTerrain()
	at := func(c, r int) grid.Square { return grid.Square{Col: c, Row: r} }
	walker := grid.Mover{Size: grid.SizeMedium}
	cases := []struct {
		name        string
		from, to    grid.Square
		m           grid.Mover
		cost, extra int
		surf        grid.Surface
	}{
		{"floor", at(0, 3), at(4, 3), walker, 200, 0, grid.SurfaceFloor},
		{"all water: twice the length", at(2, 0), at(5, 0), walker, 300, 150, grid.SurfaceWater},
		{"from the shore, 3 of 4 squares swum", at(0, 0), at(4, 0), walker, 350, 150, grid.SurfaceWater},
		{"with a swim speed: nothing extra", at(2, 0), at(5, 0), grid.Mover{Swims: true}, 150, 0, grid.SurfaceFloor},
		{"a climb speed does not swim", at(2, 0), at(5, 0), grid.Mover{Climbs: true}, 300, 150, grid.SurfaceWater},
		{"all climbed", at(2, 1), at(5, 1), walker, 300, 150, grid.SurfaceClimb},
		{"with a climb speed: nothing extra", at(2, 1), at(5, 1), grid.Mover{Climbs: true}, 150, 0, grid.SurfaceFloor},
		{"a swim speed does not climb", at(2, 1), at(5, 1), grid.Mover{Swims: true}, 300, 150, grid.SurfaceClimb},
		{"a flier swims and climbs nothing", at(2, 0), at(5, 0), grid.Mover{Flier: true}, 150, 0, grid.SurfaceFloor},
		{"a diagonal step in water costs its length again", at(1, 0), at(2, 1), walker, 71 * 2, 71, grid.SurfaceClimb},
		{"a crawl on floor costs twice", at(0, 3), at(4, 3), grid.Mover{Crawls: true}, 400, 0, grid.SurfaceFloor},
		{"a crawl in water adds to the swim: 3 per foot", at(2, 0), at(5, 0), grid.Mover{Crawls: true}, 150 * 3, 150, grid.SurfaceWater},
		{"a flier does not crawl", at(0, 3), at(4, 3), grid.Mover{Crawls: true, Flier: true}, 200, 0, grid.SurfaceFloor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mv := tr.Move(tc.from, tc.to, nil, tc.m)
			if mv.Blocked || mv.CostDFt != tc.cost || mv.SurfaceDFt != tc.extra || mv.Surface != tc.surf {
				t.Fatalf("Move = cost %d, surface %d (%d), blocked %v; want cost %d, surface %d (%d)", mv.CostDFt, mv.SurfaceDFt, mv.Surface, mv.Blocked, tc.cost, tc.extra, tc.surf)
			}
		})
	}
}

// The crawl's extra is its own number, apart from the surface's: a crawl through 4 squares of water is the line, the swim and the crawl.
func TestTheCrawlAndTheSwimAreSeparateExtras(t *testing.T) {
	t.Parallel()
	tr := surfaceTerrain()
	mv := tr.Move(grid.Square{Col: 2, Row: 0}, grid.Square{Col: 5, Row: 0}, nil, grid.Mover{Crawls: true})
	if mv.CostDFt != 450 || mv.SurfaceDFt != 150 || mv.CrawlDFt != 150 || mv.Surface != grid.SurfaceWater {
		t.Fatalf("Move = cost %d, swim %d, crawl %d; want 450, 150 and 150", mv.CostDFt, mv.SurfaceDFt, mv.CrawlDFt)
	}
	for _, r := range tr.Reach(grid.Square{Col: 2, Row: 0}, 1000, nil, grid.Mover{Crawls: true}) {
		if r.Square == (grid.Square{Col: 5, Row: 0}) && (r.CrawlDFt != 150 || r.SurfaceDFt != 150) {
			t.Errorf("Reach says swim %d and crawl %d, want 150 and 150", r.SurfaceDFt, r.CrawlDFt)
		}
	}
	if plain := tr.Move(grid.Square{Col: 0, Row: 3}, grid.Square{Col: 4, Row: 3}, nil, grid.Mover{}); plain.CrawlDFt != 0 {
		t.Errorf("a mover that does not crawl has a crawl extra of %d", plain.CrawlDFt)
	}
}

// A crawl over difficult terrain adds the two extras, 3 a foot, as the SRD does (it was 4 when the crawl doubled the whole cost).
func TestCrawlOverDifficultTerrainAddsTheExtras(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 6, Rows: 1}
	rubble := grid.NewLayer(g)
	for col := 1; col <= 4; col++ {
		rubble.Set(col, 0, true)
	}
	tr := grid.Terrain{Grid: g, Difficult: rubble}
	mv := tr.Move(grid.Square{Col: 0}, grid.Square{Col: 4}, nil, grid.Mover{Crawls: true})
	if want := 200 + 200 + 200; mv.CostDFt != want { // 4 squares: the line, the rubble and the crawl
		t.Fatalf("a 20 ft crawl over rubble cost %d, want %d", mv.CostDFt, want)
	}
}

// Move, Reach and MoveUntil are the same cost, square by square, whatever the mover is.
func TestSurfaceCostIsTheSameInMoveReachAndMoveUntil(t *testing.T) {
	t.Parallel()
	tr := surfaceTerrain()
	from := grid.Square{Col: 0, Row: 0}
	for _, m := range []grid.Mover{{}, {Swims: true}, {Climbs: true}, {Crawls: true}, {Swims: true, Climbs: true, Crawls: true}} {
		reach := tr.Reach(from, 1000, nil, m)
		if len(reach) == 0 {
			t.Fatal("nothing reachable")
		}
		for _, r := range reach {
			mv := tr.Move(from, r.Square, nil, m)
			st := tr.MoveUntil(from, r.Square, nil, m, nil, 1<<30)
			if mv.CostDFt != r.CostDFt || mv.SurfaceDFt != r.SurfaceDFt || st.Reached != r.Square || st.CostDFt != r.CostDFt {
				t.Fatalf("%+v to %v: Move %d, Reach %d, MoveUntil %d at %v", m, r.Square, mv.CostDFt, r.CostDFt, st.CostDFt, st.Reached)
			}
		}
	}
}

// The real move stops before the square that the swim makes too dear, as it does for rubble.
func TestMoveUntilStopsWhereTheSwimIsTooDear(t *testing.T) {
	t.Parallel()
	tr := surfaceTerrain()
	st := tr.MoveUntil(grid.Square{Col: 1, Row: 0}, grid.Square{Col: 5, Row: 0}, nil, grid.Mover{}, nil, 150)
	// from the shore: square 2 (cost 50 + 50 extra), square 3 (100 + 100 over 2 of 2)... the budget runs out
	if st.Reason != grid.StopMovement || st.Reached.Col >= 5 || st.CostDFt > 150 {
		t.Fatalf("stopped = %+v", st)
	}
	if st.Reached.Col != 2 {
		t.Fatalf("reached %v, want the first water square (cost 100)", st.Reached)
	}
}

func TestSurfaceValidateChecksTheGrid(t *testing.T) {
	t.Parallel()
	tr := grid.Terrain{Grid: grid.Grid{Columns: 4, Rows: 4}, Surface: grid.NewSurfaceLayer(grid.Grid{Columns: 5, Rows: 5})}
	if !errors.Is(tr.Validate(), grid.ErrBadGrid) {
		t.Fatal("a surface layer of another grid was accepted")
	}
}

func TestJumpPaysNoSurface(t *testing.T) {
	t.Parallel()
	tr := surfaceTerrain()
	mv := tr.Jump(grid.Square{Col: 2, Row: 0}, grid.Square{Col: 5, Row: 0}, nil, grid.Mover{Crawls: true})
	if mv.Blocked || mv.CostDFt != 150 || mv.SurfaceDFt != 0 {
		t.Fatalf("a jump over water cost %d (+%d)", mv.CostDFt, mv.SurfaceDFt)
	}
}
