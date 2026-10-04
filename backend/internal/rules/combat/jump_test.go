package combat

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func strength(score int) rules.Derived {
	mod := (score - 10) / 2
	if score < 10 && score%2 == 1 {
		mod-- // floor, not truncation, for an odd score below 10
	}
	return rules.Derived{Abilities: []rules.AbilityScore{
		{Ability: rules.DEX, Score: 18, Modifier: 4},
		{Ability: rules.STR, Score: score, Modifier: mod},
	}}
}

func TestJumpLimits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		score int
		want  Jumps
	}{
		// Toren, Força 16 (+3): 16 ft running, 8 standing; 6 ft high running, 3 standing.
		{"Toren", 16, Jumps{LongRunning: 160, LongStanding: 80, HighRunning: 60, HighStanding: 30}},
		// Brisa, Força 10 (+0): 10 and 5 ft; 3 and 1,5 ft.
		{"Brisa", 10, Jumps{LongRunning: 100, LongStanding: 50, HighRunning: 30, HighStanding: 15}},
		// Força 6 (-2): the high jump is 1 ft.
		{"Força 6", 6, Jumps{LongRunning: 60, LongStanding: 30, HighRunning: 10, HighStanding: 5}},
		// Força 1 (-5): 3 - 5 is below 0, and a high jump is never negative.
		{"Força 1", 1, Jumps{LongRunning: 10, LongStanding: 5, HighRunning: 0, HighStanding: 0}},
	}
	for _, c := range cases {
		if got := JumpLimits(strength(c.score)); got != c.want {
			t.Errorf("%s: JumpLimits = %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := JumpLimits(rules.Derived{}); got != (Jumps{}) {
		t.Errorf("a sheet with no Força: %+v", got)
	}
}

func TestJumpsWithAndWithoutARunningStart(t *testing.T) {
	t.Parallel()
	j := JumpLimits(strength(16))
	if j.Long(true) != 160 || j.Long(false) != 80 || j.High(true) != 60 || j.High(false) != 30 {
		t.Errorf("Long and High pick the wrong number: %+v", j)
	}
	// The move right before must be at least 10 ft.
	for _, c := range []struct {
		prev int
		want bool
	}{{0, false}, {99, false}, {100, true}, {300, true}} {
		if got := HasRunningStart(c.prev); got != c.want {
			t.Errorf("HasRunningStart(%d) = %v, want %v", c.prev, got, c.want)
		}
	}
}
