package rules

import (
	"errors"
	"slices"
	"testing"
)

// The extra_damage effect: damage added to a weapon hit once per turn, when a fact about the target
// holds, optionally spending a use of a resource. The names are invented; the engine is generic.

const (
	extraSubclass = "subclass:cacador-de-gigantes" + tableSuffix
	extraFeature  = "feature:golpe-de-gigante" + tableSuffix
	extraUses     = "golpes_de_gigante"
)

func extraDamageOverlay(effects ...Effect) Overlay {
	if effects == nil {
		effects = []Effect{
			{Type: "resource", Resource: extraUses, Max: "2", Recharge: "short_rest"},
			{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "prof() + 1", Resource: extraUses, TextPT: "Só contra alvos maiores."},
		}
	}
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: extraSubclass, NamePT: "Caçador de Gigantes", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{{
			Key: extraFeature, NamePT: "Golpe de Gigante", DescPT: []string{"Dano extra contra alvos maiores."}, Effects: effects,
		}}}},
	}}}
}

func extraBuild(level int) Build {
	b := postureBuild(level)
	b.Classes[0].Subclass = extraSubclass
	return b
}

func TestDeriveExposesTheExtraDamageWithItsAmount(t *testing.T) {
	c := withOverlay(t, extraDamageOverlay())
	d := Derive(extraBuild(5), c)
	if len(d.ExtraDamages) != 1 {
		t.Fatalf("extra damages = %+v, want one", d.ExtraDamages)
	}
	x := d.ExtraDamages[0]
	if x.Key != extraFeature || x.NamePT != "Golpe de Gigante" || x.Applies != ExtraDamageLargerTarget || x.Resource != extraUses || x.TextPT != "Só contra alvos maiores." {
		t.Errorf("extra damage = %+v", x)
	}
	if want := d.ProficiencyBonus + 1; x.Amount != want {
		t.Errorf("amount = %d, want the proficiency bonus plus 1 (%d)", x.Amount, want)
	}
	if got := Derive(extraBuild(2), c).ExtraDamages; len(got) != 0 {
		t.Errorf("extra damages at level 2 = %+v, want none before the feature", got)
	}
}

func TestExtraDamageWithoutAResourceIsOncePerTurnAlone(t *testing.T) {
	c := withOverlay(t, extraDamageOverlay(Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2"}))
	d := Derive(extraBuild(3), c)
	if len(d.ExtraDamages) != 1 || d.ExtraDamages[0].Resource != "" || d.ExtraDamages[0].Amount != 2 || d.ExtraDamages[0].Applies != ExtraDamageAnyTarget {
		t.Errorf("extra damages = %+v, want a flat 2 against any target and no resource", d.ExtraDamages)
	}
}

func TestExtraDamageThatGivesNothingIsIgnoredWithAnIssue(t *testing.T) {
	c := withOverlay(t, extraDamageOverlay(Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "0"}))
	d := Derive(extraBuild(3), c)
	if len(d.ExtraDamages) != 0 {
		t.Errorf("extra damages = %+v, want none for an amount of 0", d.ExtraDamages)
	}
	if !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueFormula }) {
		t.Errorf("issues = %+v, want a formula issue", d.Issues)
	}
}

func TestExtraDamageRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"no applies", Effect{Type: "extra_damage", Value: "2"}, at + ".applies", ReasonValue},
		{"an unknown applies", Effect{Type: "extra_damage", Applies: "smaller_target", Value: "2"}, at + ".applies", ReasonValue},
		{"no value", Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget}, at + ".value", ReasonValue},
		{"a value that does not compile", Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "prof("}, at + ".value", ReasonFormula},
		{"a resource nobody defines", Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "2", Resource: "fantasma"}, at + ".resource", ReasonReference},
		{"an economy", Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "2", Economy: "bonus_action"}, at + ".economy", ReasonValue},
		{"an ability", Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "2", Ability: "str"}, at + ".ability", ReasonValue},
		{"dice without sides", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "1"}, at + ".dice_sides", ReasonValue},
		{"sides without dice", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "1", DiceSides: 6}, at + ".dice", ReasonValue},
		{"a die that is not on the menu", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "1", DiceSides: 7}, at + ".dice_sides", ReasonValue},
		{"a dice formula that does not compile", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "prof(", DiceSides: 6}, at + ".dice", ReasonFormula},
		{"a damage type that does not exist", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "1", DiceSides: 6, DamageType: "damage-type:sonho"}, at + ".damage_type", ReasonReference},
		{"a key that is no damage type", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "1", DiceSides: 6, DamageType: "skill:stealth"}, at + ".damage_type", ReasonReference},
		{"an unknown weapon", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", Weapon: "spell"}, at + ".weapon", ReasonValue},
		{"an unknown frequency", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", Frequency: "twice"}, at + ".frequency", ReasonValue},
		{"no crit doubling without dice", Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", NoCritDoubling: true}, at + ".no_crit_doubling", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := extraDamageOverlay(tc.effect)
			o.Strict = []string{extraSubclass} // the stray fields are refused for the entry being written
			_, err := loadForTest(t).With(o)
			if err == nil {
				t.Fatal("the effect is accepted")
			}
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s reason %s", err, tc.field, tc.reason)
			}
		})
	}
}

func TestAFeatureHasOneExtraDamage(t *testing.T) {
	t.Parallel()
	two := extraDamageOverlay(
		Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "1"},
		Effect{Type: "extra_damage", Applies: ExtraDamageLargerTarget, Value: "2"},
	)
	_, err := loadForTest(t).With(two)
	var oe *OverlayError
	if !errors.As(err, &oe) || oe.Field != "subclasses[0].levels[0].features[0].effects[1].type" || oe.Reason != ReasonEffect {
		t.Errorf("error = %v, want the second extra_damage refused at its type", err)
	}
}

func TestExtraDamageIsOnTheMenu(t *testing.T) {
	menu := withOverlay(t, extraDamageOverlay()).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(ty MenuType) bool { return ty.Type == "extra_damage" })
	if i < 0 {
		t.Fatal("no extra_damage on the menu")
	}
	ty := menu.Types[i]
	if ty.NamePT != "Dano extra" {
		t.Errorf("name = %q", ty.NamePT)
	}
	for _, name := range []string{"applies", "value", "dice", "dice_sides", "damage_type", "weapon", "frequency", "no_crit_doubling", "resource", "when", "text_pt"} {
		if !slices.ContainsFunc(ty.Fields, func(f MenuField) bool { return f.Name == name }) {
			t.Errorf("the menu type has no %s field", name)
		}
	}
	j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == ListExtraDamageApplies })
	if j < 0 || len(menu.Lists[j].Values) != 2 {
		t.Errorf("the applies list = %+v, want two values", menu.Lists)
	}
	for list, want := range map[string]int{ListExtraDamageWeapons: 3, ListExtraDamageFrequencies: 2} {
		k := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == list })
		if k < 0 || len(menu.Lists[k].Values) != want {
			t.Errorf("the %s list = %+v, want %d values", list, menu.Lists, want)
		}
	}
}

// Dice beside the fixed damage: the count is a formula worked out for the character at Derive.
func TestDeriveExposesTheDiceOfAnExtraDamage(t *testing.T) {
	c := withOverlay(t, extraDamageOverlay(Effect{
		Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "prof()", DiceSides: 6, DamageType: "damage-type:fire",
		Weapon: ExtraDamageRangedWeapon, Frequency: ExtraDamageEachHit, NoCritDoubling: true,
	}))
	d := Derive(extraBuild(5), c)
	if len(d.ExtraDamages) != 1 {
		t.Fatalf("extra damages = %+v, want one", d.ExtraDamages)
	}
	x := d.ExtraDamages[0]
	if x.Dice != (DiceFormula{Count: d.ProficiencyBonus, Sides: 6}) || x.Amount != 0 || x.DamageType != "damage-type:fire" ||
		x.Weapon != ExtraDamageRangedWeapon || x.Frequency != ExtraDamageEachHit || !x.NoCritDoubling {
		t.Errorf("extra damage = %+v, want %dd6 fire, no flat, ranged, each hit, no doubling", x, d.ProficiencyBonus)
	}
}

func TestExtraDamageDefaultsToAnyWeaponOncePerTurnAndDoublingDice(t *testing.T) {
	c := withOverlay(t, extraDamageOverlay(Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: "1", DiceSides: 8, Value: "2"}))
	x := Derive(extraBuild(3), c).ExtraDamages[0]
	if x.Weapon != ExtraDamageAnyWeapon || x.Frequency != ExtraDamageOncePerTurn || x.NoCritDoubling || x.DamageType != "" ||
		x.Dice != (DiceFormula{Count: 1, Sides: 8}) || x.Amount != 2 {
		t.Errorf("extra damage = %+v, want 1d8 + 2 of the weapon's type, any weapon, once per turn, doubled on a critical", x)
	}
}

func TestAnExtraDamageDiceCountOutOfRangeIsIgnoredWithAnIssue(t *testing.T) {
	for _, dice := range []string{"0", "21", "mod(\"str\") - 5"} {
		c := withOverlay(t, extraDamageOverlay(Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Dice: dice, DiceSides: 6}))
		d := Derive(extraBuild(3), c)
		if len(d.ExtraDamages) != 0 || !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueFormula }) {
			t.Errorf("dice %q: extra damages = %+v, issues = %+v, want it ignored with a formula issue", dice, d.ExtraDamages, d.Issues)
		}
	}
}

// The dice fields belong to extra_damage: another type that carries one is refused for the entry being written.
func TestDiceFieldsOnAnotherEffectTypeAreRefused(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	for field, e := range map[string]Effect{
		"dice":             {Type: "note", Dice: "1"},
		"dice_sides":       {Type: "note", DiceSides: 6},
		"damage_type":      {Type: "note", DamageType: "damage-type:fire"},
		"weapon":           {Type: "note", Weapon: "melee"},
		"frequency":        {Type: "note", Frequency: "each_hit"},
		"no_crit_doubling": {Type: "note", NoCritDoubling: true},
	} {
		o := extraDamageOverlay(e)
		o.Strict = []string{extraSubclass}
		_, err := loadForTest(t).With(o)
		var oe *OverlayError
		if !errors.As(err, &oe) || oe.Field != at+"."+field || oe.Reason != ReasonValue {
			t.Errorf("%s: error = %v, want it refused at the field", field, err)
		}
	}
}
