package vision_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

var updateVisionSweep = flag.Bool("update", false, "rewrite testdata/see_sweep.sha256 from the current code")

// TestSeeSweepCharacterization freezes See (and so the walls it marks seen)
// from every open square of seeded maps, dense and sparse in walls, in the
// dark, in dim light and in daylight, with walls on the edges and doors of
// each kind, for a viewer with darkvision and one without. The text is far
// too long to keep: its digest is in testdata.
func TestSeeSweepCharacterization(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 18, Rows: 12}
	var b strings.Builder
	for _, perMille := range []int{120, 350} {
		for _, base := range []grid.Light{grid.Dark, grid.Dim, grid.Bright} {
			scene, _ := benchScene(g, perMille, 3, 0, base)
			scene.Doors = grid.NewDoorLayer(g)
			for i, d := range []grid.Door{grid.DoorOpen, grid.DoorClosed, grid.DoorLocked, grid.DoorBarred, grid.DoorSecret} {
				scene.Doors.Set(3+3*i, 6, d)
			}
			for col := range g.Columns {
				scene.Walls.Set(col, 0, true)
			}
			lit, err := vision.Compile(scene)
			if err != nil {
				t.Fatal(err)
			}
			for row := range g.Rows {
				for col := range g.Columns {
					at := grid.Square{Col: col, Row: row}
					for _, senses := range []vision.Senses{{}, {DarkvisionFt: 30, BlindsightFt: 10}} {
						v := lit.See(vision.Viewer{At: at, Senses: senses})
						fmt.Fprintf(&b, "%d %d %v %v %d ", perMille, base, at, senses, v.Count())
						for r := range g.Rows {
							for c := range g.Columns {
								b.WriteByte('0' + byte(v.At(grid.Square{Col: c, Row: r})))
							}
						}
						b.WriteByte('\n')
					}
				}
			}
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	got := fmt.Sprintf("%s %d\n", hex.EncodeToString(sum[:]), strings.Count(b.String(), "\n"))
	path := filepath.Join("testdata", "see_sweep.sha256")
	if *updateVisionSweep {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("the See sweep changed: digest %q, want %q", strings.TrimSpace(got), strings.TrimSpace(string(want)))
	}
}
