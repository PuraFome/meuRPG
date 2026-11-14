package rules

import (
	"slices"
	"testing"
)

// The aura effect (#579, slice 3): a feature's aura, as a table spell's, carried by the holder into every combat. The
// feature is invented (it is the shape of Aura of Protection).

func auraEffect(radius int, mods ...EffectModifier) Effect {
	return Effect{Type: "aura", Aura: &TableSpellEffect{
		Aura: &SpellAura{RadiusFt: radius, Affects: AuraAffectsAllies}, Modifiers: mods,
		Visibility: EffectVisibilityPublic, PlayerLabelPT: "Aura de proteção",
	}}
}

var chaSaveBonus = EffectModifier{Kind: ModifierRollBonus, ValueAbility: "cha", ValueMin: 1, AppliesTo: []string{RollAppliesSave}}

func TestAnAuraEffectIsCheckedAtItsOwnFields(t *testing.T) {
	t.Parallel()
	bad := func(f func(e *Effect)) Effect { e := auraEffect(10, chaSaveBonus); f(&e); return e }
	for name, tc := range map[string]struct {
		effect Effect
		field  string
	}{
		"no aura":               {Effect{Type: "aura"}, ".aura"},
		"no radius and side":    {bad(func(e *Effect) { e.Aura.Aura = nil }), ".aura.aura"},
		"a radius off the step": {bad(func(e *Effect) { e.Aura.Aura.RadiusFt = 12 }), ".aura.aura.radius_ft"},
		"a condition":           {bad(func(e *Effect) { e.Aura.Conditions = []string{"condition:poisoned"} }), ".aura.conditions"},
		"a save":                {bad(func(e *Effect) { e.Aura.Applies = "failed_save" }), ".aura.applies"},
		"no modifier":           {bad(func(e *Effect) { e.Aura.Modifiers = nil }), ".aura"},
		"an ac bonus":           {bad(func(e *Effect) { e.Aura.Modifiers = []EffectModifier{{Kind: ModifierACBonus, Value: 1}} }), ".aura.modifiers[0].kind"},
		"a next-hit effect": {bad(func(e *Effect) {
			e.Aura.Modifiers = []EffectModifier{{Kind: ModifierWeaponHitDamage, Trigger: HitTriggerNextHit, Die: 6, Dice: 1, MaxDice: 1}}
		}), ".aura.modifiers[0].kind"},
		"an ability, no least": {bad(func(e *Effect) {
			e.Aura.Modifiers = []EffectModifier{{Kind: ModifierRollBonus, ValueAbility: "cha", AppliesTo: []string{RollAppliesSave}}}
		}), ".aura.modifiers[0].value_min"},
		"an unknown ability": {bad(func(e *Effect) {
			e.Aura.Modifiers = []EffectModifier{{Kind: ModifierRollBonus, ValueAbility: "luck", ValueMin: 1, AppliesTo: []string{RollAppliesSave}}}
		}), ".aura.modifiers[0].value_ability"},
		"a value with it":   {bad(func(e *Effect) { e.Aura.Modifiers[0].Value = 3 }), ".aura.modifiers[0].value"},
		"tags":              {bad(func(e *Effect) { e.Tags = []string{"source:fiction"} }), ".tags"},
		"a flag with it":    {bad(func(e *Effect) { e.Flag = FlagNeverSurprised }), ".flag"},
		"an aura on a note": {Effect{Type: "note", Aura: auraEffect(10, chaSaveBonus).Aura}, ".aura"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(featWith(flagFeat, tc.effect))
			if err == nil {
				t.Fatalf("With() accepted %+v", tc.effect)
			}
			violationAt(t, err, "feats[0].effects[0]"+tc.field, ReasonValue)
		})
	}
}

func TestOnlyAFeaturesAuraTakesItsBonusFromAnAbility(t *testing.T) {
	t.Parallel()
	if _, err := loadForTest(t).With(featWith(flagFeat, auraEffect(10, chaSaveBonus))); err != nil {
		t.Fatalf("the aura of a feature is refused: %v", err)
	}
	if err := checkModifier(EffectModifier{Kind: ModifierRollBonus, Value: 1, ValueAbility: "cha", ValueMin: 1, AppliesTo: []string{RollAppliesSave}}); err == nil {
		t.Error("a value and an ability together are accepted")
	}
	if err := checkModifier(EffectModifier{Kind: ModifierACBonus, Value: 1, ValueMin: 1}); err == nil {
		t.Error("a value_min on an ac_bonus is accepted")
	}
}

func TestDeriveCarriesTheFirstAuraOfAFeatureWhoseWhenHolds(t *testing.T) {
	t.Parallel()
	small, wide := auraEffect(10, chaSaveBonus), auraEffect(30, chaSaveBonus)
	small.When, wide.When = "level() < 3", "level() >= 3"
	c := withOverlay(t, rollOverlay([]Effect{small, wide}, nil))
	for level, want := range map[int]int{2: 0, 3: 30} {
		// The subclass starts at level 3: the feature is the character's from there.
		auras := Derive(rollBuild(level), c).Auras
		if level == 2 {
			if len(auras) != 0 {
				t.Errorf("level %d: auras = %+v, want none", level, auras)
			}
			continue
		}
		if len(auras) != 1 || auras[0].Source != rollFirst {
			t.Fatalf("level %d: auras = %+v, want one of %s", level, auras, rollFirst)
		}
		mods := auras[0].Def.Modifiers
		if len(mods) != 2 || mods[0].Kind != ModifierAura || mods[0].RadiusFt != want || !slices.Equal(mods[1].AppliesTo, []string{RollAppliesSave}) {
			t.Errorf("level %d: modifiers = %+v, want an aura of %d ft that gives a bonus on saves", level, mods, want)
		}
		if mods[1].ValueAbility != "cha" || mods[1].ValueMin != 1 {
			t.Errorf("level %d: the bonus = %+v, want it from the charisma modifier, at least 1", level, mods[1])
		}
	}
}
