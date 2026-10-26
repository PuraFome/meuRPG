package combat

import "testing"

// TestApplyTempHP: temporary hit points do not stack, the creature keeps the larger
// (SRD 5.1, "Temporary Hit Points").
func TestApplyTempHP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		current, amount      int
		wantTemp, wantGained int
	}{
		{"none yet", 0, 5, 5, 5},
		{"a larger amount replaces", 3, 5, 5, 2},
		{"a smaller amount changes nothing", 8, 5, 8, 0},
		{"an equal amount changes nothing", 5, 5, 5, 0},
		{"a negative amount counts as 0", 4, -2, 4, 0},
		{"nothing given", 0, 0, 0, 0},
	} {
		got := ApplyTempHP(tc.current, tc.amount)
		if got.TempHP != tc.wantTemp || got.Gained != tc.wantGained {
			t.Errorf("%s: ApplyTempHP(%d, %d) = %+v, want %d and %d gained", tc.name, tc.current, tc.amount, got, tc.wantTemp, tc.wantGained)
		}
	}
}
