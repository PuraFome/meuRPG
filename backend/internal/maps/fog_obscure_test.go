package maps

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// The scene's key hashes the squares of the clouds and of the magical darkness (RN-37): a cloud makes another scene,
// the same squares in another order are the same scene, and a cloud is not the same scene as darkness on the same
// squares.
func TestSceneKeyHashesTheSquaresOfCloudsAndDarkness(t *testing.T) {
	t.Parallel()
	in := fogInput{mapID: "m", campaignID: "c", g: grid.Grid{Columns: 10, Rows: 10}, base: grid.Bright}
	a := []grid.Square{{Col: 1, Row: 1}, {Col: 2, Row: 1}, {Col: 3, Row: 4}}
	reordered := []grid.Square{{Col: 3, Row: 4}, {Col: 1, Row: 1}, {Col: 2, Row: 1}}
	none := litKeyOf(in, nil, obscured{})
	cloud := litKeyOf(in, nil, obscured{clouds: a})
	cases := []struct {
		name string
		got  litKey
		same litKey
		want bool
	}{
		{"a cloud is a new scene", cloud, none, false},
		{"the same squares in another order are the same scene", litKeyOf(in, nil, obscured{clouds: reordered}), cloud, true},
		{"darkness on the same squares is another scene", litKeyOf(in, nil, obscured{dark: a}), cloud, false},
		{"one square more is another scene", litKeyOf(in, nil, obscured{clouds: append(append([]grid.Square{}, a...), grid.Square{Col: 9, Row: 9})}), cloud, false},
		{"duplicates and squares off the grid are not part of the scene", litKeyOf(in, nil, obscured{clouds: append(append([]grid.Square{}, a...), a[0], grid.Square{Col: -1, Row: 5}, grid.Square{Col: 50, Row: 50})}), cloud, true},
		{"a cloud and darkness are not one list of both", litKeyOf(in, nil, obscured{clouds: a[:1], dark: a[1:]}), litKeyOf(in, nil, obscured{clouds: a[:2], dark: a[2:]}), false},
	}
	for _, tc := range cases {
		if got := tc.got == tc.same; got != tc.want {
			t.Errorf("%s: keys equal = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The layer of the squares leaves out the ones off the grid and is nil for none, as vision.Scene reads it.
func TestObscuredLayerKeepsOnlySquaresOfTheGrid(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 4, Rows: 4}
	if l := obscuredLayer(g, nil); l != nil {
		t.Errorf("layer of no squares = %v, want nil", l)
	}
	l := obscuredLayer(g, []grid.Square{{Col: 1, Row: 1}, {Col: 9, Row: 9}, {Col: -1, Row: 0}})
	if l == nil || l.Count() != 1 || !l.Get(1, 1) {
		t.Errorf("layer = %v, want only (1,1)", l)
	}
}

// A scene compiled from the layers blocks the line through a cloud: the same check the play tests make through the
// whole stack, here on the maps side of the seam.
func TestCompiledSceneBlocksALineThroughACloud(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 12, Rows: 5}
	cloud := obscuredLayer(g, []grid.Square{{Col: 5, Row: 2}})
	lit, err := vision.Compile(vision.Scene{Grid: g, Base: grid.Bright, Clouds: cloud})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	v := vision.Viewer{At: grid.Square{Col: 2, Row: 2}}
	if lit.CanSee(v, grid.Square{Col: 9, Row: 2}) {
		t.Error("the viewer sees through the cloud")
	}
	if !lit.CanSee(v, grid.Square{Col: 4, Row: 2}) {
		t.Error("the viewer does not see the square before the cloud")
	}
	if !lit.CanSee(v, grid.Square{Col: 9, Row: 4}) {
		t.Error("the viewer does not see a square the line to which misses the cloud")
	}
}
