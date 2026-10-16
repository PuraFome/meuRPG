package play

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Characterization of resolveTrap beyond the arithmetic of traps_effect_test.go: the modes
// of the d20, a save that fails without rolling, the lucky creature, the dice an effect adds
// to a save, and every place a die or a damage can fail. The dice are scripted, so the
// whole outcome of each case is compared to what it was before the function was split.

var errTrapDice = errors.New("the dice broke")

// failing is a scripted roller that fails on its n-th call (1 is the first d20 or the first
// damage roll); 0 never fails.
type failing struct {
	scripted
	n, calls int
}

func (f *failing) tick() error {
	f.calls++
	if f.n != 0 && f.calls == f.n {
		return errTrapDice
	}
	return nil
}

func (f *failing) d20() (int, error) {
	if err := f.tick(); err != nil {
		return 0, err
	}
	return f.scripted.d20()
}

func (f *failing) dice(e dice.Expr) (dice.Result, error) {
	if err := f.tick(); err != nil {
		return dice.Result{}, err
	}
	return f.scripted.dice(e)
}

func trapDmg(d, kind string) *rulesv1.TrapDamage {
	return &rulesv1.TrapDamage{Dice: d, DamageTypeKey: kind}
}

func TestTrapResolveModesAndSaves(t *testing.T) {
	t.Parallel()
	const pierce, poison = "damage-type:piercing", "damage-type:poison"
	save := func(applies rulesv1.TrapSaveApplies, pass rulesv1.TrapPassOutcome) *rulesv1.TrapSaveEffect {
		return &rulesv1.TrapSaveEffect{
			Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: applies, OnPass: pass,
			OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{trapDmg("1d4", poison)}, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned"}},
		}
	}
	caught := rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT

	t.Run("an attack with advantage keeps the higher die and says the other and which came first", func(t *testing.T) {
		t.Parallel()
		s := &scripted{faces: []int{5, 17, 3}}
		target := trapTarget{id: "a", armorClass: 15, attackMode: combat.ModeAdvantage}
		out, err := resolveTrap(&rulesv1.TrapEffect{Attack: &rulesv1.TrapAttack{Bonus: 2, Count: 1, Damage: trapDmg("1d6", pierce)}}, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		want := []trapOutcome{{
			target:  target,
			attacks: []trapAttackRoll{{d20: 17, bonus: 2, total: 19, armorClass: 15, hit: true, other: 5, counted: 1, mode: combat.ModeAdvantage}},
			damages: []trapDamageRoll{{count: 1, sides: 6, faces: []int{3}, damageType: pierce, rollTotal: 3, amount: 3}},
		}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("resolveTrap() = %+v, want %+v", out, want)
		}
	})

	t.Run("a save with disadvantage keeps the lower die", func(t *testing.T) {
		t.Parallel()
		s := &scripted{faces: []int{15, 4, 2}}
		target := trapTarget{id: "a", save: 1, saveKnown: true, saveMode: combat.ModeDisadvantage}
		out, err := resolveTrap(&rulesv1.TrapEffect{Save: save(caught, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		want := []trapOutcome{{
			target:     target,
			saves:      []trapSaveRoll{{d20: 4, bonus: 1, total: 5, dc: 12, known: true, other: 15, counted: 1, mode: combat.ModeDisadvantage}},
			damages:    []trapDamageRoll{{count: 1, sides: 4, faces: []int{2}, damageType: poison, rollTotal: 2, amount: 2}},
			conditions: []string{"condition:poisoned"},
		}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("resolveTrap() = %+v, want %+v", out, want)
		}
	})

	t.Run("a creature that fails the save without rolling uses no die for it", func(t *testing.T) {
		t.Parallel()
		s := &scripted{faces: []int{6}}
		target := trapTarget{id: "a", save: 3, saveKnown: true, saveAuto: true}
		out, err := resolveTrap(&rulesv1.TrapEffect{Save: save(caught, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF)}, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		want := []trapOutcome{{
			target:     target,
			saves:      []trapSaveRoll{{bonus: 3, dc: 12, known: true, auto: true}},
			damages:    []trapDamageRoll{{count: 1, sides: 4, faces: []int{6}, damageType: poison, rollTotal: 6, amount: 6}},
			conditions: []string{"condition:poisoned"},
		}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("resolveTrap() = %+v, want %+v", out, want)
		}
	})

	t.Run("a lucky creature rolls its 1 again and the new die stands", func(t *testing.T) {
		t.Parallel()
		s := &scripted{faces: []int{1, 13}}
		target := trapTarget{id: "a", save: 0, saveKnown: true, lucky: true}
		out, err := resolveTrap(&rulesv1.TrapEffect{Save: save(caught, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		want := []trapOutcome{{
			target: target,
			saves:  []trapSaveRoll{{d20: 13, total: 13, dc: 12, known: true, saved: true, lucky: &luckyReroll{Index: 0, From: 1, To: 13}}},
		}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("resolveTrap() = %+v, want %+v", out, want)
		}
	})

	t.Run("the dice an effect adds to the save are rolled when the caller gives a roller, and left alone when not", func(t *testing.T) {
		t.Parallel()
		blessing := []effectDie{{Key: "effect:bless", Faces: 4, Sign: 1}}
		rolled := []effectDie{{Key: "effect:bless", Faces: 4, Sign: 1, Face: 3}}
		target := trapTarget{id: "a", save: 2, saveKnown: true, saveDice: blessing}
		effect := &rulesv1.TrapEffect{Save: save(caught, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}
		var asked []effectDie
		extra := func(dd []effectDie) ([]effectDie, int, error) {
			asked = dd
			return rolled, 3, nil
		}
		s := &scripted{faces: []int{8}}
		out, err := resolveTrap(effect, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, extra)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		want := trapSaveRoll{d20: 8, bonus: 5, total: 13, dc: 12, known: true, saved: true, extra: rolled}
		if !reflect.DeepEqual(asked, blessing) || len(out[0].saves) != 1 || !reflect.DeepEqual(out[0].saves[0], want) {
			t.Errorf("save = %+v (asked %v), want %+v", out[0].saves, asked, want)
		}
		s = &scripted{faces: []int{8, 2}} // 8 + 2 fails: the 1d4 of the failure
		out, err = resolveTrap(effect, []trapTarget{target}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap(no roller) error = %v", err)
		}
		if want := (trapSaveRoll{d20: 8, bonus: 2, total: 10, dc: 12, known: true}); len(out[0].saves) != 1 || !reflect.DeepEqual(out[0].saves[0], want) {
			t.Errorf("save with no roller = %+v, want %+v", out[0].saves, want)
		}
	})

	t.Run("the attacks wrap round the creatures and a pass of half rolls the failure's damage again", func(t *testing.T) {
		t.Parallel()
		// Three darts at two creatures: dart 1 at a (hit, 1d6: 4), dart 2 at b (miss), dart 3 at a
		// (hit, 1d6: 2). The save is asked once of each: a passes (half of 1d4: 3 is 1), b fails (1d4: 2).
		a, b := trapTarget{id: "a", armorClass: 10, save: 4, saveKnown: true}, trapTarget{id: "b", armorClass: 10}
		e := &rulesv1.TrapEffect{
			Attack: &rulesv1.TrapAttack{Bonus: 0, Count: 3, Damage: trapDmg("1d6", pierce)},
			Save:   save(caught, rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF),
		}
		// Order of the faces: dart 1 (14), its damage (4), dart 2 (2: miss on b), dart 3 (11: hit on a), its
		// damage (2), then the saves: a 18 (+4 passes), its half damage 3, b 3 (fails), its damage 2.
		s := &scripted{faces: []int{14, 4, 2, 11, 2, 18, 3, 3, 2}}
		out, err := resolveTrap(e, []trapTarget{a, b}, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil {
			t.Fatalf("resolveTrap() error = %v", err)
		}
		if len(s.faces) != 0 {
			t.Fatalf("faces left = %v, want none", s.faces)
		}
		wantA := trapOutcome{
			target: a,
			attacks: []trapAttackRoll{
				{d20: 14, total: 14, armorClass: 10, hit: true},
				{d20: 11, total: 11, armorClass: 10, hit: true},
			},
			saves: []trapSaveRoll{{d20: 18, bonus: 4, total: 22, dc: 12, known: true, saved: true}},
			damages: []trapDamageRoll{
				{count: 1, sides: 6, faces: []int{4}, damageType: pierce, rollTotal: 4, amount: 4},
				{count: 1, sides: 6, faces: []int{2}, damageType: pierce, rollTotal: 2, amount: 2},
				{count: 1, sides: 4, faces: []int{3}, damageType: poison, rollTotal: 3, amount: 1, half: true},
			},
		}
		wantB := trapOutcome{
			target:     b,
			attacks:    []trapAttackRoll{{d20: 2, total: 2, armorClass: 10}},
			saves:      []trapSaveRoll{{d20: 3, total: 3, dc: 12}},
			damages:    []trapDamageRoll{{count: 1, sides: 4, faces: []int{2}, damageType: poison, rollTotal: 2, amount: 2}},
			conditions: []string{"condition:poisoned"},
		}
		if !reflect.DeepEqual(out[0], wantA) || !reflect.DeepEqual(out[1], wantB) {
			t.Errorf("resolveTrap() =\n a %+v\n b %+v\nwant\n a %+v\n b %+v", out[0], out[1], wantA, wantB)
		}
	})

	t.Run("no creature caught rolls nothing", func(t *testing.T) {
		t.Parallel()
		s := &scripted{}
		out, err := resolveTrap(&rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{trapDmg("2d6", pierce)}}, nil, combat.CriticalDoubledDice, s.d20, s.dice, nil)
		if err != nil || len(out) != 0 {
			t.Errorf("resolveTrap() = %+v, %v; want nothing", out, err)
		}
	})
}

func TestTrapResolveFailures(t *testing.T) {
	t.Parallel()
	const pierce = "damage-type:piercing"
	attack := &rulesv1.TrapAttack{Bonus: 20, Count: 1, Damage: trapDmg("1d6", pierce)}
	saveWith := func(pass rulesv1.TrapPassOutcome) *rulesv1.TrapSaveEffect {
		return &rulesv1.TrapSaveEffect{
			Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 30, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT, OnPass: pass,
			OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{trapDmg("1d4", pierce)}},
		}
	}
	target := trapTarget{id: "a", armorClass: 10, saveKnown: true, saveDice: []effectDie{{Key: "effect:bless", Faces: 4, Sign: 1}}}
	for _, tc := range []struct {
		name    string
		effect  *rulesv1.TrapEffect
		failAt  int // the call of the roller that fails (a d20 or a damage roll)
		extra   bool
		wantErr string
	}{
		{"the d20 of an attack", &rulesv1.TrapEffect{Attack: attack}, 1, false, "the dice broke"},
		{"the damage dice of a hit", &rulesv1.TrapEffect{Attack: attack}, 2, false, "the dice broke"},
		{"damage that always lands", &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{trapDmg("1d6", pierce)}}, 1, false, "the dice broke"},
		{"the d20 of a save", &rulesv1.TrapEffect{Save: saveWith(rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, 1, false, "the dice broke"},
		{"the dice an effect adds to a save", &rulesv1.TrapEffect{Save: saveWith(rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, 0, true, "the extra dice broke"},
		{"the damage of a failed save", &rulesv1.TrapEffect{Save: saveWith(rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, 2, false, "the dice broke"},
		{"a damage that does not read", &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{trapDmg("lots", pierce)}}, 0, false, `read the trap's damage "lots": `},
		{"a flat damage over 1000", &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{trapDmg("1001", pierce)}}, 0, false, `read the trap's damage "1001": a flat trap damage is 0 to 1000`},
		{"a negative flat damage", &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{trapDmg("-1", pierce)}}, 0, false, `read the trap's damage "-1": a flat trap damage is 0 to 1000`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &failing{scripted: scripted{faces: []int{10, 1, 1, 1, 1}}, n: tc.failAt}
			var extra func([]effectDie) ([]effectDie, int, error)
			if tc.extra {
				extra = func([]effectDie) ([]effectDie, int, error) { return nil, 0, errors.New("the extra dice broke") }
			}
			out, err := resolveTrap(tc.effect, []trapTarget{target}, combat.CriticalDoubledDice, f.d20, f.dice, extra)
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) || out != nil {
				t.Errorf("resolveTrap() = %+v, %v; want no outcomes and the error %q", out, err, tc.wantErr)
			}
		})
	}

	t.Run("the damage of a pass that halves it fails like any other", func(t *testing.T) {
		t.Parallel()
		// The save passes (d20 30 against DC 30), and the half damage's roll is the first damage roll.
		f := &failing{scripted: scripted{faces: []int{30}}}
		calls := 0
		rollDice := func(dice.Expr) (dice.Result, error) {
			calls++
			return dice.Result{}, errTrapDice
		}
		_, err := resolveTrap(&rulesv1.TrapEffect{Save: saveWith(rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF)}, []trapTarget{{id: "a", saveKnown: true}}, combat.CriticalDoubledDice, f.d20, rollDice, nil)
		if !errors.Is(err, errTrapDice) || calls != 1 {
			t.Errorf("resolveTrap() error = %v after %d damage rolls; want the dice error at the first", err, calls)
		}
	})

	t.Run("a lucky creature's second die can fail", func(t *testing.T) {
		t.Parallel()
		f := &failing{scripted: scripted{faces: []int{1}}, n: 2}
		_, err := resolveTrap(&rulesv1.TrapEffect{Save: saveWith(rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE)}, []trapTarget{{id: "a", lucky: true}}, combat.CriticalDoubledDice, f.d20, f.dice, nil)
		if !errors.Is(err, errTrapDice) {
			t.Errorf("resolveTrap() error = %v, want the dice error", err)
		}
	})

	t.Run("an attack's first die of two can fail", func(t *testing.T) {
		t.Parallel()
		f := &failing{scripted: scripted{faces: []int{10}}, n: 2}
		_, err := resolveTrap(&rulesv1.TrapEffect{Attack: attack}, []trapTarget{{id: "a", attackMode: combat.ModeAdvantage}}, combat.CriticalDoubledDice, f.d20, f.dice, nil)
		if !errors.Is(err, errTrapDice) {
			t.Errorf("resolveTrap() error = %v, want the dice error", err)
		}
	})
}
