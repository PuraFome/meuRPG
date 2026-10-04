package maps

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// The CPU a filtered GetMap spends on one player (MR-036): their view, the
// per-square states, and the filtered layers, for the cave and for a 60 x 40 map
// with six players and three lights. The reads' other costs (the database) are
// measured by TestFogGetMapTiming. "warm" is a read after the scene and the views
// are cached (the usual one); "cold" has to see again, as after any write that
// moved a character.

func benchScene(g grid.Grid, walls *grid.Layer, lights ...vision.Source) (*sight, *grid.Layer, []string) {
	lit, err := vision.Compile(vision.Scene{Grid: g, Walls: walls, Base: grid.Dark, Sources: lights})
	if err != nil {
		panic(err)
	}
	sg := &sight{g: g, entry: &litEntry{lit: lit, views: map[vision.Viewer]*vision.View{}}, stands: map[string]grid.Square{}}
	var users []string
	for i := range 6 {
		id := fmt.Sprintf("user-%d", i)
		sq := grid.Square{Col: 3 + 7*i%(g.Columns-6), Row: 2 + 5*i%(g.Rows-4)}
		sg.members = append(sg.members, link.PartyMember{CharacterID: "char-" + id, UserID: id, Senses: vision.Senses{DarkvisionFt: 60 * (i % 2)}})
		sg.stands["char-"+id] = sq
		users = append(users, id)
	}
	return sg, walls, users
}

func runFiltered(b *testing.B, sg *sight, walls *grid.Layer, users []string, cold bool) {
	b.ReportAllocs()
	terrain := grid.NewLayer(sg.g)
	set := layerSet{terrain: terrain, walls: walls, cover: grid.NewCoverLayer(sg.g)}
	memory := grid.NewLayer(sg.g)
	for range b.N {
		if cold {
			clear(sg.entry.views)
		}
		now, onMap := sg.viewOf(users[0])
		pv := newPlayerView(sg.g, now, memory, onMap)
		_ = pv.pack()
		filtered := filterLayers(set, pv)
		_ = filtered.walls.Encode()
	}
}

func BenchmarkFilteredMap(b *testing.B) {
	cave := grid.NewLayer(caveGrid)
	for row, line := range caveWalls {
		for col, ch := range line {
			if ch == '#' {
				cave.Set(col, row, true)
			}
		}
	}
	big := grid.Grid{Columns: 60, Rows: 40}
	room := grid.NewLayer(big)
	for col := range big.Columns {
		room.Set(col, 0, true)
		room.Set(col, big.Rows-1, true)
	}
	for row := range big.Rows {
		room.Set(0, row, true)
		room.Set(big.Columns-1, row, true)
	}
	for col := 6; col < big.Columns-6; col += 6 {
		for row := 4; row < big.Rows-4; row += 6 {
			room.Set(col, row, true) // pillars
		}
	}
	torch := vision.Source{BrightFt: 20, DimFt: 20}
	scenes := []struct {
		name  string
		g     grid.Grid
		walls *grid.Layer
		src   []vision.Source
	}{
		{"cave", caveGrid, cave, []vision.Source{{At: grid.Square{Col: 19, Row: 4}, BrightFt: 20, DimFt: 20}}},
		{"60x40 with 6 players and 3 lights", big, room, []vision.Source{
			{At: grid.Square{Col: 10, Row: 10}, BrightFt: torch.BrightFt, DimFt: torch.DimFt},
			{At: grid.Square{Col: 30, Row: 20}, BrightFt: 40, DimFt: 40},
			{At: grid.Square{Col: 50, Row: 30}, BrightFt: 20, DimFt: 20},
		}},
	}
	for _, sc := range scenes {
		sg, walls, users := benchScene(sc.g, sc.walls, sc.src...)
		b.Run(sc.name+"/warm", func(b *testing.B) { runFiltered(b, sg, walls, users, false) })
		b.Run(sc.name+"/cold", func(b *testing.B) { runFiltered(b, sg, walls, users, true) })
	}
}

// TestSceneMemory measures what a compiled scene of the largest grid (200 x 400)
// and one of its views take, the numbers behind the cache's size (litCacheSize,
// viewsPerScene; CONTRIBUTING, "As medidas da névoa"). Run it with -v.
func TestSceneMemory(t *testing.T) {
	g := grid.Grid{Columns: 200, Rows: 400}
	walls := grid.NewLayer(g)
	for c := 0; c < 200; c += 7 {
		for r := 0; r < 400; r += 5 {
			walls.Set(c, r, true)
		}
	}
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	a := heap()
	lit, err := vision.Compile(vision.Scene{Grid: g, Walls: walls, Sources: []vision.Source{{At: grid.Square{Col: 5, Row: 5}, BrightFt: 60, DimFt: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	b := heap()
	view := lit.See(vision.Viewer{At: grid.Square{Col: 3, Row: 3}, Senses: vision.Senses{DarkvisionFt: 60}})
	c := heap()
	runtime.KeepAlive(lit)
	runtime.KeepAlive(view)
	scene, one := (b-a)/1024, (c-b)/1024
	worst := (uint64(litCacheSize)*scene + uint64(litCacheSize*viewsPerScene)*one) / 1024
	t.Logf("a compiled 200 x 400 scene takes %d kB and each view %d kB: at most %d MB for %d scenes of %d views", scene, one, worst, litCacheSize, viewsPerScene)
}
