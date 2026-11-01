package vision_test

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// An open, bright strip 12 squares wide and 5 tall: the viewer at (1,2) looks east.
var stripGrid = grid.Grid{Columns: 12, Rows: 5}

func stripScene(clouds, dark []grid.Square) vision.Scene {
	c, d := grid.NewLayer(stripGrid), grid.NewLayer(stripGrid)
	for _, sq := range clouds {
		c.Set(sq.Col, sq.Row, true)
	}
	for _, sq := range dark {
		d.Set(sq.Col, sq.Row, true)
	}
	return vision.Scene{Grid: stripGrid, Base: grid.Bright, Clouds: c, MagicDark: d}
}

func column(col int) []grid.Square {
	var out []grid.Square
	for row := range stripGrid.Rows {
		out = append(out, grid.Square{Col: col, Row: row})
	}
	return out
}

func mustCompile(t *testing.T, s vision.Scene) *vision.Lit {
	t.Helper()
	lit, err := vision.Compile(s)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return lit
}

var viewer = grid.Square{Col: 1, Row: 2}

// A line that crosses a cloud square is blocked: the cloud and what is behind it are not seen, what is before it is.
func TestACloudHidesItselfAndWhatIsBehindIt(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene(column(5), nil))
	v := lit.See(vision.Viewer{At: viewer})
	for _, tc := range []struct {
		col  int
		want bool
	}{{3, true}, {4, true}, {5, false}, {6, false}, {10, false}} {
		if got := v.Seen(grid.Square{Col: tc.col, Row: 2}); got != tc.want {
			t.Errorf("column %d seen = %v, want %v", tc.col, got, tc.want)
		}
	}
	if lit.CanSee(vision.Viewer{At: viewer}, grid.Square{Col: 8, Row: 2}) {
		t.Error("CanSee behind the cloud = true, want false")
	}
}

// A viewer inside a cloud sees only its own square.
func TestAViewerInsideACloudSeesOnlyItsOwnSquare(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene(column(1), nil))
	v := lit.See(vision.Viewer{At: viewer})
	if n := v.Count(); n != 1 || !v.Seen(viewer) {
		t.Errorf("seen squares = %d (own %v), want only the own square", n, v.Seen(viewer))
	}
}

// Blindsight sees through a cloud within its range, and no further.
func TestBlindsightSeesThroughACloudWithinItsRange(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene(column(3), nil))
	if lit.See(vision.Viewer{At: viewer}).Seen(grid.Square{Col: 5, Row: 2}) {
		t.Error("control: a square behind the cloud is seen without blindsight")
	}
	v := lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{BlindsightFt: 20}})
	if !v.Seen(grid.Square{Col: 5, Row: 2}) {
		t.Error("a square behind the cloud inside the blindsight is unseen")
	}
	if v.Seen(grid.Square{Col: 8, Row: 2}) {
		t.Error("a square behind the cloud beyond the blindsight is seen")
	}
}

// A cloud is not a wall: no wall of it is drawn, and a square the cloud holds is not a wall for the squares around it.
func TestACloudIsNotDrawnAsAWall(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene([]grid.Square{{Col: 5, Row: 2}}, nil))
	v := lit.See(vision.Viewer{At: viewer})
	if st := v.At(grid.Square{Col: 5, Row: 2}); st != vision.Unseen {
		t.Errorf("state of the cloud square = %v, want Unseen (never SeenWall)", st)
	}
	if !v.Seen(grid.Square{Col: 6, Row: 0}) {
		t.Error("a square beside the one-square cloud, off the line, is unseen")
	}
}

// Magical darkness is dark whatever lights it, and darkvision does not see in it.
func TestMagicalDarknessIsDarkToLightAndToDarkvision(t *testing.T) {
	t.Parallel()
	dark := []grid.Square{{Col: 6, Row: 2}}
	s := stripScene(nil, dark)
	s.Sources = []vision.Source{{At: grid.Square{Col: 6, Row: 2}, BrightFt: 20, DimFt: 20}}
	lit := mustCompile(t, s)
	if got := lit.LightAt(dark[0]); got != grid.Dark {
		t.Errorf("light in the darkness = %v, want Dark", got)
	}
	for _, tc := range []struct {
		name   string
		senses vision.Senses
		want   vision.State
	}{
		{"no sense", vision.Senses{}, vision.Unseen},
		{"darkvision", vision.Senses{DarkvisionFt: 60}, vision.Unseen},
		{"blindsight", vision.Senses{BlindsightFt: 60}, vision.SeenGrey},
		{"truesight", vision.Senses{TruesightFt: 60}, vision.SeenBright},
	} {
		if got := lit.See(vision.Viewer{At: viewer, Senses: tc.senses}).At(dark[0]); got != tc.want {
			t.Errorf("%s: state = %v, want %v", tc.name, got, tc.want)
		}
	}
	if st := lit.See(vision.Viewer{At: viewer}).At(grid.Square{Col: 7, Row: 2}); st != vision.SeenBright {
		t.Errorf("the square past the darkness = %v, want SeenBright (darkness blocks no line)", st)
	}
}

// A viewer standing in the darkness with darkvision is in the dark; with truesight it is not.
func TestInTheDarkInMagicalDarkness(t *testing.T) {
	t.Parallel()
	at := grid.Square{Col: 6, Row: 2}
	lit := mustCompile(t, stripScene(nil, []grid.Square{at}))
	for _, tc := range []struct {
		senses vision.Senses
		want   bool
	}{{vision.Senses{}, true}, {vision.Senses{DarkvisionFt: 60}, true}, {vision.Senses{TruesightFt: 30}, false}} {
		if got := lit.InTheDark(vision.Viewer{At: at, Senses: tc.senses}); got != tc.want {
			t.Errorf("InTheDark(%+v) = %v, want %v", tc.senses, got, tc.want)
		}
	}
}

// A layer sized for another grid is refused.
func TestCompileRefusesZoneLayersOfAnotherGrid(t *testing.T) {
	t.Parallel()
	other := grid.NewLayer(grid.Grid{Columns: 3, Rows: 3})
	for _, s := range []vision.Scene{{Grid: stripGrid, Clouds: other}, {Grid: stripGrid, MagicDark: other}} {
		if _, err := vision.Compile(s); err == nil {
			t.Error("Compile() error = nil, want ErrBadGrid")
		}
	}
}

// Devil's Sight sees in Darkness, magical or not, within its range, and in no further square; darkvision does not see in the Darkness.
func TestDevilsSightSeesInDarknessWithinItsRange(t *testing.T) {
	t.Parallel()
	magic := []grid.Square{{Col: 4, Row: 2}, {Col: 7, Row: 2}}
	lit := mustCompile(t, stripScene(nil, magic))
	devils := lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 15}})
	if got := devils.At(magic[0]); got != vision.SeenBright {
		t.Errorf("Devil's Sight, Darkness in range: state = %v, want SeenBright", got)
	}
	if devils.Seen(magic[1]) {
		t.Error("Devil's Sight, Darkness beyond the range: seen")
	}
	if lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DarkvisionFt: 120}}).Seen(magic[0]) {
		t.Error("control: darkvision sees in the Darkness")
	}
	if !lit.CanSee(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 15}}, magic[0]) {
		t.Error("CanSee in the Darkness with Devil's Sight = false")
	}
}

// Devil's Sight also sees the ordinary dark within its range.
func TestDevilsSightSeesTheOrdinaryDark(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, vision.Scene{Grid: stripGrid, Base: grid.Dark})
	at := grid.Square{Col: 4, Row: 2}
	if lit.See(vision.Viewer{At: viewer}).Seen(at) {
		t.Error("control: a viewer without senses sees in the dark")
	}
	if !lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 15}}).Seen(at) {
		t.Error("Devil's Sight does not see the dark in range")
	}
	if lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 15}}).Seen(grid.Square{Col: 8, Row: 2}) {
		t.Error("Devil's Sight sees the dark beyond its range")
	}
}

// Devil's Sight sees through no cloud.
func TestDevilsSightDoesNotSeeThroughACloud(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene(column(3), nil))
	v := lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 120}})
	for _, col := range []int{3, 5, 8} {
		if v.Seen(grid.Square{Col: col, Row: 2}) {
			t.Errorf("column %d seen through a cloud with Devil's Sight", col)
		}
	}
}

// A viewer with Devil's Sight standing in a Darkness is not in the dark.
func TestInTheDarkWithDevilsSight(t *testing.T) {
	t.Parallel()
	at := grid.Square{Col: 6, Row: 2}
	lit := mustCompile(t, stripScene(nil, []grid.Square{at}))
	if lit.InTheDark(vision.Viewer{At: at, Senses: vision.Senses{DevilsSightFt: 120}}) {
		t.Error("InTheDark with Devil's Sight in a Darkness = true, want false")
	}
	if !lit.InTheDark(vision.Viewer{At: at, Senses: vision.Senses{DarkvisionFt: 120}}) {
		t.Error("control: darkvision in a Darkness is not in the dark")
	}
}

// Devil's Sight does not lift dim light: a dim square stays dim within its range, with the passive penalty it carries.
func TestDevilsSightLeavesDimLightDim(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, vision.Scene{Grid: stripGrid, Base: grid.Dim})
	at := grid.Square{Col: 4, Row: 2}
	st := lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 120}}).At(at)
	if st != vision.SeenDim {
		t.Errorf("Devil's Sight, dim square in range: state = %v, want SeenDim", st)
	}
	if got, want := vision.PassivePenalty(st), vision.PassivePenalty(lit.See(vision.Viewer{At: viewer}).At(at)); got != want || got == 0 {
		t.Errorf("passive penalty with Devil's Sight = %d, without = %d, want the same and not zero", got, want)
	}
}

// A viewer standing inside a Darkness with Devil's Sight sees its surroundings within range, in the Darkness or out of it.
func TestDevilsSightSeesFromInsideADarkness(t *testing.T) {
	t.Parallel()
	dark := []grid.Square{{Col: 1, Row: 2}, {Col: 2, Row: 2}, {Col: 3, Row: 2}, {Col: 7, Row: 2}}
	lit := mustCompile(t, stripScene(nil, dark))
	v := lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 15}})
	for _, sq := range []grid.Square{{Col: 2, Row: 2}, {Col: 3, Row: 2}, {Col: 4, Row: 2}} {
		if !v.Seen(sq) {
			t.Errorf("Devil's Sight inside the Darkness does not see %v", sq)
		}
	}
	if v.Seen(grid.Square{Col: 7, Row: 2}) {
		t.Error("Devil's Sight inside the Darkness sees beyond its range")
	}
	if lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DarkvisionFt: 60}}).Seen(grid.Square{Col: 2, Row: 2}) {
		t.Error("control: darkvision inside the Darkness sees the Darkness")
	}
}

// Blindsight still sees through a cloud, and Devil's Sight alone does not, also with both senses together.
func TestDevilsSightWithBlindsightSeesThroughACloudOnlyByBlindsight(t *testing.T) {
	t.Parallel()
	lit := mustCompile(t, stripScene(column(3), nil))
	behind := grid.Square{Col: 5, Row: 2}
	if lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 120}}).Seen(behind) {
		t.Error("Devil's Sight alone sees through the cloud")
	}
	if !lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 120, BlindsightFt: 20}}).Seen(behind) {
		t.Error("blindsight with Devil's Sight does not see through the cloud")
	}
	if lit.See(vision.Viewer{At: viewer, Senses: vision.Senses{DevilsSightFt: 120, BlindsightFt: 5}}).Seen(behind) {
		t.Error("blindsight beyond its range sees through the cloud")
	}
}
