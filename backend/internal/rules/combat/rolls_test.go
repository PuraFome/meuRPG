package combat

import "testing"

// A critical range below 20 (Improved Critical 19, Superior Critical 18) makes
// those natural rolls hit and be critical hits, whatever the armor class; a
// natural 1 still misses.
func TestResolveAttackFromCriticalRange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		face, from int
		hit, crit  bool
		name       string
	}{
		{19, 20, false, false, "19 with the plain range"},
		{19, 19, true, true, "19 with Improved Critical"},
		{18, 19, false, false, "18 with Improved Critical"},
		{18, 18, true, true, "18 with Superior Critical"},
		{20, 18, true, true, "20 with Superior Critical"},
		{1, 18, false, false, "1 with Superior Critical"},
		{19, 0, false, false, "19 with no range set"},
		{19, 25, false, false, "19 with a range above 20"},
	} {
		r := ResolveAttackFrom(0, 30, tc.face, tc.from)
		if r.Hit != tc.hit || r.Critical != tc.crit {
			t.Errorf("%s: hit %v, critical %v; want %v and %v", tc.name, r.Hit, r.Critical, tc.hit, tc.crit)
		}
	}
}
