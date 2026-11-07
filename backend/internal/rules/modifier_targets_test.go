package rules

import "testing"

// The table modifier targets of the passives, the melee reach and the hit die minimum (#572). The names are invented.

func derivedWith(t *testing.T, level int, effects ...Effect) Derived {
	t.Helper()
	return Derive(rollBuild(level), withOverlay(t, rollOverlay(effects, nil)))
}

func TestPassiveTargetsAddToThePassivesAlone(t *testing.T) {
	t.Parallel()
	base := derivedWith(t, 3)
	d := derivedWith(t, 3,
		Effect{Type: "modifier", Target: PassivePerceptionTarget, Mode: "add", Value: "5"},
		Effect{Type: "modifier", Target: PassiveInvestigationTarget, Mode: "set", Value: "20"},
		Effect{Type: "modifier", Target: PassiveInsightTarget, Mode: "max", Value: "1"},
	)
	if got, want := d.PassivePerception, base.PassivePerception+5; got != want {
		t.Errorf("PassivePerception = %d, want %d: add 5", got, want)
	}
	if d.PassiveInvestigation != 20 {
		t.Errorf("PassiveInvestigation = %d, want 20: set", d.PassiveInvestigation)
	}
	if d.PassiveInsight != base.PassiveInsight {
		t.Errorf("PassiveInsight = %d, want %d: max 1 is below it", d.PassiveInsight, base.PassiveInsight)
	}
	for i, sk := range d.Skills {
		if sk.Bonus != base.Skills[i].Bonus {
			t.Errorf("skill %s bonus = %d, want %d: a passive modifier is no check bonus", sk.Key, sk.Bonus, base.Skills[i].Bonus)
		}
	}
}

func TestPassiveTargetWhenFalseChangesNothing(t *testing.T) {
	t.Parallel()
	base := derivedWith(t, 3)
	d := derivedWith(t, 3, Effect{Type: "modifier", Target: PassivePerceptionTarget, Mode: "add", Value: "5", When: "level() >= 10"})
	if d.PassivePerception != base.PassivePerception {
		t.Errorf("PassivePerception = %d, want %d: the when is false", d.PassivePerception, base.PassivePerception)
	}
}

func meleeReaches(d Derived) map[string]int {
	out := map[string]int{}
	for _, a := range d.Attacks {
		if a.Melee {
			out[a.Key] = max(a.ReachFt, a.RangeFt)
		}
	}
	return out
}

func TestReachTargetWidensEveryMeleeLineButNeverShortensOne(t *testing.T) {
	t.Parallel()
	base := meleeReaches(derivedWith(t, 3))
	if len(base) == 0 {
		t.Fatal("the fixture has no melee line")
	}
	for _, tc := range []struct {
		name   string
		effect Effect
		want   func(own int) int
	}{
		{"add 5 gives 10", Effect{Type: "modifier", Target: ReachMeleeTarget, Mode: "add", Value: "5"}, func(own int) int { return max(own, 10) }},
		{"set 15 gives 15", Effect{Type: "modifier", Target: ReachMeleeTarget, Mode: "set", Value: "15"}, func(own int) int { return max(own, 15) }},
		{"set 0 never shortens", Effect{Type: "modifier", Target: ReachMeleeTarget, Mode: "set", Value: "0"}, func(own int) int { return own }},
		{"a huge reach is bounded", Effect{Type: "modifier", Target: ReachMeleeTarget, Mode: "set", Value: "500"}, func(own int) int { return max(own, MaxWeaponReachFt) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for key, got := range meleeReaches(derivedWith(t, 3, tc.effect)) {
				if want := tc.want(base[key]); got != want {
					t.Errorf("reach of %s = %d, want %d (own %d)", key, got, want, base[key])
				}
			}
		})
	}
}

func TestReachTargetLeavesRangedLinesAlone(t *testing.T) {
	t.Parallel()
	before, after := derivedWith(t, 3), derivedWith(t, 3, Effect{Type: "modifier", Target: ReachMeleeTarget, Mode: "set", Value: "30"})
	for i, a := range after.Attacks {
		if !a.Melee && (a.ReachFt != before.Attacks[i].ReachFt || a.RangeFt != before.Attacks[i].RangeFt) {
			t.Errorf("ranged line %s changed: %+v, was %+v", a.Key, a, before.Attacks[i])
		}
	}
}

func TestHitDieMinimumTarget(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		effect []Effect
		want   int
	}{
		{"none by default", nil, 0},
		{"set 4", []Effect{{Type: "modifier", Target: HitDieMinimumTarget, Mode: "set", Value: "4"}}, 4},
		{"a formula reads the modifier", []Effect{{Type: "modifier", Target: HitDieMinimumTarget, Mode: "add", Value: `mod("con") * 2`}}, 4},
		{"a negative result is none", []Effect{{Type: "modifier", Target: HitDieMinimumTarget, Mode: "set", Value: "0 - 3"}}, 0},
		{"a huge one is bounded", []Effect{{Type: "modifier", Target: HitDieMinimumTarget, Mode: "set", Value: "900"}}, MaxHitDieMinimum},
		{"a false when keeps none", []Effect{{Type: "modifier", Target: HitDieMinimumTarget, Mode: "set", Value: "4", When: "level() >= 10"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := derivedWith(t, 3, tc.effect...)
			want := tc.want
			if tc.name == "a formula reads the modifier" {
				want = 2 * abilityModifier(t, d, CON)
			}
			if d.HitDieMinimum != want {
				t.Errorf("HitDieMinimum = %d, want %d", d.HitDieMinimum, want)
			}
		})
	}
}

func abilityModifier(t *testing.T, d Derived, a Ability) int {
	t.Helper()
	for _, x := range d.Abilities {
		if x.Ability == a {
			return x.Modifier
		}
	}
	t.Fatalf("no ability %s", a)
	return 0
}

// The menu lists the new targets with their Portuguese names, so the editor's picker shows them.
func TestTheEffectMenuListsTheNewModifierTargets(t *testing.T) {
	t.Parallel()
	menu := withOverlay(t, Overlay{Revision: 1}).EffectMenu()
	got := map[string]string{}
	for _, l := range menu.Lists {
		if l.Name != ListModifierTargets {
			continue
		}
		for _, v := range l.Values {
			got[v.Key] = v.NamePT
		}
	}
	for _, k := range []string{PassivePerceptionTarget, PassiveInvestigationTarget, PassiveInsightTarget, ReachMeleeTarget, HitDieMinimumTarget} {
		if got[k] == "" {
			t.Errorf("the menu has no named target %q: %v", k, got)
		}
	}
}
