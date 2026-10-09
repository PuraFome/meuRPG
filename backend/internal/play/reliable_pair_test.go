package play

import "testing"

// Reliable Talent on a check with two d20: the die that counts is the one marked, and
// a die that already counts as 10 or more is not.
func TestMarkTreatedMarksTheDieThatCounted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		faces   []int32
		counted int32
		total   int32
		want    int32 // 0: not marked
	}{
		{"advantage, the higher is low", []int32{6, 4}, 0, 19, 10},
		{"disadvantage, the lower is low", []int32{15, 6}, 1, 19, 10},
		{"the die that counts is high", []int32{14, 3}, 0, 23, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			roll := diceRoll(2, 20, tc.faces, 9, tc.total, false)
			roll.CountedIndex = tc.counted
			markTreated(roll, 9, tc.total)
			if got := roll.GetTreatedAs(); got != tc.want {
				t.Errorf("treated_as = %d, want %d", got, tc.want)
			}
		})
	}
}
