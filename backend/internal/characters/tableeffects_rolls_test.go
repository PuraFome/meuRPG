package characters

import (
	"reflect"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestStoredRollEffectsReachTheEngine: the rolls, at_most, resource and cost of a stored reroll become the
// engine's effect.
func TestStoredRollEffectsReachTheEngine(t *testing.T) {
	t.Parallel()
	got := tableEffectsOf([]*rulesv1.TableEffect{
		{Type: "reroll", Rolls: []string{"attack", "damage"}, AtMost: 2, Resource: "sorte", Cost: "1"},
	})
	want := []rules.Effect{
		{Type: "reroll", Rolls: []string{"attack", "damage"}, AtMost: 2, Resource: "sorte", Cost: "1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("effects = %+v, want %+v", got, want)
	}
}

// TestEffectMenuCarriesTheCriticalRangeLimits: the critical_range modifier target reaches the proto with
// its only mode and its literal range.
func TestEffectMenuCarriesTheCriticalRangeLimits(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range srd.EffectMenu().Lists {
		if l.Name != "modifier_targets" {
			continue
		}
		for _, v := range menuValues(l.Values) {
			if v.GetKey() != rules.CriticalRangeTarget {
				if len(v.GetOnlyModes()) != 0 || v.GetMin() != 0 || v.GetMax() != 0 {
					t.Errorf("target %s has limits: %v", v.GetKey(), v)
				}
				continue
			}
			if !reflect.DeepEqual(v.GetOnlyModes(), []string{"set"}) || v.GetMin() != 15 || v.GetMax() != 20 {
				t.Errorf("critical_range = %v, want only_modes [set], min 15, max 20", v)
			}
			return
		}
	}
	t.Fatal("critical_range is not in the modifier_targets list")
}
