package characters

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The resources and hooks of these tests are invented: a pool that returns one use on a rest and
// the on_event hooks a table feat gives.

// fxMax is a character of 30 hit points with the given resources and hooks.
func fxMax(resources []rules.Resource, hooks ...rules.EventHook) vitalsMax {
	return vitalsMax{hitPoints: 30, resources: resources, hooks: hooks, hitDice: []rules.HitDice{{Die: 8, Count: 3}}, hitDiceTotal: 3}
}

func fxVitals(m vitalsMax, hp, temp int32, used map[string]int) *playv1.CharacterVitals {
	row := vitalsRow{ID: "c-fx", Name: "Teste", HitPointsCurrent: &hp, HitPointsTemporary: &temp}
	if len(used) > 0 {
		row.ResourcesUsed, _ = json.Marshal(used)
	}
	return vitalsToProto(row, m)
}

func usedIn(v *playv1.CharacterVitals, key string) int32 {
	for _, r := range v.GetResources() {
		if r.GetKey() == key {
			return r.GetUsed()
		}
	}
	return -1
}

// TestARestReturnsOneUseOfAResourceThatRestoresOne: restore_one_on_short_rest gives one use on a short rest and
// all on a long one; restore_one_on_long_rest gives one on a long rest and none on a short one; the preview tells
// how many come back.
func TestARestReturnsOneUseOfAResourceThatRestoresOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		recharge string
		kind     string
		used     int
		want     int32
		back     int32 // RestResourceBack.Back, 0 for all
	}{
		{"one on a short rest", rules.RechargeRestoreOneShort, rules.RestShort, 3, 2, 1},
		{"all on a long rest", rules.RechargeRestoreOneShort, rules.RestLong, 3, 0, 0},
		{"one on a long rest", rules.RechargeRestoreOneLong, rules.RestLong, 3, 2, 1},
		{"none on a short rest", rules.RechargeRestoreOneLong, rules.RestShort, 3, 3, 0},
		{"the last one", rules.RechargeRestoreOneShort, rules.RestShort, 1, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := fxMax([]rules.Resource{{Key: "reserva_de_teste", NamePT: "Reserva de Teste", Max: 4, Recharge: tc.recharge}})
			v := fxVitals(m, 30, 0, map[string]int{"reserva_de_teste": tc.used})
			plan, err := planRest(tc.kind, v, m, nil)
			if err != nil {
				t.Fatalf("planRest() error = %v", err)
			}
			if got := usedIn(plan.after, "reserva_de_teste"); got != tc.want {
				t.Errorf("uses spent after = %d, want %d", got, tc.want)
			}
			var back int32
			for _, r := range plan.preview.GetResources() {
				if r.GetKey() == "reserva_de_teste" {
					back = r.GetBack()
				}
			}
			if back != tc.back {
				t.Errorf("preview back = %d, want %d", back, tc.back)
			}
		})
	}
}

// TestARestRunsTheOwnersHooks: a long rest hook gives temporary hit points after the rest ended the old ones; a
// short rest heals by the share of the maximum and restores uses; a hook that spends a use runs only when one is
// left; a second rest changes nothing more.
func TestARestRunsTheOwnersHooks(t *testing.T) {
	t.Parallel()
	pool := rules.Resource{Key: "reserva_de_teste", NamePT: "Reserva de Teste", Max: 3, Recharge: rules.RechargeNone}
	t.Run("temporary hit points on a long rest", func(t *testing.T) {
		t.Parallel()
		m := fxMax(nil, rules.EventHook{Event: rules.EventLongRest, Source: "feat:x", TempHP: 5})
		v := fxVitals(m, 20, 3, nil)
		long, err := planRest(rules.RestLong, v, m, nil)
		if err != nil {
			t.Fatalf("planRest() error = %v", err)
		}
		if got := long.after.GetHitPointsTemporary(); got != 5 {
			t.Errorf("temporary hit points after a long rest = %d, want 5 (given after the rest ended the old ones)", got)
		}
		if h := long.preview.GetHooks(); len(h) != 1 || h[0].GetTempHpGained() != 5 {
			t.Errorf("preview hooks = %v, want one that gives 5", h)
		}
		if short, _ := planRest(rules.RestShort, v, m, nil); short.after.GetHitPointsTemporary() != 3 {
			t.Errorf("a short rest ran a long rest hook: temporary hit points = %d, want 3", short.after.GetHitPointsTemporary())
		}
		again, err := planRest(rules.RestLong, long.after, m, nil)
		if err != nil || again.after.GetHitPointsTemporary() != 5 {
			t.Errorf("a second long rest = %v, %v; want 5 temporary hit points again, not 10", again.after.GetHitPointsTemporary(), err)
		}
	})
	t.Run("a heal by share of the maximum and a restore on a short rest", func(t *testing.T) {
		t.Parallel()
		m := fxMax([]rules.Resource{pool}, rules.EventHook{Event: rules.EventShortRest, Source: "feat:x", HealPercentMax: 50, Restore: "reserva_de_teste", RestoreCount: 2})
		v := fxVitals(m, 4, 0, map[string]int{"reserva_de_teste": 3})
		first, err := planRest(rules.RestShort, v, m, nil)
		if err != nil {
			t.Fatalf("planRest() error = %v", err)
		}
		if first.after.GetHitPointsCurrent() != 19 || usedIn(first.after, "reserva_de_teste") != 1 {
			t.Errorf("after the first rest %d hit points and %d uses spent, want 19 and 1", first.after.GetHitPointsCurrent(), usedIn(first.after, "reserva_de_teste"))
		}
		second, err := planRest(rules.RestShort, first.after, m, nil)
		if err != nil {
			t.Fatalf("planRest() error = %v", err)
		}
		if usedIn(second.after, "reserva_de_teste") != 0 {
			t.Errorf("the second rest restored to %d spent, want 0 (never more than was spent)", usedIn(second.after, "reserva_de_teste"))
		}
		third, _ := planRest(rules.RestShort, second.after, m, nil)
		if usedIn(third.after, "reserva_de_teste") != 0 {
			t.Errorf("a third rest left %d spent, want 0: nothing to give back", usedIn(third.after, "reserva_de_teste"))
		}
	})
	t.Run("a hook that spends a use", func(t *testing.T) {
		t.Parallel()
		m := fxMax([]rules.Resource{pool}, rules.EventHook{Event: rules.EventShortRest, Source: "feat:x", TempHP: 4, Spends: "reserva_de_teste", SpendCount: 2})
		with := fxVitals(m, 30, 0, map[string]int{"reserva_de_teste": 1})
		plan, err := planRest(rules.RestShort, with, m, nil)
		if err != nil || plan.after.GetHitPointsTemporary() != 4 || usedIn(plan.after, "reserva_de_teste") != 3 {
			t.Errorf("with 2 left: temp %d, spent %d, err %v; want 4, 3", plan.after.GetHitPointsTemporary(), usedIn(plan.after, "reserva_de_teste"), err)
		}
		without := fxVitals(m, 30, 0, map[string]int{"reserva_de_teste": 2})
		plan, err = planRest(rules.RestShort, without, m, nil)
		if err != nil || plan.after.GetHitPointsTemporary() != 0 || !proto.Equal(plan.after, without) {
			t.Errorf("with 1 left the hook ran: temp %d, err %v", plan.after.GetHitPointsTemporary(), err)
		}
	})
	t.Run("a dice heal is rolled by the rest", func(t *testing.T) {
		t.Parallel()
		m := fxMax(nil, rules.EventHook{Event: rules.EventShortRest, Source: "feat:x", Heal: &rules.DiceFormula{Count: 2, Sides: 6, Bonus: 1}})
		v := fxVitals(m, 10, 0, nil)
		plan, err := planRestWith(rules.RestShort, v, m, nil, func(d rules.DiceFormula) (int, error) { return d.Count*d.Sides + d.Bonus, nil })
		if err != nil || plan.after.GetHitPointsCurrent() != 23 {
			t.Errorf("hit points after = %d, err %v; want 10 + 13 = 23", plan.after.GetHitPointsCurrent(), err)
		}
		estimate, err := planRest(rules.RestShort, v, m, nil)
		if err != nil || estimate.after.GetHitPointsCurrent() != 18 {
			t.Errorf("the estimate (no roller) = %d, err %v; want the average 2d6+1 = 8, so 18", estimate.after.GetHitPointsCurrent(), err)
		}
	})
}
