package dungeonimg

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"testing"
)

// randomPlan is a plan of cols x rows squares, about a third of them solid, the same for
// the same seed.
func randomPlan(cols, rows int, seed uint64) Floorplan {
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: a repeatable plan for a test, not a secret
	f := Floorplan{Cols: cols, Rows: rows, Solid: make([]bool, cols*rows)}
	for i := range f.Solid {
		f.Solid[i] = rng.IntN(3) == 0
	}
	return f
}

// The pixels of Render, frozen by a hash for plans of several shapes: squares that are not
// square, a size that is not a multiple of the grid, a one-pixel square and an outline wider
// than a pixel. Every pixel is floor, ink or wall, and which one is what the hash holds.
func TestRenderPixelsAreFrozen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		cols, rows int
		w, h       int
		seed       uint64
		want       string
	}{
		{"square squares", 12, 9, 12 * 24, 9 * 24, 1, "d967a3b7da755d14e822fca6e3d677f51484c65c3f601752cee88b7bbab78e02"},
		{"wider than tall squares", 12, 9, 12 * 30, 9 * 12, 2, "64a979cac4bd800322b2844c97adb712f43c106a30321d374ff178bef9520e2d"},
		{"a size that is not a multiple", 13, 7, 100, 61, 3, "a49f7e856fcf1f8ce3094110dae00a44dfe69f4e02cfd87d6ece5f82310d544e"},
		{"one pixel a square", 20, 15, 20, 15, 4, "d6865e68e11c8ba69c2c134cd05b9689d5a4ae50a9cf0b69e55a62166261f16d"},
		{"a single square", 1, 1, 24, 24, 5, "b88b6c3565891d5a2fef1b9768a47fea25586390be9ca82296d3568d1cd2c8da"},
		{"a thin plan", 40, 2, 40 * 5, 2 * 5, 6, "8ca30627037bb89d95f6464df26ab92613c75bfb0dab80c4fa31b2df21227309"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			img, err := Render(randomPlan(tc.cols, tc.rows, tc.seed), tc.w, tc.h)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			sum := sha256.Sum256(img.Pix)
			if got := hex.EncodeToString(sum[:]); got != tc.want {
				t.Errorf("sha256(pixels) = %s, want %s", got, tc.want)
			}
		})
	}
}

// Each way a plan does not fit is ErrPlan, whatever else is wrong with it.
func TestRenderPlanRefusalsAreFrozen(t *testing.T) {
	t.Parallel()
	ok := randomPlan(4, 3, 1)
	short := Floorplan{Cols: 4, Rows: 3, Solid: make([]bool, 11)}
	for name, tc := range map[string]struct {
		f    Floorplan
		w, h int
	}{
		"no columns":       {Floorplan{Cols: 0, Rows: 3}, 10, 10},
		"no rows":          {Floorplan{Cols: 4, Rows: 0}, 10, 10},
		"a short slice":    {short, 40, 30},
		"narrower":         {ok, 3, 30},
		"shorter":          {ok, 40, 2},
		"negative columns": {Floorplan{Cols: -1, Rows: 3, Solid: make([]bool, 3)}, 10, 10},
	} {
		if img, err := Render(tc.f, tc.w, tc.h); img != nil || !errors.Is(err, ErrPlan) {
			t.Errorf("%s: Render() = %v, %v; want ErrPlan", name, img, err)
		}
	}
}
