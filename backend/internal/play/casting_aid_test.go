package play

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

func TestAidRaise(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		before, amount      int32
		wantBonus, wantRose int32
	}{
		{"first Aid", 0, 5, 5, 5},
		{"a stronger Aid over a weaker one adds only the difference", 5, 10, 10, 5},
		{"a weaker Aid over a stronger one changes nothing", 10, 5, 10, 0},
		{"the same Aid again changes nothing", 5, 5, 5, 0},
	} {
		bonus, rose := aidRaise(tc.before, tc.amount)
		if bonus != tc.wantBonus || rose != tc.wantRose {
			t.Errorf("%s: aidRaise(%d, %d) = (%d, %d), want (%d, %d)", tc.name, tc.before, tc.amount, bonus, rose, tc.wantBonus, tc.wantRose)
		}
	}
}

func TestAidEnd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		hp, maxBefore, bonus int32
		wantHP, wantMaxAfter int32
	}{
		{"full hit points are cut to the old maximum", 25, 25, 5, 20, 20},
		{"hit points below the new maximum stay", 12, 25, 5, 12, 20},
		{"the cut never takes the last hit point", 10, 5, 5, 1, 1},
		{"a target at 0 stays at 0", 0, 25, 5, 0, 20},
		{"the maximum never falls below 1", 3, 4, 10, 1, 1},
	} {
		hp, mx := aidEnd(tc.hp, tc.maxBefore, tc.bonus)
		if hp != tc.wantHP || mx != tc.wantMaxAfter {
			t.Errorf("%s: aidEnd(%d, %d, %d) = (%d, %d), want (%d, %d)", tc.name, tc.hp, tc.maxBefore, tc.bonus, hp, mx, tc.wantHP, tc.wantMaxAfter)
		}
	}
}

func TestAidLeft(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		bonus  int32
		others []int32
		want   int32
	}{
		{"no other Aid: all of it goes", 10, nil, 0},
		{"a weaker one still on the target is what stays", 10, []int32{5}, 5},
		{"the strongest of several stays", 15, []int32{5, 10}, 10},
		{"never more than the bonus the target has", 5, []int32{10}, 5},
		{"a bonus already gone stays gone", 0, []int32{5}, 0},
	} {
		if got := aidLeft(tc.bonus, tc.others); got != tc.want {
			t.Errorf("%s: aidLeft(%d, %v) = %d, want %d", tc.name, tc.bonus, tc.others, got, tc.want)
		}
	}
}

func TestAidTargetsOf(t *testing.T) {
	t.Parallel()
	targets, err := json.Marshal([]castTarget{
		{ID: "a", Effect: castMaxHP, Amount: 5, Before: 20, After: 25},
		{ID: "b", Effect: castNarrated},
		{ID: "c", Effect: castHeal, Amount: 3},
		{ID: "d", Effect: castMaxHP, Amount: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := aidTargetsOf(playdb.SpellCast{Targets: targets})
	var ids []string
	for _, g := range got {
		ids = append(ids, g.ID)
	}
	if want := []string{"a", "d"}; !slices.Equal(ids, want) {
		t.Errorf("aidTargetsOf() = %v, want %v", ids, want)
	}
}
