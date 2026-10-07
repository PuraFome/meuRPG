package maps

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/maps/refimg"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// TestMeasureTheBiggestMapReference measures what a picture made from the biggest map
// (200 x 400 squares) costs the server before the call: the layers, the floor plan, the
// sight of two characters (the union), the players' view as a plan, and the two
// drawings, as playersSeenOf and prepareMap do them, without the database. Run it with
// MEURPG_MEASURE=1 -v (docs/operations.md has the numbers).
func TestMeasureTheBiggestMapReference(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	g := grid.Grid{Columns: 200, Rows: 400}
	walls := grid.NewLayer(g)
	for c := range 200 {
		for r := range 400 {
			if c == 0 || r == 0 || c == 199 || r == 399 || (c%7 == 3 && r%5 == 2) {
				walls.Set(c, r, true)
			}
		}
	}
	walls.Set(100, 200, false)
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	var alloc0 runtime.MemStats
	runtime.ReadMemStats(&alloc0)
	base := heap()
	start := time.Now()

	set := loadLayers(mapsdb.MapLayer{Walls: walls.Encode()}, g)
	solid := floorPlan(set, g)
	lit, err := vision.Compile(vision.Scene{Grid: g, Walls: set.walls, Doors: set.doors, Base: grid.Bright, Painted: set.light})
	if err != nil {
		t.Fatal(err)
	}
	view := lit.Union(
		lit.See(vision.Viewer{At: grid.Square{Col: 100, Row: 200}}),
		lit.See(vision.Viewer{At: grid.Square{Col: 120, Row: 220}}),
	)
	seen := make([]bool, g.Squares())
	for i := range seen {
		seen[i] = view.Seen(grid.Square{Col: i % g.Columns, Row: i / g.Columns})
	}
	peak := heap() - base
	players := &playersSeen{seen: seen, markers: []refimg.Marker{{Col: 100, Row: 200, Party: true}, {Col: 121, Row: 220}}}
	sub := &subject{g: g, imgW: 1200, imgH: 2400, set: set, solid: solid}
	one, err := (&Service{}).referenceOf("scene", sub, players, refimg.Side)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := (&Service{}).referenceOf("texture", sub, players, refimg.Side)
	if err != nil {
		t.Fatal(err)
	}
	var alloc1 runtime.MemStats
	runtime.ReadMemStats(&alloc1)
	runtime.KeepAlive(lit)
	t.Logf("200 x 400: layers, plan, sight and the two drawings in %v, %.1f MB allocated in all; live after the sight and the plan: %.2f MB; players' view drawing: %d squares seen, PNG %d kB; textured map drawing: %s, PNG %d kB",
		time.Since(start).Round(time.Millisecond), float64(alloc1.TotalAlloc-alloc0.TotalAlloc)/1e6, float64(peak)/1e6, one.seen, len(one.png)/1000, whole.pad.Ratio, len(whole.png)/1000)
}
