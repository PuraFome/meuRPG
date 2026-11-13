package rules

import (
	"fmt"
	"strings"
	"testing"
)

const useFeat = "feat:uso-da-mesa" + tableSuffix

// useAction is a grant_action that declares the use the test gives it.
func useAction(u *FeatureUse) Effect {
	return Effect{Type: "grant_action", Economy: EconomyBonusAction, Use: u}
}

func goodEffect() *TableSpellEffect {
	return &TableSpellEffect{Modifiers: []EffectModifier{{Kind: ModifierRollBonus, Value: 2, AppliesTo: []string{RollAppliesAttack}}}}
}

// TestFeatureUseShapes: a use is accepted in each shape and refused at the field that is wrong.
func TestFeatureUseShapes(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	with := func(mut func(u *FeatureUse)) []Effect {
		u := &FeatureUse{}
		mut(u)
		return []Effect{useAction(u)}
	}
	repeat := func(n int) []FeatureChoice {
		var out []FeatureChoice
		for i := range n {
			out = append(out, FeatureChoice{Key: fmt.Sprintf("opt_%d", i), TextPT: "Opção"})
		}
		return out
	}
	cases := []struct {
		name       string
		effects    []Effect
		field, why string // "" when accepted; the field under effects[0]
	}{
		// Accepted.
		{"a limit per turn", with(func(u *FeatureUse) { u.LimitScope, u.LimitUses = UseScopeTurn, "1" }), "", ""},
		{"a limit by level", with(func(u *FeatureUse) { u.LimitScope, u.LimitUses = UseScopeCombat, "max(1, prof())" }), "", ""},
		{"targets", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Min: 1, Max: 3, Who: FeatureWhoAlly, RangeFt: 30} }), "", ""},
		{"a self use", with(func(u *FeatureUse) {
			u.Targets = &FeatureTargets{Max: 1, Who: FeatureWhoSelf}
			u.Applies = &FeatureApplies{Heal: "1d6"}
		}), "", ""},
		{"choices", with(func(u *FeatureUse) { u.Choices = repeat(8) }), "", ""},
		{"a heal and temporary hit points", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Heal: "2d4+2", TempHP: "level()"} }), "", ""},
		{"an effect", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Effect: goodEffect(), EffectRounds: 10} }), "", ""},
		{"an effect that applies to all", with(func(u *FeatureUse) {
			e := goodEffect()
			e.Applies = "all"
			u.Applies = &FeatureApplies{Effect: e, EffectRounds: 14400}
		}), "", ""},
		// A stray use.
		{"a use on a note", []Effect{{Type: "note", TextPT: "Texto.", Use: &FeatureUse{LimitScope: UseScopeTurn, LimitUses: "1"}}}, ".use", ReasonValue},
		// Nothing declared.
		{"an empty use", with(func(*FeatureUse) {}), ".use", ReasonValue},
		{"an empty use with empty targets and applies", with(func(u *FeatureUse) { u.Targets, u.Applies = &FeatureTargets{}, &FeatureApplies{} }), ".use", ReasonValue},
		// The limit.
		{"a scope with no uses", with(func(u *FeatureUse) { u.LimitScope = UseScopeRound }), ".use.limit_uses", ReasonValue},
		{"uses with no scope", with(func(u *FeatureUse) { u.LimitUses = "2" }), ".use.limit_scope", ReasonValue},
		{"an unknown scope", with(func(u *FeatureUse) { u.LimitScope, u.LimitUses = "unknown", "1" }), ".use.limit_scope", ReasonValue},
		{"uses that do not compile", with(func(u *FeatureUse) { u.LimitScope, u.LimitUses = UseScopeTurn, "level(" }), ".use.limit_uses", ReasonFormula},
		{"a literal of 0", with(func(u *FeatureUse) { u.LimitScope, u.LimitUses = UseScopeTurn, "0" }), ".use.limit_uses", ReasonValue},
		// The targets.
		{"a min below 0", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Min: -1, Max: 1, Who: FeatureWhoAny} }), ".use.targets.min", ReasonValue},
		{"a min above 20", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Min: 21, Max: 21, Who: FeatureWhoAny} }), ".use.targets.min", ReasonValue},
		{"a max above 20", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 21, Who: FeatureWhoAny} }), ".use.targets.max", ReasonValue},
		{"a max below min", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Min: 3, Max: 2, Who: FeatureWhoAny} }), ".use.targets.max", ReasonValue},
		{"targets with no who", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 1} }), ".use.targets.who", ReasonValue},
		{"an unknown who", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 1, Who: "unknown"} }), ".use.targets.who", ReasonValue},
		{"a range above 300", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 1, Who: FeatureWhoEnemy, RangeFt: 301} }), ".use.targets.range_ft", ReasonValue},
		{"self with two targets", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 2, Who: FeatureWhoSelf} }), ".use.targets.max", ReasonValue},
		{"self with a range", with(func(u *FeatureUse) { u.Targets = &FeatureTargets{Max: 1, Who: FeatureWhoSelf, RangeFt: 30} }), ".use.targets.range_ft", ReasonValue},
		{"a range with no targets", with(func(u *FeatureUse) {
			u.Targets = &FeatureTargets{RangeFt: 30}
			u.LimitScope, u.LimitUses = UseScopeTurn, "1"
		}), ".use.targets.range_ft", ReasonValue},
		// The choices.
		{"nine choices", with(func(u *FeatureUse) { u.Choices = repeat(9) }), ".use.choices", ReasonLimit},
		{"an empty key", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: "", TextPT: "A"}} }), ".use.choices[0].key", ReasonValue},
		{"an upper case key", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: "a", TextPT: "A"}, {Key: "Bad", TextPT: "B"}} }), ".use.choices[1].key", ReasonValue},
		{"a long key", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: strings.Repeat("a", 41), TextPT: "A"}} }), ".use.choices[0].key", ReasonValue},
		{"a repeated key", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: "a", TextPT: "A"}, {Key: "a", TextPT: "B"}} }), ".use.choices[1].key", ReasonValue},
		{"an empty text", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: "a"}} }), ".use.choices[0].text_pt", ReasonValue},
		{"a long text", with(func(u *FeatureUse) { u.Choices = []FeatureChoice{{Key: "a", TextPT: strings.Repeat("ã", 81)}} }), ".use.choices[0].text_pt", ReasonValue},
		// What it applies.
		{"a heal with no dice", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Heal: "5"} }), ".use.applies.heal", ReasonValue},
		{"a heal with MOD", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Heal: "1d6 + MOD"} }), ".use.applies.heal", ReasonValue},
		{"temp_hp that does not compile", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{TempHP: "level("} }), ".use.applies.temp_hp", ReasonFormula},
		{"an effect that applies on a failed save", with(func(u *FeatureUse) {
			e := goodEffect()
			e.Applies = "failed_save"
			u.Applies = &FeatureApplies{Effect: e, EffectRounds: 10}
		}), ".use.applies.effect.applies", ReasonValue},
		{"an effect with nothing in it", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Effect: &TableSpellEffect{}, EffectRounds: 10} }), ".use.applies.effect", ReasonValue},
		{"a bad modifier", with(func(u *FeatureUse) {
			u.Applies = &FeatureApplies{Effect: &TableSpellEffect{Modifiers: []EffectModifier{{Kind: ModifierNoReaction}}}, EffectRounds: 10}
		}), ".use.applies.effect.modifiers[0].kind", ReasonValue},
		{"an effect with no rounds", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Effect: goodEffect()} }), ".use.applies.effect_rounds", ReasonValue},
		{"an effect of too many rounds", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Effect: goodEffect(), EffectRounds: 14401} }), ".use.applies.effect_rounds", ReasonValue},
		{"rounds with no effect", with(func(u *FeatureUse) { u.Applies = &FeatureApplies{Heal: "1d4", EffectRounds: 5} }), ".use.applies.effect_rounds", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := srd.With(featWith(useFeat, tc.effects...))
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			violationAt(t, err, "feats[0].effects[0]"+tc.field, tc.why)
		})
	}
}

func TestAFeatureUseDerivesForTheCharacter(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := featWith(useFeat, useAction(&FeatureUse{
		LimitScope: UseScopeRound, LimitUses: "floor(level() / 3)",
		Targets: &FeatureTargets{Min: 1, Max: 2, Who: FeatureWhoAlly, RangeFt: 30},
		Choices: []FeatureChoice{{Key: "fogo", TextPT: "Fogo"}, {Key: "gelo", TextPT: "Gelo"}},
		Applies: &FeatureApplies{Heal: "1d6", TempHP: "level() * 2", Effect: goodEffect(), EffectRounds: 10},
	}))
	c, err := srd.With(o)
	if err != nil {
		t.Fatal(err)
	}
	derive := func(level int) Action {
		b := pensantus()
		b.Classes[0].Level = level
		b.Feats = append(b.Feats, useFeat)
		return grantedActionOf(t, Derive(b, c), useFeat)
	}
	for _, tc := range []struct{ level, uses, temp int }{{3, 1, 6}, {9, 3, 18}} {
		a := derive(tc.level)
		u := a.Use
		if !a.ViaUseFeature() || u == nil {
			t.Fatalf("level %d: Use = nil, want the declared use", tc.level)
		}
		if u.Scope != UseScopeRound || u.Uses != tc.uses || u.Result.TempHP != tc.temp || u.Result.Heal == nil || *u.Result.Heal != (DiceFormula{Count: 1, Sides: 6}) {
			t.Errorf("level %d: use = %+v, want round x%d, heal 1d6, %d temp", tc.level, u, tc.uses, tc.temp)
		}
		if u.Targets != (FeatureTargets{Min: 1, Max: 2, Who: FeatureWhoAlly, RangeFt: 30}) || len(u.Choices) != 2 || u.Choices[1].Key != "gelo" {
			t.Errorf("level %d: targets/choices = %+v %+v", tc.level, u.Targets, u.Choices)
		}
		if !u.HasEffect || u.EffectRounds != 10 {
			t.Errorf("level %d: effect = %v for %d rounds", tc.level, u.HasEffect, u.EffectRounds)
		}
		if got := u.Left(1); got != max(tc.uses-1, 0) {
			t.Errorf("level %d: Left(1) = %d", tc.level, got)
		}
	}
	if d, ok := c.FeatureUseEffect(useFeat); !ok || d.Kind != "feature" || len(d.Modifiers) != 1 || d.Applies != "all" {
		t.Errorf("FeatureUseEffect = %+v, %v, want the feature's effect that applies to all", d, ok)
	}
}

func TestAnActionWithoutAUseStaysOnTakeAction(t *testing.T) {
	t.Parallel()
	d := deriveWithOverlay(t, featWith(useFeat, Effect{Type: "grant_action", Economy: EconomyBonusAction, TempHP: "4"}))
	a := grantedActionOf(t, d, useFeat)
	if a.ViaUseFeature() || a.Use != nil || a.Result.TempHP != 4 {
		t.Errorf("action = %+v, want no use and the old result", a)
	}
	var none *ActionUse
	if none.Limited() || none.Left(3) != -1 {
		t.Error("a nil use has no limit")
	}
}
