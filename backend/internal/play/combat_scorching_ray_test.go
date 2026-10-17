package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// rayTargets lists a spell's targets with the rays each one is given (the target's darts field).
func rayTargets(t *testing.T, a *armed, rays map[string]int32, order ...string) []*playv1.SpellTarget {
	t.Helper()
	var out []*playv1.SpellTarget
	for _, label := range order {
		out = append(out, darts(a, t, label, rays[label]))
	}
	return out
}

// TestScorchingRayAllRaysAtOneTarget: the caster may send every ray at one creature; each ray is its own
// spell attack with its own d20, hit or miss, and each ray that hits opens its own damage to roll.
func TestScorchingRayAllRaysAtOneTarget(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)
	goblin := a.id(t, "Goblin 1")
	// An enemy stands next to the caster, so every ray rolls two d20 and keeps the lower. Against armor class
	// 15 with +7: a critical hit, a miss, a hit.
	a.h.roller.queue(20, 20, 2, 2, 10, 10)
	cast := a.mustCast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), rayTargets(t, a, map[string]int32{"Goblin 1": 3}, "Goblin 1"), poolInApp)
	targets := cast.GetCast().GetTargets()
	if len(targets) != 3 {
		t.Fatalf("rays = %d, want 3 attack rolls", len(targets))
	}
	want := []playv1.AttackOutcome{playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT, playv1.AttackOutcome_ATTACK_OUTCOME_MISS, playv1.AttackOutcome_ATTACK_OUTCOME_HIT}
	faces := []int32{20, 2, 10}
	for i, tg := range targets {
		if tg.GetCombatantId() != goblin || tg.GetOutcome() != want[i] || tg.GetAttackRoll().GetFaces()[0] != faces[i] {
			t.Errorf("ray %d = %v, want the goblin, outcome %v, d20 %d", i+1, tg, want[i], faces[i])
		}
	}
	pending := cast.GetCast().GetPendingDamages()
	if len(pending) != 2 || targets[0].GetPendingDamageId() != pending[0].GetId() || targets[2].GetPendingDamageId() != pending[1].GetId() || targets[1].GetPendingDamageId() != "" {
		t.Fatalf("pending damages = %v, want one for the critical ray and one for the hit, in ray order", pending)
	}
	if pending[0].GetDiceCount() != 4 || pending[1].GetDiceCount() != 2 || pending[0].GetDiceSides() != 6 || pending[1].GetDiceSides() != 6 {
		t.Errorf("damages = %v, want 4d6 for the critical ray and 2d6 for the hit", pending)
	}
	if got := usedSlots(a.vitals(t, a.pens), 2); got != 1 {
		t.Errorf("2nd circle slots used = %d, want 1 for the whole cast", got)
	}
}

// TestScorchingRayRaysSplitBetweenTargets: the rays shared out over several targets, one or more each.
func TestScorchingRayRaysSplitBetweenTargets(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)
	a.h.roller.queue(10, 10, 10, 10, 10, 10, 10, 10) // two d20 for each ray, the caster has an enemy next to it
	rays := map[string]int32{"Goblin 1": 3, "Goblin 2": 1}
	cast := a.mustCast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(3), rayTargets(t, a, rays, "Goblin 1", "Goblin 2"), poolInApp)
	var got []string
	for _, tg := range cast.GetCast().GetTargets() {
		got = append(got, tg.GetCombatantId())
		if tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || tg.GetPendingDamageId() == "" {
			t.Errorf("ray at %s = %v, want a hit with damage to roll", tg.GetCombatantId(), tg)
		}
	}
	g1, g2 := a.id(t, "Goblin 1"), a.id(t, "Goblin 2")
	if want := []string{g1, g1, g1, g2}; len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("rays' targets = %v, want three at Goblin 1 then one at Goblin 2", got)
	}
	if n := len(cast.GetCast().GetPendingDamages()); n != 4 {
		t.Errorf("pending damages = %d, want one for each ray", n)
	}
}

// TestScorchingRayDamagesLandOneByOne: the damage of each ray at the same target is its own roll and lands
// by itself, never settled together with another ray.
func TestScorchingRayDamagesLandOneByOne(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)
	a.h.roller.queue(10, 10, 10, 10, 10, 10)
	cast := a.mustCast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), rayTargets(t, a, map[string]int32{"Goblin 1": 3}, "Goblin 1"), poolInApp)
	first := cast.GetCast().GetPendingDamages()[0]
	a.h.roller.queue(1, 1)
	res := a.mustDamage(t, a.ana, e, first.GetId(), inAppDamage)
	if n := len(res.GetCastPendingDamages()); n != 0 {
		t.Errorf("settled pending damages of the cast = %d, want the one roll alone (%v)", n, res.GetCastPendingDamages())
	}
}

// TestScorchingRayRefusals: the shares the spell does not allow are refused before anything is spent.
func TestScorchingRayRefusals(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 4)
	tests := map[string]struct {
		slot    int32
		targets []*playv1.SpellTarget
	}{
		"more rays than the circle gives":      {2, rayTargets(t, a, map[string]int32{"Goblin 1": 4}, "Goblin 1")},
		"more rays over several targets":       {2, rayTargets(t, a, map[string]int32{"Goblin 1": 2, "Goblin 2": 2}, "Goblin 1", "Goblin 2")},
		"more targets than rays":               {2, a.at(t, "Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4")},
		"no target":                            {2, nil},
		"a negative number of rays":            {2, rayTargets(t, a, map[string]int32{"Goblin 1": -1}, "Goblin 1")},
		"more rays than the upcast circle has": {3, rayTargets(t, a, map[string]int32{"Goblin 1": 5}, "Goblin 1")},
	}
	for name, tc := range tests {
		_, err := a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(tc.slot), tc.targets, poolInApp)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	for lvl := int32(2); lvl <= 3; lvl++ {
		if got := usedSlots(a.vitals(t, a.pens), lvl); got != 0 {
			t.Errorf("circle %d slots used = %d after refused casts, want 0", lvl, got)
		}
	}
}

// TestScorchingRayTypedD20FitsOneRay: a typed d20 face is one roll, so it cannot stand for several rays.
func TestScorchingRayTypedD20FitsOneRay(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)
	_, err := a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), rayTargets(t, a, map[string]int32{"Goblin 1": 2}, "Goblin 1"), d20Face(12))
	wantCode(t, "a typed d20 for two rays at one target", err, connect.CodeInvalidArgument)
}
