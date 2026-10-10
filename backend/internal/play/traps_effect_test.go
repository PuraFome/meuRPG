package play

import (
	"slices"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The arithmetic of a trap's effect (MR-035, D5) needs no database: the dice are
// scripted.

// scripted gives the faces queued, in order.
type scripted struct{ faces []int }

func (s *scripted) d20() (int, error) {
	f := s.faces[0]
	s.faces = s.faces[1:]
	return f, nil
}

func (s *scripted) dice(e dice.Expr) (dice.Result, error) {
	res := dice.Result{Expr: e, Modifier: e.Modifier, Total: e.Modifier}
	for range e.Count {
		f := s.faces[0]
		s.faces = s.faces[1:]
		res.Faces = append(res.Faces, f)
		res.Total += f
	}
	return res, nil
}

func TestMR035_TheEffectsArithmetic(t *testing.T) {
	t.Parallel()
	dmg := func(d, kind string) *rulesv1.TrapDamage { return &rulesv1.TrapDamage{Dice: d, DamageTypeKey: kind} }
	saveEffect := func(applies rulesv1.TrapSaveApplies, pass rulesv1.TrapPassOutcome) *rulesv1.TrapEffect {
		return &rulesv1.TrapEffect{
			Attack: &rulesv1.TrapAttack{Bonus: 5, Count: 2, Damage: dmg("1d6", "damage-type:piercing")},
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_CONSTITUTION, Dc: 13, AppliesTo: applies,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{dmg("2d4", "damage-type:poison")}, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned"}},
				OnPass: pass,
			},
		}
	}
	targets := []trapTarget{{id: "a", armorClass: 12, save: 2, saveKnown: true}, {id: "b", armorClass: 18}}

	t.Run("the attacks go round, a natural 20 doubles the dice and a natural 1 misses", func(t *testing.T) {
		t.Parallel()
		// Attack 1 at a: 20 (critical, 1d6 doubled: 2 + 3), attack 2 at b: 1 (misses).
		s := &scripted{faces: []int{20, 2, 3, 1}}
		out, err := resolveTrap(&rulesv1.TrapEffect{Attack: saveEffect(0, 0).Attack}, targets, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		a, b := out[0], out[1]
		if len(a.attacks) != 1 || !a.attacks[0].hit || !a.attacks[0].critical || a.attacks[0].total != 25 || len(a.damages) != 1 || a.damages[0].amount != 5 || a.damages[0].count != 2 || !a.damages[0].critical {
			t.Errorf("a = %+v, want a critical hit (25) for 2d6 = 5", a)
		}
		if len(b.attacks) != 1 || b.attacks[0].hit || len(b.damages) != 0 {
			t.Errorf("b = %+v, want a miss with no damage", b)
		}
		if len(s.faces) != 0 {
			t.Errorf("faces left = %v, want none", s.faces)
		}
	})

	t.Run("under the table's \"máximo mais uma rolagem\" a critical rolls once and keeps the maximum", func(t *testing.T) {
		t.Parallel()
		// Attack 1 at a: 20 (critical, 1d6: roll 2, plus the maximum 6), attack 2 at b: 1 (misses).
		s := &scripted{faces: []int{20, 2, 1}}
		out, err := resolveTrap(&rulesv1.TrapEffect{Attack: saveEffect(0, 0).Attack}, targets, combat.CriticalMaxPlusRoll, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		d := out[0].damages[0]
		if d.count != 1 || d.criticalMax != 6 || !d.maxRule || d.rollTotal != 8 || d.amount != 8 || !d.critical || len(s.faces) != 0 {
			t.Errorf("the critical damage = %+v, want one die, 6 kept, a roll of 2 + 6 = 8", d)
		}
	})

	t.Run("the save is asked of every creature caught; a pass halves, rounded down", func(t *testing.T) {
		t.Parallel()
		// Attacks: a 1 (miss), b 1 (miss). Saves: a 10 + 2 = 12 < 13 fails (2d4: 3 + 4 = 7);
		// b 13 + 0 = 13 passes (2d4: 1 + 4 = 5, half is 2).
		s := &scripted{faces: []int{1, 1, 10, 3, 4, 13, 1, 4}}
		out, err := resolveTrap(saveEffect(rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF), targets, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		a, b := out[0], out[1]
		if len(a.saves) != 1 || a.saves[0].saved || a.saves[0].total != 12 || len(a.damages) != 1 || a.damages[0].amount != 7 || a.damages[0].half || !slices.Equal(a.conditions, []string{"condition:poisoned"}) {
			t.Errorf("a = %+v, want a failed save (12), 7 poison, poisoned", a)
		}
		if len(b.saves) != 1 || !b.saves[0].saved || b.saves[0].known || len(b.damages) != 1 || b.damages[0].amount != 2 || b.damages[0].rollTotal != 5 || !b.damages[0].half || len(b.conditions) != 0 {
			t.Errorf("b = %+v, want a passed save with no bonus, half of 5 = 2, no condition", b)
		}
	})

	t.Run("a pass that avoids it, and a save asked only of the creatures hit", func(t *testing.T) {
		t.Parallel()
		// a is hit (10 + 5 = 15 >= 12; 1d6: 4), b is missed (2 + 5 = 7 < 18): only a saves, and
		// passes (15 + 2): nothing.
		s := &scripted{faces: []int{10, 4, 2, 15}}
		out, err := resolveTrap(saveEffect(rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE), targets, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		a, b := out[0], out[1]
		if len(b.saves) != 0 || len(b.damages) != 0 || len(b.conditions) != 0 {
			t.Errorf("b = %+v, want no save and nothing taken", b)
		}
		if len(a.saves) != 1 || !a.saves[0].saved || len(a.damages) != 1 || a.damages[0].amount != 4 || len(a.conditions) != 0 {
			t.Errorf("a = %+v, want the attack's 4 only, the save passed", a)
		}
	})

	t.Run("damage that always lands and conditions reach everyone; a flat number is no dice", func(t *testing.T) {
		t.Parallel()
		effect := &rulesv1.TrapEffect{
			Damage:     []*rulesv1.TrapDamage{dmg("1", "damage-type:piercing"), dmg("2d6", "damage-type:bludgeoning")},
			Conditions: []*rulesv1.TrapCondition{{ConditionKey: "condition:prone"}, {ConditionKey: "condition:prone"}},
		}
		s := &scripted{faces: []int{6, 6, 1, 2}}
		out, err := resolveTrap(effect, targets, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		for i, o := range out {
			if len(o.damages) != 2 || o.damages[0].count != 0 || o.damages[0].amount != 1 || o.damages[1].count != 2 || !slices.Equal(o.conditions, []string{"condition:prone"}) {
				t.Errorf("target %d = %+v, want 1 flat and 2d6, prone once", i, o)
			}
		}
		if out[0].damages[1].amount != 12 || out[1].damages[1].amount != 3 {
			t.Errorf("the 2d6 = %d and %d, want 12 and 3 (each creature's own roll)", out[0].damages[1].amount, out[1].damages[1].amount)
		}
	})

	t.Run("a trap that asks the save of the creatures hit asks it once for each hit", func(t *testing.T) {
		t.Parallel()
		// Three darts at a: 15, 12 and 1 against an armor class of 12 (bonus 5): two hits (1d6: 2, 5)
		// and a miss, so two saves, 10 + 2 fails (2d4: 1 + 1) and 18 + 2 passes (nothing).
		e := saveEffect(rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)
		e.Attack.Count = 3
		s := &scripted{faces: []int{15, 2, 12, 5, 1, 10, 1, 1, 18}}
		out, err := resolveTrap(e, targets[:1], combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		a := out[0]
		if len(a.attacks) != 3 || len(a.saves) != 2 || a.saves[0].saved || !a.saves[1].saved {
			t.Fatalf("a = %+v, want 3 darts and 2 saves (one failed, one passed)", a)
		}
		// 2 + 5 from the hits, 1 + 1 from the failed save.
		if len(a.damages) != 3 || a.damages[0].amount != 2 || a.damages[1].amount != 5 || a.damages[2].amount != 2 || !slices.Equal(a.conditions, []string{"condition:poisoned"}) {
			t.Errorf("damages = %+v, conditions %v; want 2, 5 and 2, poisoned", a.damages, a.conditions)
		}
		if len(s.faces) != 0 {
			t.Errorf("faces left = %v, want none", s.faces)
		}
	})

	t.Run("an empty effect does nothing", func(t *testing.T) {
		t.Parallel()
		out, err := resolveTrap(&rulesv1.TrapEffect{}, targets, combat.CriticalDoubledDice, (&scripted{}).d20, (&scripted{}).dice, nil)
		if err != nil || len(out) != 2 || len(out[0].damages) != 0 || len(out[0].saves) != 0 || len(out[0].attacks) != 0 {
			t.Errorf("resolveTrap() = %+v, %v; want nothing done", out, err)
		}
	})
}
