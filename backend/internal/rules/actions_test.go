package rules

import "testing"

// toren is the table's fighter (the canonical fight): STR 16, a battleaxe.
func toren() Build {
	b := standard("class:fighter", 3)
	b.Weapons = []string{"equipment:battleaxe"}
	return b
}

func TestResourcesAndActions(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)

	d := Derive(toren(), c)
	var sw *Resource
	for i := range d.Resources {
		if d.Resources[i].Key == "second_wind" {
			sw = &d.Resources[i]
		}
	}
	if sw == nil || sw.Max != 1 || sw.Recharge != RechargeShortRest || sw.NamePT != "Retomar o Fôlego" || sw.Source != "feature:second-wind" {
		t.Errorf("Second Wind = %+v", sw)
	}
	var act *Action
	for i := range d.Actions {
		if d.Actions[i].Key == "feature:second-wind" {
			act = &d.Actions[i]
		}
	}
	if act == nil || act.Economy != EconomyBonusAction || act.Resource != "second_wind" {
		t.Errorf("Second Wind action = %+v", act)
	}

	// Action Surge comes at fighter level 2, with its own resource and no
	// grant_action of its own.
	if !hasResource(d, "action_surge") {
		t.Errorf("a level 3 fighter has no Action Surge: %+v", d.Resources)
	}
	if hasResource(Derive(standard("class:fighter", 1), c), "action_surge") {
		t.Error("a level 1 fighter has Action Surge")
	}
	// Formulas run at the character's level.
	for level, want := range map[int]int{1: 2, 3: 3, 6: 4, 12: 5, 17: 6} {
		if got := resourceMax(Derive(standard("class:barbarian", level), c), "rage"); got != want {
			t.Errorf("rage at level %d = %d, want %d", level, got, want)
		}
	}
	if got := resourceMax(Derive(standard("class:monk", 5), c), "ki"); got != 5 {
		t.Errorf("ki at monk 5 = %d", got)
	}

	// The wizard has Arcane Recovery once a day and no feature actions.
	p := Derive(pensantus(), c)
	if got := resourceMax(p, "arcane_recovery"); got != 1 || len(p.Actions) != 0 {
		t.Errorf("Pensantus: arcane recovery %d, actions %+v", got, p.Actions)
	}

	// The standard actions are the same for everyone, in this order.
	want := []string{"Atacar", "Conjurar uma magia", "Disparada", "Desengajar", "Esquivar", "Ajudar", "Esconder", "Preparar", "Procurar", "Usar um objeto"}
	if len(p.StandardActions) != len(want) {
		t.Fatalf("standard actions = %+v", p.StandardActions)
	}
	for i, a := range p.StandardActions {
		if a.NamePT != want[i] || a.Economy != EconomyAction || a.Resource != "" {
			t.Errorf("standard action %d = %+v, want %s", i, a, want[i])
		}
	}
}

func hasResource(d Derived, key string) bool { return resourceMax(d, key) > 0 }

func resourceMax(d Derived, key string) int {
	for _, r := range d.Resources {
		if r.Key == key {
			return r.Max
		}
	}
	return 0
}

// TestAttackDice: the structured damage is the string, as numbers.
func TestAttackDice(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d := Derive(toren(), c)
	a, ok := attackOf(d, "equipment:battleaxe")
	if !ok || a.AttackBonus != 5 || a.Damage != "1d8+3" || a.DamageDice != (DiceFormula{Count: 1, Sides: 8, Bonus: 3}) ||
		a.VersatileDice != (DiceFormula{Count: 1, Sides: 10, Bonus: 3}) {
		t.Errorf("battleaxe = %+v, want +5 1d8+3, versatile 1d10+3", a)
	}
	p := Derive(pensantus(), c)
	if fb, _ := attackOf(p, "spell:fire-bolt"); fb.AttackBonus != 6 || fb.DamageDice != (DiceFormula{Count: 1, Sides: 10}) {
		t.Errorf("fire bolt = %+v", fb)
	}
	if qs, _ := attackOf(p, "equipment:quarterstaff"); qs.DamageDice != (DiceFormula{Count: 1, Sides: 6, Bonus: 1}) {
		t.Errorf("quarterstaff = %+v", qs)
	}
}
