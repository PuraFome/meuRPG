package vision_test

import (
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

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

// caveScene is the cave in the dark with the guard room's torch at (19, 4):
// 6 m of bright light and 6 m more of dim light (20 + 20 ft). Only # blocks
// light and sight: the crates and the column are low or thin.
func caveScene(extra ...vision.Source) vision.Scene {
	walls := grid.NewLayer(caveGrid)
	for row, line := range caveRows {
		for col, ch := range line {
			if ch == '#' {
				walls.Set(col, row, true)
			}
		}
	}
	return vision.Scene{
		Grid:    caveGrid,
		Walls:   walls,
		Base:    grid.Dark,
		Sources: append([]vision.Source{{At: grid.Square{Col: 19, Row: 4}, BrightFt: 20, DimFt: 20}}, extra...),
	}
}

var (
	toren     = grid.Square{Col: 6, Row: 7}
	pensantus = grid.Square{Col: 5, Row: 8}
	brisa     = grid.Square{Col: 4, Row: 7}
	salvia    = grid.Square{Col: 3, Row: 8}
)

// darkvision of a gnome and a half-elf: 18 m.
var darkvision = vision.Senses{DarkvisionFt: 60}

func picture(v *vision.View, g grid.Grid, own grid.Square) []string {
	rows := make([]string, g.Rows)
	for row := 0; row < g.Rows; row++ {
		var b strings.Builder
		for col := 0; col < g.Columns; col++ {
			sq := grid.Square{Col: col, Row: row}
			ch := byte(' ')
			switch {
			case sq == own:
				ch = '@'
			case v.At(sq) == vision.SeenBright:
				ch = 'B'
			case v.At(sq) == vision.SeenDim:
				ch = 'd'
			case v.At(sq) == vision.SeenGrey:
				ch = 'g'
			case v.At(sq) == vision.SeenWall:
				ch = '#'
			}
			b.WriteByte(ch)
		}
		rows[row] = b.String()
	}
	return rows
}

func diff(t *testing.T, name string, got, want []string) {
	t.Helper()
	for row := range want {
		if got[row] != want[row] {
			t.Errorf("%s: row %d\n got  %q\n want %q", name, row, got[row], want[row])
		}
	}
}

func TestVisionAgainstTheCave(t *testing.T) {
	t.Parallel()
	dark := compile(t, caveScene())
	// Toren carries a torch in the second scene (E9-04): 6 m + 6 m.
	torch := compile(t, caveScene(vision.Source{At: toren, BrightFt: 20, DimFt: 20}))
	cases := []struct {
		key    string
		lit    *vision.Lit
		viewer vision.Viewer
	}{
		{"pensantus", dark, vision.Viewer{At: pensantus, Senses: darkvision}},
		{"pensantus-tocha", torch, vision.Viewer{At: pensantus, Senses: darkvision}},
		{"salvia-tocha", torch, vision.Viewer{At: salvia, Senses: darkvision}},
		{"toren", dark, vision.Viewer{At: toren}},
		{"brisa", dark, vision.Viewer{At: brisa}},
		{"salvia", dark, vision.Viewer{At: salvia, Senses: darkvision}},
		{"salvia-lobo", dark, vision.Viewer{At: salvia}}, // Wild Shape: a wolf has no darkvision here
		{"toren-tocha", torch, vision.Viewer{At: toren}},
		{"brisa-tocha", torch, vision.Viewer{At: brisa}},
	}
	for _, c := range cases {
		diff(t, c.key, picture(c.lit.See(c.viewer), caveGrid, c.viewer.At), caveVision[c.key])
	}
}

func TestOwnSquareIsAlwaysKnown(t *testing.T) {
	t.Parallel()
	lit := compile(t, caveScene())
	v := lit.See(vision.Viewer{At: toren}) // no darkvision, standing in the dark
	if got := v.At(toren); got != vision.SeenGrey {
		t.Errorf("own square in the dark = %d, want grey", got)
	}
	if !v.Seen(toren) || v.Count() == 0 {
		t.Error("a viewer always knows its own square")
	}
	if got := lit.See(vision.Viewer{At: grid.Square{Col: 19, Row: 4}}).At(grid.Square{Col: 19, Row: 4}); got != vision.SeenBright {
		t.Errorf("own square under the torch = %d, want bright", got)
	}
	if got := lit.See(vision.Viewer{At: grid.Square{Col: -1, Row: 4}}).Count(); got != 0 {
		t.Errorf("a viewer outside the grid sees %d squares", got)
	}
}

func TestSensesBeyondDarkvision(t *testing.T) {
	t.Parallel()
	lit := compile(t, caveScene())
	at := vision.Viewer{At: grid.Square{Col: 10, Row: 7}}
	far := grid.Square{Col: 12, Row: 7} // 2 squares = 10 ft away, in the dark, in the open

	if lit.See(at).At(far) != vision.Unseen {
		t.Fatal("without senses the dark is not seen")
	}
	// Blindsight within its range sees regardless of the light, in grey; beyond it, no.
	at.Senses = vision.Senses{BlindsightFt: 10}
	if got := lit.See(at).At(far); got != vision.SeenGrey {
		t.Errorf("blindsight at 10 ft = %d, want grey", got)
	}
	if got := lit.See(at).At(grid.Square{Col: 13, Row: 7}); got != vision.Unseen {
		t.Errorf("blindsight 15 ft away = %d, want unseen", got)
	}
	// But not through a wall.
	at = vision.Viewer{At: grid.Square{Col: 10, Row: 10}, Senses: vision.Senses{BlindsightFt: 30}}
	if got := lit.See(at).At(grid.Square{Col: 10, Row: 7}); got != vision.Unseen {
		t.Errorf("blindsight through rock = %d, want unseen", got)
	}
	// Truesight sees darkness as bright, within range.
	at = vision.Viewer{At: grid.Square{Col: 10, Row: 7}, Senses: vision.Senses{TruesightFt: 15}}
	if got := lit.See(at).At(far); got != vision.SeenBright {
		t.Errorf("truesight in the dark = %d, want bright", got)
	}
	if got := lit.See(at).At(grid.Square{Col: 14, Row: 7}); got != vision.Unseen {
		t.Errorf("truesight out of range = %d, want unseen", got)
	}
	// Darkvision, within its range, sees dim light as bright (no -5) and
	// darkness as dim (grey); beyond its range dim light stays dim.
	at = vision.Viewer{At: grid.Square{Col: 14, Row: 7}, Senses: vision.Senses{DarkvisionFt: 30}}
	v := lit.See(at)
	if got := v.At(grid.Square{Col: 16, Row: 7}); got != vision.SeenBright { // dim light, 15 ft away
		t.Errorf("dim light within darkvision = %d, want bright", got)
	}
	if got := v.At(grid.Square{Col: 12, Row: 7}); got != vision.SeenGrey { // darkness, 10 ft away
		t.Errorf("darkness within darkvision = %d, want grey", got)
	}
	if got := lit.See(vision.Viewer{At: grid.Square{Col: 14, Row: 7}}).At(grid.Square{Col: 16, Row: 7}); got != vision.SeenDim {
		t.Errorf("dim light with no darkvision = %d, want dim", got)
	}
	// Truesight sees dim light as bright too.
	at.Senses = vision.Senses{TruesightFt: 30}
	if got := lit.See(at).At(grid.Square{Col: 16, Row: 7}); got != vision.SeenBright {
		t.Errorf("dim light within truesight = %d, want bright", got)
	}
}

func TestLightIsBlockedByWalls(t *testing.T) {
	t.Parallel()
	lit := compile(t, caveScene())
	for _, c := range []struct {
		sq   grid.Square
		want grid.Light
	}{
		{grid.Square{Col: 19, Row: 4}, grid.Bright}, // the torch itself
		{grid.Square{Col: 22, Row: 4}, grid.Bright}, // 3 squares (15 ft)
		{grid.Square{Col: 19, Row: 8}, grid.Bright}, // 4 squares down: exactly 20 ft is still bright
		{grid.Square{Col: 19, Row: 9}, grid.Dim},    // 5 squares: 25 ft
		{grid.Square{Col: 23, Row: 4}, grid.Dark},   // a wall is not lit
		{grid.Square{Col: 16, Row: 7}, grid.Dim},    // in the corridor mouth, at 4,2 squares
		{grid.Square{Col: 12, Row: 7}, grid.Dark},   // beyond 8 squares
		{grid.Square{Col: 6, Row: 7}, grid.Dark},
		{grid.Square{Col: -3, Row: 7}, grid.Dark},
	} {
		if got := lit.LightAt(c.sq); got != c.want {
			t.Errorf("light at %v = %d, want %d", c.sq, got, c.want)
		}
	}
	// Painted and base light: a lit room needs no source, and a source does not
	// make a painted-dark square darker.
	painted := grid.NewLightLayer(caveGrid)
	painted.Set(2, 7, grid.Bright)
	painted.Set(19, 5, grid.Dark)
	scene := caveScene()
	scene.Painted, scene.Base = painted, grid.Dim
	l2 := compile(t, scene)
	if l2.LightAt(grid.Square{Col: 2, Row: 7}) != grid.Bright {
		t.Error("a painted square takes its own light")
	}
	if l2.LightAt(grid.Square{Col: 3, Row: 7}) != grid.Dim {
		t.Error("an unpainted square takes the base light")
	}
	if l2.LightAt(grid.Square{Col: 19, Row: 5}) != grid.Bright {
		t.Error("a source lights a square painted dark: the brightest that reaches it counts")
	}
	// A wall between the source and the square stops the light, but painted
	// light does not need a line.
	if l2.LightAt(grid.Square{Col: 12, Row: 12}) != grid.Dim {
		t.Error("the base light reaches everywhere")
	}
}

func TestUnionOfViewers(t *testing.T) {
	t.Parallel()
	lit := compile(t, caveScene())
	a := lit.See(vision.Viewer{At: pensantus, Senses: darkvision})
	b := lit.See(vision.Viewer{At: toren})
	u := lit.Union(a, b)
	for row := 0; row < caveGrid.Rows; row++ {
		for col := 0; col < caveGrid.Columns; col++ {
			sq := grid.Square{Col: col, Row: row}
			if want := max(a.At(sq), b.At(sq)); u.At(sq) != want {
				t.Fatalf("union at %v = %d, want %d", sq, u.At(sq), want)
			}
		}
	}
	if u.Count() < a.Count() || u.Count() < b.Count() {
		t.Error("the union sees at least what each sees")
	}
	if lit.Union().Count() != 0 || lit.Union(nil, a).Count() != a.Count() {
		t.Error("the union of none sees nothing, and nil views are skipped")
	}
}

func TestPassivePenalty(t *testing.T) {
	t.Parallel()
	for s, want := range map[vision.State]int{
		vision.Unseen: 0, vision.SeenWall: 0, vision.SeenBright: 0,
		vision.SeenDim: -5, vision.SeenGrey: -5,
	} {
		if got := vision.PassivePenalty(s); got != want {
			t.Errorf("PassivePenalty(%d) = %d, want %d", s, got, want)
		}
	}
}

func TestNoWalls(t *testing.T) {
	t.Parallel()
	// Without walls, sight is limited only by light (Q68).
	g := grid.Grid{Columns: 12, Rows: 12}
	lit := compile(t, vision.Scene{Grid: g, Base: grid.Dark, Sources: []vision.Source{{At: grid.Square{Col: 6, Row: 6}, BrightFt: 10, DimFt: 10}}})
	v := lit.See(vision.Viewer{At: grid.Square{Col: 0, Row: 0}})
	if got := v.At(grid.Square{Col: 6, Row: 6}); got != vision.SeenBright {
		t.Errorf("the torch is seen from afar, got %d", got)
	}
	if got := v.At(grid.Square{Col: 6, Row: 11}); got != vision.Unseen {
		t.Errorf("dark squares are not seen, got %d", got)
	}
}

func compile(t testing.TB, s vision.Scene) *vision.Lit {
	t.Helper()
	l, err := vision.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestCompileRefusesBadScenesAndClampsLights(t *testing.T) {
	t.Parallel()
	for _, g := range []grid.Grid{{}, {Columns: -1, Rows: 3}, {Columns: 300, Rows: 3}} {
		if _, err := vision.Compile(vision.Scene{Grid: g}); err == nil {
			t.Errorf("Compile accepted the grid %v", g)
		}
	}
	g := grid.Grid{Columns: 10, Rows: 10}
	other := grid.Grid{Columns: 5, Rows: 5}
	if _, err := vision.Compile(vision.Scene{Grid: g, Walls: grid.NewLayer(other)}); err == nil {
		t.Error("Compile accepted walls of another grid")
	}
	if _, err := vision.Compile(vision.Scene{Grid: g, Painted: grid.NewLightLayer(other)}); err == nil {
		t.Error("Compile accepted painted light of another grid")
	}
	// An absurd radius is clamped, not run to the end of the world.
	huge := vision.Source{At: grid.Square{Col: 5, Row: 5}, BrightFt: 1 << 40, DimFt: 1 << 40}
	if l := compile(t, vision.Scene{Grid: g, Sources: []vision.Source{huge, {At: grid.Square{Col: -4, Row: 99}, BrightFt: 20}}}); l.LightAt(grid.Square{Col: 9, Row: 9}) != grid.Bright {
		t.Error("a clamped light still lights the map")
	}
}

// TestCanSeeMatchesSee: the one-square question gives, for every square of the cave
// and every viewer of the drawings, what the whole view says: seen (in grey or
// better), and a wall never a creature's place.
func TestCanSeeMatchesSee(t *testing.T) {
	t.Parallel()
	torch := compile(t, caveScene(vision.Source{At: toren, BrightFt: 20, DimFt: 20}))
	for _, v := range []vision.Viewer{
		{At: pensantus, Senses: darkvision},
		{At: toren},
		{At: brisa},
		{At: salvia, Senses: vision.Senses{BlindsightFt: 30}},
		{At: toren, Senses: vision.Senses{TruesightFt: 60}},
	} {
		view := torch.See(v)
		for row := 0; row < caveGrid.Rows; row++ {
			for col := 0; col < caveGrid.Columns; col++ {
				sq := grid.Square{Col: col, Row: row}
				if want := view.At(sq) >= vision.SeenGrey; torch.CanSee(v, sq) != want {
					t.Fatalf("viewer %v, square %v: CanSee = %v, See = %v", v.At, sq, !want, view.At(sq))
				}
			}
		}
	}
	if torch.CanSee(vision.Viewer{At: grid.Square{Col: -1, Row: 0}}, toren) || torch.CanSee(vision.Viewer{At: toren}, grid.Square{Col: 99, Row: 0}) {
		t.Errorf("a square outside the grid sees or is seen")
	}
}
