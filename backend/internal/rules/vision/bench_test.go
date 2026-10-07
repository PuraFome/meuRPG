package vision_test

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// benchScene is a map of g with walls on a share of its squares (per mille),
// some lights and some viewers, from a fixed seed so every run is the same
// work. base is the map's base light: Dark is a dungeon, Bright an open field,
// the hard case, where every square is lit and every one needs a line.
func benchScene(g grid.Grid, wallsPerMille, lights, viewers int, base grid.Light) (vision.Scene, []vision.Viewer) {
	rng := &xorshift{state: 0x9E3779B97F4A7C15}
	walls := grid.NewLayer(g)
	for row := range g.Rows {
		for col := range g.Columns {
			if rng.intn(1000) < wallsPerMille {
				walls.Set(col, row, true)
			}
		}
	}
	free := func() grid.Square {
		for {
			sq := grid.Square{Col: rng.intn(g.Columns), Row: rng.intn(g.Rows)}
			if !walls.Get(sq.Col, sq.Row) {
				return sq
			}
		}
	}
	scene := vision.Scene{Grid: g, Walls: walls, Base: base}
	for range lights {
		scene.Sources = append(scene.Sources, vision.Source{At: free(), BrightFt: 20, DimFt: 20})
	}
	var vs []vision.Viewer
	for range viewers {
		vs = append(vs, vision.Viewer{At: free(), Senses: vision.Senses{DarkvisionFt: 60}})
	}
	return scene, vs
}

// run is what the server does for one change: work out the light and what each
// viewer sees, and the group's union.
func run(scene vision.Scene, vs []vision.Viewer) int {
	lit, err := vision.Compile(scene)
	if err != nil {
		panic(err)
	}
	views := make([]*vision.View, len(vs))
	for i, v := range vs {
		views[i] = lit.See(v)
	}
	return lit.Union(views...).Count()
}

// The budget of the slice: the cave (24 x 16, 4 viewers, 2 lights) well under
// 1 ms; 60 x 40 with 6 viewers and 10 lights under 10 ms; the largest grid
// (200 x 400) with 6 viewers and 20 lights under 300 ms.
func BenchmarkCave(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 24, Rows: 16}, 350, 2, 4, grid.Dark)
	for b.Loop() {
		run(scene, vs)
	}
}

func BenchmarkMedium(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 60, Rows: 40}, 200, 10, 6, grid.Dark)
	for b.Loop() {
		run(scene, vs)
	}
}

func BenchmarkLargest(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 200, Rows: 400}, 200, 20, 6, grid.Dark)
	for b.Loop() {
		run(scene, vs)
	}
}

// The hard cases: every square lit (an open field in daylight), with few walls,
// so each viewer needs a line to nearly every square.
func BenchmarkLargestInDaylightFewWalls(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 200, Rows: 400}, 10, 20, 6, grid.Bright)
	for b.Loop() {
		run(scene, vs)
	}
}

func BenchmarkLargestInDaylightNoWalls(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 200, Rows: 400}, 0, 20, 6, grid.Bright)
	for b.Loop() {
		run(scene, vs)
	}
}

func BenchmarkMediumInDaylight(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 60, Rows: 40}, 30, 10, 6, grid.Bright)
	for b.Loop() {
		run(scene, vs)
	}
}

// xorshift is a small deterministic generator, so a benchmark does the same
// work on every run and every machine.
type xorshift struct{ state uint64 }

func (r *xorshift) intn(n int) int {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return int(uint32(r.state>>32)) % n
}

// A 200 x 400 field in daylight with a wall around it and five pillars: the
// case where every line is long and almost none meets a wall.
func BenchmarkLargestInDaylightBorderAndPillars(b *testing.B) {
	g := grid.Grid{Columns: 200, Rows: 400}
	scene, vs := benchScene(g, 0, 20, 6, grid.Bright)
	for col := range g.Columns {
		scene.Walls.Set(col, 0, true)
		scene.Walls.Set(col, g.Rows-1, true)
	}
	for row := range g.Rows {
		scene.Walls.Set(0, row, true)
		scene.Walls.Set(g.Columns-1, row, true)
	}
	for i := range 5 {
		scene.Walls.Set(40+i*30, 80+i*60, true)
	}
	for i := range vs {
		vs[i].At = grid.Square{Col: 10 + i*30, Row: 20 + i*60}
	}
	for b.Loop() {
		run(scene, vs)
	}
}

// The same field with 1 wall in 1000 squares at random.
func BenchmarkLargestInDaylightSparseWalls(b *testing.B) {
	scene, vs := benchScene(grid.Grid{Columns: 200, Rows: 400}, 1, 20, 6, grid.Bright)
	for b.Loop() {
		run(scene, vs)
	}
}
