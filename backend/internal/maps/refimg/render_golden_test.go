package refimg

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/maps/dungeonimg"
)

// goldenPlan is a plan of cols x rows squares, about a third of them solid, about a third not
// seen when withSeen, with some markers (some of them outside the plan); the same for the
// same seed.
func goldenPlan(cols, rows int, seed uint64, withSeen bool) Plan {
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: a repeatable plan for a test, not a secret
	p := Plan{Cols: cols, Rows: rows, Solid: make([]bool, cols*rows)}
	for i := range p.Solid {
		p.Solid[i] = rng.IntN(3) == 0
	}
	if withSeen {
		p.Seen = make([]bool, cols*rows)
		for i := range p.Seen {
			p.Seen[i] = rng.IntN(3) != 0
		}
	}
	for i := range 6 {
		p.Markers = append(p.Markers, Marker{Col: rng.IntN(cols+2) - 1, Row: rng.IntN(rows+2) - 1, Party: i%2 == 0})
	}
	return p
}

// The pixels and the palette of Render, frozen by a hash for plans that show everything, plans
// with unseen squares and plans with markers, at sizes that are and are not a multiple of the
// grid.
func TestRenderPixels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    Plan
		w, h int
		want string
	}{
		{"everything seen", goldenPlan(12, 9, 1, false), 12 * 24, 9 * 24, "a83f7c574a76d9bfd27b1afba314d1286bb6b821d73033ba35c0fd1364b8443e"},
		{"unseen squares", goldenPlan(12, 9, 2, true), 12 * 24, 9 * 24, "3abff6da068f349a1fb45f9ceb632470c6d70ec81a0394320977f949ff8512d8"},
		{"a size that is not a multiple", goldenPlan(13, 7, 3, true), 100, 61, "73578b84713fbfb3ad2c4acdd66039ce21f3b107b67a6b672d3f2ea18a88a8c8"},
		{"one pixel a square", goldenPlan(20, 15, 4, true), 20, 15, "dec62d18a6e08c87f11744d2519d3cacb9f6eb65a565e9de14ba969790d5d05d"},
		{"a single square", goldenPlan(1, 1, 5, false), 24, 24, "09b32f6aaa8770ad509909fd36e86f08e83bc98cf150179b0ef74245552a8a5a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			img, err := Render(tc.p, tc.w, tc.h)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			sum := sha256.New()
			sum.Write(img.Pix)
			fmt.Fprint(sum, img.Palette)
			if got := hex.EncodeToString(sum.Sum(nil)); got != tc.want {
				t.Errorf("sha256(pixels, palette) = %s, want %s", got, tc.want)
			}
		})
	}
}

// A plan that does not fit is refused with refimg's own ErrPlan before anything is drawn. A size
// too small for the plan is refused by the drawing under it, and the error that comes up is
// dungeonimg's ErrPlan, not refimg's (frozen as it is: the caller sees one error value or the
// other depending on which check caught it).
func TestRenderPlanRefusals(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]Plan{
		"no columns":       {Cols: 0, Rows: 3},
		"a short slice":    {Cols: 4, Rows: 3, Solid: make([]bool, 11)},
		"a short seen":     {Cols: 4, Rows: 3, Solid: make([]bool, 12), Seen: make([]bool, 5)},
		"negative columns": {Cols: -1, Rows: 3, Solid: make([]bool, 3)},
	} {
		if img, err := Render(p, 40, 30); img != nil || !errors.Is(err, ErrPlan) {
			t.Errorf("%s: Render() = %v, %v; want refimg's ErrPlan", name, img, err)
		}
	}
	for name, size := range map[string][2]int{"narrower than the plan": {3, 30}, "shorter than the plan": {40, 2}} {
		img, err := Render(cave(), size[0], size[1])
		if img != nil || !errors.Is(err, dungeonimg.ErrPlan) || errors.Is(err, ErrPlan) {
			t.Errorf("%s: Render() = %v, %v; want dungeonimg's ErrPlan only", name, img, err)
		}
	}
}
