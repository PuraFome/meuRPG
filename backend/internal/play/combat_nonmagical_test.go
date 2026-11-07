package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// A creature that resists the bludgeoning, piercing and slashing damage of nonmagical
// attacks (a grick) takes half of a rapier's piercing: the master reads the step, named
// for what it is, and the attacker's player reads the damage as rolled (RN-10, RN-20).
func TestNonmagicalResistanceHalvesAMundaneAttackAndTheMasterSeesTheStep(t *testing.T) {
	t.Parallel()
	a, e, grick := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, abilities(16, 12, 14, 10, 8), []string{rapier}, nil)
	}, "monster:grick")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, grick, d20(15))
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	line := func(u *user) *playv1.CombatLogDamage {
		for _, r := range a.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
					return en.GetDamage()
				}
			}
		}
		return nil
	}
	master := line(a.master)
	if master == nil {
		t.Fatal("the master reads no attack line")
	}
	steps := master.GetSteps()
	if len(steps) != 1 || steps[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE ||
		len(steps[0].GetSourceKeys()) != 1 || steps[0].GetSourceKeys()[0] != "monster:grick" || steps[0].GetBefore() != 9 || steps[0].GetAfter() != 4 {
		t.Fatalf("the master's steps = %v, want one resistance from the creature, 9 to 4", steps)
	}
	if got := steps[0].GetLabelPt(); !strings.HasPrefix(got, "Resistência a perfurante de ataque não mágico") {
		t.Errorf("the step reads %q, want a resistance to piercing from nonmagical attacks", got)
	}
	if player := line(a.caio); player == nil || len(player.GetSteps()) != 0 || player.GetAmount() != 9 {
		t.Errorf("the attacker's player reads %v, want the damage as rolled (9) and no step", player)
	}
	if hp, _, _ := a.hp(t, grick); hp != 27-4 {
		t.Errorf("the grick has %d hit points, want 23", hp)
	}
}

// A spell's damage is magical: the same creature takes the whole of a Fire Bolt, with
// no step, and the damage it does not resist is never reduced by the clause.
func TestNonmagicalResistanceDoesNotReachASpell(t *testing.T) {
	t.Parallel()
	a, e, grick := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, abilities(16, 12, 14, 10, 8), []string{rapier}, nil)
	}, "monster:grick")
	e = a.passTo(t, e, "Pensantus")
	hit := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, grick, d20(15))
	dmg := a.mustDamage(t, a.ana, e, hit.GetPendingDamage().GetId(), typedDamage(6)).GetPendingDamage()
	if len(dmg.GetSteps()) != 0 {
		t.Errorf("the steps = %v, want none for a spell", dmg.GetSteps())
	}
	if hp, _, _ := a.hp(t, grick); hp != 27-6 {
		t.Errorf("the grick has %d hit points, want 21", hp)
	}
}

// A wight resists the damage of nonmagical weapons that aren't silvered: a rapier is
// halved and the step says so (SRD 5.1, Wight).
func TestUnlessSilveredResistanceHalvesAnUnsilveredAttack(t *testing.T) {
	t.Parallel()
	a, e, wight := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, abilities(16, 12, 14, 10, 8), []string{rapier}, nil)
	}, "monster:wight")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, wight, d20(15))
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				continue
			}
			st := en.GetDamage().GetSteps()
			if len(st) != 1 || st[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || st[0].GetBefore() != 9 || st[0].GetAfter() != 4 {
				t.Fatalf("steps = %v, want one resistance 9 to 4", st)
			}
			if got := st[0].GetLabelPt(); !strings.HasPrefix(got, "Resistência a perfurante de ataque não mágico nem prateado") {
				t.Errorf("the step reads %q", got)
			}
			return
		}
	}
	t.Fatal("no attack line")
}

// The same blow from a silvered weapon is not reduced; a damage with no attack is not
// either; the adamantine entry follows the adamantine weapon.
func TestSettleStepsMaterialOrigin(t *testing.T) {
	t.Parallel()
	slash := "damage-type:slashing"
	silver := []combat.Modifier{{Kind: combat.StepResistance, Source: "monster:wight", DamageTypes: []string{slash}, Qualifier: combat.QualifierNonmagicalUnlessSilvered}}
	adamant := []combat.Modifier{{Kind: combat.StepResistance, Source: "monster:gargoyle", DamageTypes: []string{slash}, Qualifier: combat.QualifierNonmagicalUnlessAdamantine}}
	tests := []struct {
		name string
		mods []combat.Modifier
		from combat.Origin
		want int32
	}{
		{"unsilvered", silver, combat.Origin{Attack: true}, 4},
		{"silvered", silver, combat.Origin{Attack: true, Silvered: true}, 9},
		{"silver entry, adamantine weapon", silver, combat.Origin{Attack: true, Adamantine: true}, 4},
		{"no attack", silver, combat.Origin{}, 9},
		{"non-adamantine", adamant, combat.Origin{Attack: true, Silvered: true}, 4},
		{"adamantine", adamant, combat.Origin{Attack: true, Adamantine: true}, 9},
	}
	for _, tt := range tests {
		if got := settleSteps(map[string]int{slash: 9}, tt.mods, nil, tt.from).amount; got != tt.want {
			t.Errorf("%s: amount = %d, want %d", tt.name, got, tt.want)
		}
	}
	names := func(s string) string { return s }
	pt := func(s string) string { return s }
	for q, want := range map[string]string{
		combat.QualifierNonmagicalUnlessSilvered:   "Resistência a x de ataque não mágico nem prateado",
		combat.QualifierNonmagicalUnlessAdamantine: "Resistência a x de ataque não mágico nem adamantino",
	} {
		if got := stepLabel(combat.Step{Kind: combat.StepResistance, DamageType: "x", Qualifier: q}, names, pt); got != want {
			t.Errorf("label = %q, want %q", got, want)
		}
	}
}
