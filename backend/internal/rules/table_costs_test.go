package rules

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// The entries of these tests are invented: the table's feats carry a resource and
// the action or the hook that uses it.
const (
	reserveFeat    = "feat:reserva-de-teste" + tableSuffix
	breathFeat     = "feat:folego-da-mesa" + tableSuffix
	vigorFeat      = "feat:vigor-de-nivel" + tableSuffix
	initiativeFeat = "feat:reserva-de-iniciativa" + tableSuffix
	hooksFeat      = "feat:ganchos-da-mesa" + tableSuffix
)

// resourceEffect is a resource of the tests' feats.
func resourceEffect(name, limit, recharge string) Effect {
	return Effect{Type: "resource", Resource: name, Max: limit, Recharge: recharge}
}

// featWith writes a feat of effects, checked as the editor does (strict).
func featWith(key string, effects ...Effect) Overlay {
	return Overlay{Revision: 1, Strict: []string{key}, Feats: []TableFeat{{
		TableEntry: TableEntry{Key: key, NamePT: "Teste"}, DescPT: []string{"Texto."}, Effects: effects,
	}}}
}

// violationAt says the error is an OverlayError with a violation at field (the path
// under feats[0].effects[i]) for the reason.
func violationAt(t *testing.T, err error, field, reason string) {
	t.Helper()
	oe, ok := errors.AsType[*OverlayError](err)
	if !ok {
		t.Fatalf("With() error = %v, want an OverlayError at %s", err, field)
	}
	for _, v := range oe.Violations() {
		if v.Field == field && v.Reason == reason {
			return
		}
	}
	t.Errorf("violations = %+v, want one at %s (%s)", oe.Violations(), field, reason)
}

// deriveWithOverlay derives the wizard of the other tests with the feats of the overlay.
func deriveWithOverlay(t *testing.T, o Overlay) Derived {
	t.Helper()
	c, err := loadForTest(t).With(o)
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	b := pensantus()
	for _, f := range o.Feats {
		b.Feats = append(b.Feats, f.Key)
	}
	return Derive(b, c)
}

// grantedActionOf is the action a feat grants.
func grantedActionOf(t *testing.T, d Derived, source string) Action {
	t.Helper()
	i := slices.IndexFunc(d.Actions, func(a Action) bool { return a.Source == source })
	if i < 0 {
		t.Fatalf("no action from %s in %+v", source, d.Actions)
	}
	return d.Actions[i]
}

// TestCostedActionShapes: a grant_action that spends several points, chooses how
// many, heals or gives temporary hit points is accepted, and refused at the field
// that is wrong: a cost the resource can never pay, cost_max below cost, a heal with
// no dice, and the new fields on any other type.
func TestCostedActionShapes(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	pool := resourceEffect("reserva_de_teste", "4", RechargeLongRest)
	act := func(mut func(e *Effect)) Effect {
		e := Effect{Type: "grant_action", Economy: EconomyBonusAction, Resource: "reserva_de_teste"}
		mut(&e)
		return e
	}
	cases := []struct {
		name       string
		effects    []Effect
		field, why string // "" when it is accepted; the field under effects[1]
	}{
		{"a cost of 2", []Effect{pool, act(func(e *Effect) { e.Cost = "2" })}, "", ""},
		{"a cost by level", []Effect{pool, act(func(e *Effect) { e.Cost = "max(1, floor(level() / 2))" })}, "", ""},
		{"1 to 3 points that heal 1d6 each", []Effect{pool, act(func(e *Effect) {
			e.Cost, e.CostMax, e.Heal, e.ValuePerPoint = "1", "3", "1d6", "1d6"
		})}, "", ""},
		{"a heal with an ability modifier", []Effect{pool, act(func(e *Effect) { e.Heal, e.Ability = "1d8", "wis" })}, "", ""},
		{"temporary hit points by level, no resource", []Effect{{Type: "grant_action", Economy: EconomyFree, TempHP: "level()"}}, "", ""},
		{"temporary hit points per point", []Effect{pool, act(func(e *Effect) {
			e.Cost, e.CostMax, e.TempHP, e.ValuePerPoint = "1", "3", "2", "2"
		})}, "", ""},
		{"a cost above the maximum", []Effect{resourceEffect("reserva_de_teste", "2", RechargeLongRest), act(func(e *Effect) { e.Cost = "3" })}, ".cost", ReasonValue},
		{"a cost no level can pay", []Effect{resourceEffect("reserva_de_teste", "level()", RechargeLongRest), act(func(e *Effect) { e.Cost = "25" })}, ".cost", ReasonValue},
		{"a cost of 0", []Effect{pool, act(func(e *Effect) { e.Cost = "0" })}, ".cost", ReasonValue},
		{"a cost that does not compile", []Effect{pool, act(func(e *Effect) { e.Cost = "level( +" })}, ".cost", ReasonFormula},
		{"a cost with no resource", []Effect{{Type: "grant_action", Economy: EconomyAction, Cost: "2"}}, ".cost", ReasonValue},
		{"cost_max below cost", []Effect{pool, act(func(e *Effect) { e.Cost, e.CostMax = "3", "2" })}, ".cost_max", ReasonValue},
		{"cost_max with no resource", []Effect{{Type: "grant_action", Economy: EconomyAction, CostMax: "3"}}, ".cost_max", ReasonValue},
		{"cost_max that does not compile", []Effect{pool, act(func(e *Effect) { e.CostMax = "mod(" })}, ".cost_max", ReasonFormula},
		{"a heal with no dice (a number)", []Effect{pool, act(func(e *Effect) { e.Heal = "5" })}, ".heal", ReasonValue},
		{"a heal with no dice (a word)", []Effect{pool, act(func(e *Effect) { e.Heal = "muito" })}, ".heal", ReasonValue},
		{"a heal that adds MOD in the text", []Effect{pool, act(func(e *Effect) { e.Heal = "1d6 + MOD" })}, ".heal", ReasonValue},
		{"an unknown ability on a heal", []Effect{pool, act(func(e *Effect) { e.Heal, e.Ability = "1d6", "sorte" })}, ".ability", ReasonValue},
		{"an ability with no heal", []Effect{pool, act(func(e *Effect) { e.Ability = "wis" })}, ".ability", ReasonValue},
		{"temp_hp that does not compile", []Effect{pool, act(func(e *Effect) { e.TempHP = "level(" })}, ".temp_hp", ReasonFormula},
		{"value_per_point without cost_max", []Effect{pool, act(func(e *Effect) { e.Heal, e.ValuePerPoint = "1d6", "1d6" })}, ".value_per_point", ReasonValue},
		{"value_per_point of other sides", []Effect{pool, act(func(e *Effect) {
			e.Cost, e.CostMax, e.Heal, e.ValuePerPoint = "1", "3", "1d6", "1d8"
		})}, ".value_per_point", ReasonValue},
		{"dice per point and no heal", []Effect{pool, act(func(e *Effect) { e.Cost, e.CostMax, e.TempHP, e.ValuePerPoint = "1", "3", "2", "1d6" })}, ".value_per_point", ReasonValue},
		{"value_per_point that does not compile", []Effect{pool, act(func(e *Effect) {
			e.Cost, e.CostMax, e.TempHP, e.ValuePerPoint = "1", "3", "2", "level("
		})}, ".value_per_point", ReasonFormula},
		{"value_per_point with no result", []Effect{pool, act(func(e *Effect) { e.Cost, e.CostMax, e.ValuePerPoint = "1", "3", "2" })}, ".value_per_point", ReasonValue},
		{"an event on an action", []Effect{pool, act(func(e *Effect) { e.Event = EventLongRest })}, ".event", ReasonValue},
		{"restore on an action", []Effect{pool, act(func(e *Effect) { e.Restore = "reserva_de_teste" })}, ".restore", ReasonValue},
		{"heal_percent_max on an action", []Effect{pool, act(func(e *Effect) { e.HealPercentMax = 50 })}, ".heal_percent_max", ReasonValue},
		{"a cost on a modifier", []Effect{{Type: "modifier", Target: "ac", Mode: "add", Value: "1", Cost: "2"}}, ".cost", ReasonValue},
		{"cost_max on a resource", []Effect{{Type: "resource", Resource: "outra_reserva", Max: "1", Recharge: RechargeNone, CostMax: "2"}}, ".cost_max", ReasonValue},
		{"a heal on a note", []Effect{{Type: "note", TextPT: "Texto.", Heal: "1d6"}}, ".heal", ReasonValue},
		{"temp_hp on a sense", []Effect{{Type: "sense", Sense: "darkvision", RangeFt: 30, TempHP: "2"}}, ".temp_hp", ReasonValue},
		{"value_per_point on an extra attack", []Effect{{Type: "extra_attack", Count: 2, ValuePerPoint: "1d6"}}, ".value_per_point", ReasonValue},
		{"an event on a modifier", []Effect{{Type: "modifier", Target: "ac", Mode: "add", Value: "1", Event: EventLongRest}}, ".event", ReasonValue},
		{"restore on a resource", []Effect{{Type: "resource", Resource: "outra_reserva", Max: "1", Recharge: RechargeNone, Restore: "x"}}, ".restore", ReasonValue},
		{"heal_percent_max on a note", []Effect{{Type: "note", TextPT: "Texto.", HealPercentMax: 50}}, ".heal_percent_max", ReasonValue},
		{"an unknown recharge", []Effect{resourceEffect("reserva_de_teste", "4", "weekly")}, ".recharge", ReasonValue},
		{"every recharge is a known one: initiative", []Effect{resourceEffect("reserva_de_teste", "1", RechargeInitiative)}, "", ""},
		{"every recharge is a known one: restore one on a short rest", []Effect{resourceEffect("reserva_de_teste", "3", RechargeRestoreOneShort)}, "", ""},
		{"every recharge is a known one: restore one on a long rest", []Effect{resourceEffect("reserva_de_teste", "3", RechargeRestoreOneLong)}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := srd.With(featWith(reserveFeat, tc.effects...))
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			// The effect that is wrong is the last one.
			violationAt(t, err, fmt.Sprintf("feats[0].effects[%d]%s", len(tc.effects)-1, tc.field), tc.why)
		})
	}
}

// TestOnEventShapes: an on_event hook is accepted in each of its shapes and refused
// at the field that is wrong: an unknown or missing event, no result, a heal with no
// dice, both a heal and a share of the maximum, a resource the entry does not define,
// and the fields of other types.
func TestOnEventShapes(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	pool := resourceEffect("reserva_de_teste", "1", RechargeLongRest)
	hook := func(mut func(e *Effect)) Effect {
		e := Effect{Type: "on_event", Event: EventLongRest}
		mut(&e)
		return e
	}
	cases := []struct {
		name       string
		effects    []Effect
		field, why string
	}{
		{"temporary hit points after a long rest", []Effect{hook(func(e *Effect) { e.TempHP = "level()" })}, "", ""},
		{"temporary hit points after a short rest", []Effect{hook(func(e *Effect) { e.Event, e.TempHP = EventShortRest, "2" })}, "", ""},
		{"restore a use on a short rest", []Effect{pool, hook(func(e *Effect) { e.Event, e.Restore = EventShortRest, "reserva_de_teste" })}, "", ""},
		{"restore several uses", []Effect{resourceEffect("reserva_de_teste", "4", RechargeNone), hook(func(e *Effect) { e.Restore, e.Value = "reserva_de_teste", "2" })}, "", ""},
		{"stand up with half the maximum, once per long rest", []Effect{pool, hook(func(e *Effect) {
			e.Event, e.Resource, e.HealPercentMax = EventDeathSaveFailed, "reserva_de_teste", 50
		})}, "", ""},
		{"a heal of dice on dropping to zero", []Effect{hook(func(e *Effect) { e.Event, e.Heal, e.Ability = EventDropToZero, "1d4", "wis" })}, "", ""},
		{"a hook that spends two", []Effect{resourceEffect("reserva_de_teste", "3", RechargeLongRest), hook(func(e *Effect) {
			e.Event, e.Resource, e.Cost, e.TempHP = EventDropToZero, "reserva_de_teste", "2", "5"
		})}, "", ""},
		{"no event", []Effect{hook(func(e *Effect) { e.Event, e.TempHP = "", "2" })}, ".event", ReasonValue},
		{"an unknown event", []Effect{hook(func(e *Effect) { e.Event, e.TempHP = "on_sneeze", "2" })}, ".event", ReasonValue},
		{"no result", []Effect{hook(func(*Effect) {})}, ".type", ReasonValue},
		{"a heal with no dice", []Effect{hook(func(e *Effect) { e.Heal = "5" })}, ".heal", ReasonValue},
		{"a heal and a share of the maximum", []Effect{hook(func(e *Effect) { e.Heal, e.HealPercentMax = "1d6", 50 })}, ".heal_percent_max", ReasonValue},
		{"a share above 100", []Effect{hook(func(e *Effect) { e.HealPercentMax = 101 })}, ".heal_percent_max", ReasonValue},
		{"a negative share", []Effect{hook(func(e *Effect) { e.HealPercentMax = -5 })}, ".heal_percent_max", ReasonValue},
		{"temp_hp that does not compile", []Effect{hook(func(e *Effect) { e.TempHP = "level(" })}, ".temp_hp", ReasonFormula},
		{"restore of a resource the entry does not define", []Effect{hook(func(e *Effect) { e.Restore = "reserva_fantasma" })}, ".restore", ReasonReference},
		{"a resource the entry does not define", []Effect{hook(func(e *Effect) { e.Resource, e.TempHP = "reserva_fantasma", "2" })}, ".resource", ReasonReference},
		{"a cost with no resource", []Effect{hook(func(e *Effect) { e.Cost, e.TempHP = "2", "2" })}, ".cost", ReasonValue},
		{"a cost above the maximum", []Effect{pool, hook(func(e *Effect) { e.Resource, e.Cost, e.TempHP = "reserva_de_teste", "2", "2" })}, ".cost", ReasonValue},
		{"a value with no restore", []Effect{hook(func(e *Effect) { e.Value, e.TempHP = "2", "2" })}, ".value", ReasonValue},
		{"a restore count that does not compile", []Effect{pool, hook(func(e *Effect) { e.Restore, e.Value = "reserva_de_teste", "level(" })}, ".value", ReasonFormula},
		{"cost_max on a hook", []Effect{pool, hook(func(e *Effect) { e.Resource, e.CostMax, e.TempHP = "reserva_de_teste", "2", "2" })}, ".cost_max", ReasonValue},
		{"value_per_point on a hook", []Effect{hook(func(e *Effect) { e.ValuePerPoint, e.TempHP = "1d6", "2" })}, ".value_per_point", ReasonValue},
		{"an economy on a hook", []Effect{hook(func(e *Effect) { e.Economy, e.TempHP = EconomyAction, "2" })}, ".economy", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := srd.With(featWith(hooksFeat, tc.effects...))
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			violationAt(t, err, fmt.Sprintf("feats[0].effects[%d]%s", len(tc.effects)-1, tc.field), tc.why)
		})
	}
	// A class feature may have a hook too, and a feature's hook can restore the resource
	// of its class.
	if _, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{pool, hook(func(e *Effect) { e.Restore = "reserva_de_teste" })})}}); err != nil {
		t.Errorf("on a class feature: With() error = %v", err)
	}
}

// TestCostedActionsOnTheSheet: the derived action carries what a use needs: the
// cost, the range the player picks from, the heal and the temporary hit points.
func TestCostedActionsOnTheSheet(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, Overlay{Revision: 1, Feats: []TableFeat{
		{TableEntry: TableEntry{Key: reserveFeat, NamePT: "Reserva de Teste"}, Effects: []Effect{
			resourceEffect("reserva_de_teste", "4", RechargeLongRest),
			{Type: "grant_action", Economy: EconomyBonusAction, Resource: "reserva_de_teste", Cost: "2"},
		}},
		{TableEntry: TableEntry{Key: breathFeat, NamePT: "Fôlego da Mesa"}, Effects: []Effect{
			resourceEffect("folego_de_teste", "3", RechargeShortRest),
			{Type: "grant_action", Economy: EconomyAction, Resource: "folego_de_teste", Cost: "1", CostMax: "3", Heal: "1d6", ValuePerPoint: "1d6", Ability: "wis"},
		}},
		{TableEntry: TableEntry{Key: vigorFeat, NamePT: "Vigor de Nível"}, Effects: []Effect{
			{Type: "grant_action", Economy: EconomyFree, TempHP: "level()"},
		}},
	}})

	t.Run("a fixed cost of 2", func(t *testing.T) {
		t.Parallel()
		a := grantedActionOf(t, d, reserveFeat)
		if a.Cost != 2 || a.CostMax != 0 || a.Resource != "reserva_de_teste" || a.UnitCost() != 2 || a.ChoosesSpend() {
			t.Fatalf("action = %+v, want a fixed cost of 2 of reserva_de_teste", a)
		}
		if lo, hi, ok := a.SpendRange(4); !ok || lo != 2 || hi != 2 {
			t.Errorf("SpendRange(4) = %d, %d, %v, want 2, 2, true", lo, hi, ok)
		}
		if _, _, ok := a.SpendRange(1); ok {
			t.Error("with 1 use left, a cost of 2 is still on")
		}
		if n, err := a.Spend(0, 3); err != nil || n != 2 {
			t.Errorf("Spend(0, 3) = %d, %v, want 2", n, err)
		}
		for _, chosen := range []int{1, 3} {
			if _, err := a.Spend(chosen, 4); !errors.Is(err, ErrSpend) {
				t.Errorf("Spend(%d, 4) of a fixed cost: error = %v, want ErrSpend", chosen, err)
			}
		}
		if _, err := a.Spend(0, 1); !errors.Is(err, ErrSpend) {
			t.Errorf("Spend(0, 1) with too few left: error = %v, want ErrSpend", err)
		}
	})

	t.Run("1 to 3 points that heal 1d6 each", func(t *testing.T) {
		t.Parallel()
		a := grantedActionOf(t, d, breathFeat)
		if a.Cost != 1 || a.CostMax != 3 || !a.ChoosesSpend() || a.Result.Heal == nil {
			t.Fatalf("action = %+v, want cost 1 to 3 with a heal", a)
		}
		// The ability modifier (Wisdom 13) is in the bonus once: the base is 1d6+1.
		if got := *a.Result.Heal; got != (DiceFormula{Count: 1, Sides: 6, Bonus: 1}) {
			t.Errorf("heal = %+v, want 1d6+1", got)
		}
		for left, want := range map[int][2]int{3: {1, 3}, 2: {1, 2}, 1: {1, 1}, 5: {1, 3}} {
			if lo, hi, ok := a.SpendRange(left); !ok || lo != want[0] || hi != want[1] {
				t.Errorf("SpendRange(%d) = %d, %d, %v, want %v", left, lo, hi, ok, want)
			}
		}
		if _, _, ok := a.SpendRange(0); ok {
			t.Error("with none left, the action is still on")
		}
		for _, tc := range []struct{ chosen, left, want int }{{0, 5, 1}, {1, 5, 1}, {2, 5, 2}, {3, 5, 3}, {2, 2, 2}} {
			if n, err := a.Spend(tc.chosen, tc.left); err != nil || n != tc.want {
				t.Errorf("Spend(%d, %d) = %d, %v, want %d", tc.chosen, tc.left, n, err, tc.want)
			}
		}
		for _, tc := range []struct{ chosen, left int }{{4, 5}, {3, 2}, {-1, 5}, {1, 0}, {0, 0}} {
			if _, err := a.Spend(tc.chosen, tc.left); !errors.Is(err, ErrSpend) {
				t.Errorf("Spend(%d, %d): error = %v, want ErrSpend", tc.chosen, tc.left, err)
			}
		}
		// The result is the base plus the per-point dice for each point above the cost.
		for spent, want := range map[int]DiceFormula{1: {Count: 1, Sides: 6, Bonus: 1}, 2: {Count: 2, Sides: 6, Bonus: 1}, 3: {Count: 3, Sides: 6, Bonus: 1}} {
			if got := a.Outcome(spent); got.Heal == nil || *got.Heal != want || got.TempHP != 0 {
				t.Errorf("Outcome(%d) = %+v, want heal %+v", spent, got, want)
			}
		}
	})

	t.Run("temporary hit points equal to the level", func(t *testing.T) {
		t.Parallel()
		a := grantedActionOf(t, d, vigorFeat)
		if a.Result.TempHP != 3 || a.Result.Heal != nil || a.Resource != "" || a.UnitCost() != 0 {
			t.Fatalf("action = %+v, want 3 temporary hit points and no resource", a)
		}
		if lo, hi, ok := a.SpendRange(0); !ok || lo != 0 || hi != 0 {
			t.Errorf("SpendRange(0) = %d, %d, %v, want 0, 0, true", lo, hi, ok)
		}
		if n, err := a.Spend(0, 0); err != nil || n != 0 {
			t.Errorf("Spend(0, 0) = %d, %v, want 0", n, err)
		}
		if got := a.Outcome(0); got.TempHP != 3 || got.Heal != nil {
			t.Errorf("Outcome(0) = %+v, want 3 temporary hit points", got)
		}
	})
}

// TestFlatPointsScaleTemporaryHitPoints: a flat value_per_point adds to the temporary
// hit points, once for each point above the cost.
func TestFlatPointsScaleTemporaryHitPoints(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, featWith(reserveFeat,
		resourceEffect("reserva_de_teste", "5", RechargeLongRest),
		Effect{Type: "grant_action", Economy: EconomyBonusAction, Resource: "reserva_de_teste", Cost: "1", CostMax: "3", TempHP: "2", ValuePerPoint: "level()"},
	))
	a := grantedActionOf(t, d, reserveFeat)
	for spent, want := range map[int]int{1: 2, 2: 5, 3: 8} {
		if got := a.Outcome(spent); got.TempHP != want || got.Heal != nil {
			t.Errorf("Outcome(%d) = %+v, want %d temporary hit points", spent, got, want)
		}
	}
}

// TestActionWithoutItsResourceIsNotGranted: an action that spends a resource that is not
// at the level is not on the sheet, as before.
func TestActionWithoutItsResourceIsNotGranted(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, featWith(reserveFeat,
		resourceEffect("reserva_de_teste", "level() - 3", RechargeLongRest),
		Effect{Type: "grant_action", Economy: EconomyBonusAction, Resource: "reserva_de_teste", Cost: "1"},
	))
	if slices.ContainsFunc(d.Actions, func(a Action) bool { return a.Source == reserveFeat }) {
		t.Errorf("the action is there with no use of the resource: %+v", d.Actions)
	}
}

// TestRechargeVariants: a pool that returns one use when its owner rolls initiative,
// and the rests that return one use.
func TestRechargeVariants(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, featWith(initiativeFeat, resourceEffect("reserva_de_iniciativa", "2", RechargeInitiative)))
	i := slices.IndexFunc(d.Resources, func(r Resource) bool { return r.Key == "reserva_de_iniciativa" })
	if i < 0 || d.Resources[i].Recharge != RechargeInitiative || d.Resources[i].Max != 2 {
		t.Fatalf("resources = %+v, want reserva_de_iniciativa with 2 uses that recharge on initiative", d.Resources)
	}

	for _, tc := range []struct {
		recharge, kind string
		want           Recovery
		all            bool
	}{
		{RechargeShortRest, RestShort, RecoverAll, true},
		{RechargeShortRest, RestLong, RecoverAll, true},
		{RechargeLongRest, RestShort, RecoverNone, false},
		{RechargeLongRest, RestLong, RecoverAll, true},
		{RechargeRestoreOneShort, RestShort, RecoverOne, false},
		{RechargeRestoreOneShort, RestLong, RecoverAll, true},
		{RechargeRestoreOneLong, RestShort, RecoverNone, false},
		{RechargeRestoreOneLong, RestLong, RecoverOne, false},
		{RechargeInitiative, RestShort, RecoverNone, false},
		{RechargeInitiative, RestLong, RecoverNone, false},
		{RechargeDawn, RestLong, RecoverNone, false},
		{RechargeNone, RestLong, RecoverNone, false},
	} {
		if got := RestRecovery(tc.recharge, tc.kind); got != tc.want {
			t.Errorf("RestRecovery(%s, %s) = %v, want %v", tc.recharge, tc.kind, got, tc.want)
		}
		if got := RechargesOnRest(tc.recharge, tc.kind); got != tc.all {
			t.Errorf("RechargesOnRest(%s, %s) = %v, want %v", tc.recharge, tc.kind, got, tc.all)
		}
	}

	for _, tc := range []struct {
		recharge    string
		used, total int
		want        int
	}{
		{RechargeInitiative, 2, 2, 1}, // none left: one comes back
		{RechargeInitiative, 1, 2, 0}, // one left: nothing comes back
		{RechargeInitiative, 0, 2, 0},
		{RechargeInitiative, 3, 2, 1}, // more spent than the maximum (it went down): still one
		{RechargeInitiative, 0, 0, 0},
		{RechargeLongRest, 2, 2, 0},
		{RechargeRestoreOneShort, 2, 2, 0},
	} {
		if got := InitiativeRecovery(tc.recharge, tc.used, tc.total); got != tc.want {
			t.Errorf("InitiativeRecovery(%s, %d, %d) = %d, want %d", tc.recharge, tc.used, tc.total, got, tc.want)
		}
	}
}

// TestHooksOnTheSheet: the hooks of the character, by event, with the numbers worked
// out for its level and abilities.
func TestHooksOnTheSheet(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, featWith(hooksFeat,
		resourceEffect("reserva_de_teste", "1", RechargeLongRest),
		Effect{Type: "on_event", Event: EventLongRest, TempHP: "level()"},
		Effect{Type: "on_event", Event: EventShortRest, Restore: "reserva_de_teste", Value: "1"},
		Effect{Type: "on_event", Event: EventDeathSaveFailed, Resource: "reserva_de_teste", HealPercentMax: 50},
		Effect{Type: "on_event", Event: EventDropToZero, Heal: "1d4", Ability: "wis", Resource: "reserva_de_teste", Cost: "1"},
		Effect{Type: "on_event", Event: EventLongRest, When: "level() >= 10", TempHP: "10"},
	))
	got := d.HooksFor(EventLongRest)
	if len(got) != 1 || got[0].TempHP != 3 || got[0].Source != hooksFeat || got[0].Event != EventLongRest {
		t.Errorf("long rest hooks = %+v, want one with 3 temporary hit points (the one for level 10 is off)", got)
	}
	got = d.HooksFor(EventShortRest)
	if len(got) != 1 || got[0].Restore != "reserva_de_teste" || got[0].RestoreCount != 1 {
		t.Errorf("short rest hooks = %+v, want one that restores 1 use of reserva_de_teste", got)
	}
	got = d.HooksFor(EventDeathSaveFailed)
	if len(got) != 1 || got[0].HealPercentMax != 50 || got[0].Spends != "reserva_de_teste" || got[0].SpendCount != 1 {
		t.Errorf("death save hooks = %+v, want half the maximum for 1 use of reserva_de_teste", got)
	}
	got = d.HooksFor(EventDropToZero)
	if len(got) != 1 || got[0].Heal == nil || *got[0].Heal != (DiceFormula{Count: 1, Sides: 4, Bonus: 1}) || got[0].Spends != "reserva_de_teste" {
		t.Errorf("drop to zero hooks = %+v, want 1d4+1 for a use of reserva_de_teste", got)
	}
	if got := d.HooksFor("nunca"); len(got) != 0 {
		t.Errorf("an unknown event has hooks: %+v", got)
	}
	// Whoever has no feat has no hook.
	if plain := Derive(pensantus(), loadForTest(t)); len(plain.Hooks) != 0 {
		t.Errorf("a character with no table feat has hooks: %+v", plain.Hooks)
	}
}

// TestHookNumbers: a share of the maximum rounds down and is at least 1, and restoring
// never gives back more than is spent, so a hook that runs twice changes nothing.
func TestHookNumbers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ pct, maxHP, want int }{
		{50, 20, 10}, {50, 21, 10}, {50, 1, 1}, {1, 20, 1}, {100, 17, 17}, {25, 3, 1}, {50, 0, 1},
	} {
		if got := PercentOfMax(tc.pct, tc.maxHP); got != tc.want {
			t.Errorf("PercentOfMax(%d, %d) = %d, want %d", tc.pct, tc.maxHP, got, tc.want)
		}
	}
	for _, tc := range []struct{ used, count, want int }{
		{3, 1, 1}, {3, 5, 3}, {0, 2, 0}, {2, 0, 0}, {-1, 2, 0}, {1, -4, 0},
	} {
		if got := RestoreUses(tc.used, tc.count); got != tc.want {
			t.Errorf("RestoreUses(%d, %d) = %d, want %d", tc.used, tc.count, got, tc.want)
		}
	}
	// Run twice: the second time nothing is left to give back.
	used := 1
	used -= RestoreUses(used, 1)
	used -= RestoreUses(used, 1)
	if used != 0 {
		t.Errorf("a restore run twice left %d spent, want 0", used)
	}
}

// TestNewEffectFieldsBelongToTheirTypes: the SRD loader refuses them on other types
// and checks a costed action as the editor does.
func TestNewEffectFieldsBelongToTheirTypes(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for name, e := range map[string]*Effect{
		"cost on a modifier":         {Type: "modifier", Target: "ac", Mode: "add", Value: "1", Cost: "2"},
		"heal on a sense":            {Type: "sense", Sense: "darkvision", RangeFt: 60, Heal: "1d6"},
		"event on a grant_action":    {Type: "grant_action", Economy: EconomyAction, Event: EventLongRest},
		"restore on a resource":      {Type: "resource", Resource: "x", Max: "1", Recharge: RechargeNone, Restore: "x"},
		"an on_event with no event":  {Type: "on_event", TempHP: "1"},
		"an on_event with an event":  {Type: "on_event", Event: "nunca", TempHP: "1"},
		"an on_event with no result": {Type: "on_event", Event: EventLongRest},
		"cost_max below cost":        {Type: "grant_action", Economy: EconomyAction, Resource: "x", Cost: "3", CostMax: "2"},
		"a heal with no dice":        {Type: "grant_action", Economy: EconomyAction, Heal: "4"},
	} {
		if err := srd.c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	for name, e := range map[string]*Effect{
		"a costed action": {Type: "grant_action", Economy: EconomyAction, Resource: "x", Cost: "2", CostMax: "3", Heal: "1d6", ValuePerPoint: "1d6"},
		"a hook":          {Type: "on_event", Event: EventLongRest, TempHP: "level()"},
	} {
		if err := srd.c.compileEffect("feature:x", e); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Stored, not being written: a stray field is dropped and the effect still works.
	stored := featWith("feat:guardado"+tableSuffix, Effect{Type: "modifier", Target: "initiative", Mode: "add", Value: "1", Cost: "2", Heal: "1d6"})
	stored.Strict = nil
	if _, err := srd.With(stored); err != nil {
		t.Errorf("a stored effect with stray fields is refused: %v", err)
	}
}
