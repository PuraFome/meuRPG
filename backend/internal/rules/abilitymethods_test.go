package rules

import (
	"errors"
	"testing"
)

func TestCheckStandardArray(t *testing.T) {
	tests := []struct {
		name   string
		scores []int
		want   error
	}{
		{"in order", []int{15, 14, 13, 12, 10, 8}, nil},
		{"permuted", []int{8, 10, 12, 13, 14, 15}, nil},
		{"a value twice", []int{15, 15, 13, 12, 10, 8}, ErrNotStandardArray},
		{"another value", []int{15, 14, 13, 12, 10, 9}, ErrNotStandardArray},
		{"five values", []int{15, 14, 13, 12, 10}, ErrNotStandardArray},
		{"seven values", []int{15, 14, 13, 12, 10, 8, 8}, ErrNotStandardArray},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckStandardArray(tt.scores); !errors.Is(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointBuy(t *testing.T) {
	tests := []struct {
		name      string
		scores    []int
		want      error
		wantSpent int
	}{
		{"the spec's 25 of 27", []int{10, 14, 13, 8, 15, 10}, nil, 25},
		{"exactly 27", []int{15, 15, 15, 8, 8, 8}, nil, 27},
		{"all 8", []int{8, 8, 8, 8, 8, 8}, nil, 0},
		{"28 points", []int{15, 15, 15, 9, 8, 8}, ErrBadPointBuy, 28},
		{"a 16", []int{16, 8, 8, 8, 8, 8}, ErrBadPointBuy, 0},
		{"a 7", []int{7, 8, 8, 8, 8, 8}, ErrBadPointBuy, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPointBuy(tt.scores); !errors.Is(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if spent, ok := PointBuySpent(tt.scores); ok && spent != tt.wantSpent {
				t.Errorf("spent %d, want %d", spent, tt.wantSpent)
			}
		})
	}
	for score, want := range map[int]int{8: 0, 9: 1, 10: 2, 11: 3, 12: 4, 13: 5, 14: 7, 15: 9} {
		if got, ok := PointBuyCost(score); !ok || got != want {
			t.Errorf("cost of %d: got %d, %v; want %d", score, got, ok, want)
		}
	}
}

func TestCheckTyped(t *testing.T) {
	tests := []struct {
		name   string
		scores []int
		want   error
	}{
		{"the edges", []int{3, 18, 3, 18, 10, 10}, nil},
		{"a 2", []int{2, 10, 10, 10, 10, 10}, ErrTypedRange},
		{"a 19", []int{19, 10, 10, 10, 10, 10}, ErrTypedRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckTyped(tt.scores); !errors.Is(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAbilityRolls(t *testing.T) {
	// The dice of the E10-03 drawing: 16, 14, 13, 12, 10, 8.
	sets := [][]int{{6, 5, 5, 2}, {5, 5, 4, 1}, {5, 4, 4, 3}, {4, 4, 4, 2}, {4, 3, 3, 2}, {3, 3, 2, 1}}
	if err := CheckAbilityRollDice(sets); err != nil {
		t.Fatal(err)
	}
	totals := AbilityRollTotals(sets)
	for i, want := range []int{16, 14, 13, 12, 10, 8} {
		if totals[i] != want {
			t.Errorf("set %d: got %d, want %d", i, totals[i], want)
		}
	}
	tests := []struct {
		name   string
		scores []int
		want   error
	}{
		{"as rolled", []int{16, 14, 13, 12, 10, 8}, nil},
		{"assigned", []int{12, 14, 13, 8, 16, 10}, nil},
		{"a result twice", []int{16, 16, 13, 12, 10, 8}, ErrNotTheRolls},
		{"another result", []int{17, 14, 13, 12, 10, 8}, ErrNotTheRolls},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckRolled(tt.scores, sets); !errors.Is(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
	bad := map[string][][]int{
		"five sets":  sets[:5],
		"three dice": {{1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}},
		"a 7":        {{7, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}},
		"a 0":        {{0, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}},
	}
	for name, s := range bad {
		if CheckAbilityRollDice(s) == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestFreeAbilityPoints(t *testing.T) {
	t.Parallel()
	c, err := LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		race  string
		class string
		level int
		want  int
	}{
		{"a gnome has none", "race:gnome", "class:wizard", 1, 0},
		{"the half-elf's two points are picks, not manual points", "race:half-elf", "class:wizard", 1, 0},
		{"a fighter's first ASI is level 4", "race:human", "class:fighter", 3, 0},
		{"a level 4 wizard has one ASI", "race:gnome", "class:wizard", 4, 2},
		{"a half-elf fighter of level 6 has the ASIs of the class only", "race:half-elf", "class:fighter", 6, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := Build{Race: tt.race, Classes: []ClassLevel{{Class: tt.class, Level: tt.level}}}
			if got := c.FreeAbilityPoints(b); got != tt.want {
				t.Errorf("FreeAbilityPoints() = %d, want %d", got, tt.want)
			}
		})
	}
	if got := PositiveManualBonus(Build{ExtraAbilityBonuses: map[Ability]int{STR: 2, DEX: -3, CON: 1}}); got != 3 {
		t.Errorf("PositiveManualBonus() = %d, want 3", got)
	}
}
