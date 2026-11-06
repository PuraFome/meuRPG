package characters

import (
	"reflect"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The roll minimums of a sheet reach the combat's traits as they are.
func TestRollMinimumsOfCopiesTheDerivedMinimums(t *testing.T) {
	t.Parallel()
	d := rules.Derived{
		RollMinimums: []rules.RollMinimum{
			{Applies: []string{rules.RollAppliesAttack, rules.RollAppliesSave}, Value: 10, Source: "feature:a@mesa"},
			{Applies: []string{rules.RollDamage}, Value: 2, DamageType: "damage-type:fire", Source: "feature:b@mesa"},
		},
	}
	want := []link.RollMinimum{
		{Applies: []string{"attack", "save"}, Value: 10, Source: "feature:a@mesa"},
		{Applies: []string{"damage"}, Value: 2, DamageType: "damage-type:fire", Source: "feature:b@mesa"},
	}
	if got := rollMinimumsOf(d); !reflect.DeepEqual(got, want) {
		t.Errorf("rollMinimumsOf = %+v; want %+v", got, want)
	}
	if got := rollMinimumsOf(rules.Derived{}); len(got) != 0 {
		t.Errorf("rollMinimumsOf of nothing = %+v; want empty", got)
	}
}

// The spell critical range reaches the traits only when it is lowered (0 is 20).
func TestSpellCriticalRangeOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ derived, want int }{{20, 0}, {0, 0}, {19, 19}, {15, 15}} {
		if got := spellCriticalRangeOf(rules.Derived{SpellCriticalRange: tc.derived}); got != tc.want {
			t.Errorf("spellCriticalRangeOf(%d) = %d; want %d", tc.derived, got, tc.want)
		}
	}
	tr := traitsOf(nil, rules.Derived{SpellCriticalRange: 18, RollMinimums: []rules.RollMinimum{{Applies: []string{"damage"}, Value: 2}}})
	if tr.SpellCriticalRange != 18 || len(tr.RollMinimums) != 1 {
		t.Errorf("traitsOf = spell range %d, minimums %+v; want 18 and one", tr.SpellCriticalRange, tr.RollMinimums)
	}
}

// TestStoredRollMinimumReachesTheEngine: the rolls, value and damage type of a stored roll_minimum, and the
// scope (applies) of a stored critical_range modifier, become the engine's effect.
func TestStoredRollMinimumReachesTheEngine(t *testing.T) {
	t.Parallel()
	got := tableEffectsOf([]*rulesv1.TableEffect{
		{Type: "roll_minimum", Rolls: []string{"damage"}, Value: "2", DamageType: "damage-type:fire"},
		{Type: "modifier", Target: rules.CriticalRangeTarget, Mode: "set", Value: "19", Applies: "both"},
	})
	want := []rules.Effect{
		{Type: "roll_minimum", Rolls: []string{"damage"}, Value: "2", DamageType: "damage-type:fire"},
		{Type: "modifier", Target: rules.CriticalRangeTarget, Mode: "set", Value: "19", Applies: "both"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("effects = %+v, want %+v", got, want)
	}
}
