package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The power_attack effect: a trade the player may make on an attack, a penalty to the attack roll for a
// flat bonus to the damage of a hit, for the weapons its filters choose. The names are invented; the
// engine is generic.

const (
	powerSubclass = "subclass:tiro-pesado" + tableSuffix
	powerFeature  = "feature:troca-pesada" + tableSuffix
	powerUses     = "trocas_pesadas"
)

func powerAttackOverlay(effects ...Effect) Overlay {
	if effects == nil {
		effects = []Effect{{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10}}
	}
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: powerSubclass, NamePT: "Tiro Pesado", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{{
			Key: powerFeature, NamePT: "Troca Pesada", DescPT: []string{"Troca precisão por dano."}, Effects: effects,
		}}}},
	}}}
}

func powerBuild(level int, weapons ...string) Build {
	b := postureBuild(level)
	b.Classes[0].Subclass = powerSubclass
	b.Weapons = weapons
	return b
}

func TestPowerAttackRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	pa := func(edit func(e *Effect)) Effect {
		e := Effect{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10}
		edit(&e)
		return e
	}
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"no penalty", pa(func(e *Effect) { e.AttackPenalty = 0 }), at + ".attack_penalty", ReasonValue},
		{"a negative penalty", pa(func(e *Effect) { e.AttackPenalty = -5 }), at + ".attack_penalty", ReasonValue},
		{"a penalty above 10", pa(func(e *Effect) { e.AttackPenalty = 11 }), at + ".attack_penalty", ReasonValue},
		{"no bonus", pa(func(e *Effect) { e.DamageBonus = 0 }), at + ".damage_bonus", ReasonValue},
		{"a negative bonus", pa(func(e *Effect) { e.DamageBonus = -1 }), at + ".damage_bonus", ReasonValue},
		{"a bonus above 20", pa(func(e *Effect) { e.DamageBonus = 21 }), at + ".damage_bonus", ReasonValue},
		{"an unknown weapon", pa(func(e *Effect) { e.Weapon = "spell" }), at + ".weapon", ReasonValue},
		{"a property that does not exist", pa(func(e *Effect) { e.WeaponProperties = []string{"weapon-property:fantasma"} }), at + ".weapon_properties", ReasonReference},
		{"a key that is no weapon property", pa(func(e *Effect) { e.WeaponProperties = []string{"skill:stealth"} }), at + ".weapon_properties", ReasonReference},
		{"an empty property key", pa(func(e *Effect) { e.WeaponProperties = []string{""} }), at + ".weapon_properties", ReasonReference},
		{"a property twice", pa(func(e *Effect) {
			e.WeaponProperties = []string{"weapon-property:heavy", "weapon-property:heavy"}
		}), at + ".weapon_properties", ReasonValue},
		{"five properties", pa(func(e *Effect) {
			e.WeaponProperties = []string{"weapon-property:ammunition", "weapon-property:finesse", "weapon-property:heavy", "weapon-property:light", "weapon-property:loading"}
		}), at + ".weapon_properties", ReasonValue},
		{"a resource nobody defines", pa(func(e *Effect) { e.Resource = "fantasma" }), at + ".resource", ReasonReference},
		{"a when that does not compile", pa(func(e *Effect) { e.When = "level(" }), at + ".when", ReasonFormula},
		{"a stray value", pa(func(e *Effect) { e.Value = "2" }), at + ".value", ReasonValue},
		{"a stray applies", pa(func(e *Effect) { e.Applies = ExtraDamageAnyTarget }), at + ".applies", ReasonValue},
		{"a stray economy", pa(func(e *Effect) { e.Economy = "bonus_action" }), at + ".economy", ReasonValue},
		{"a stray frequency", pa(func(e *Effect) { e.Frequency = ExtraDamageEachHit }), at + ".frequency", ReasonValue},
		{"a stray attack_bonus", pa(func(e *Effect) { e.AttackBonus = 1 }), at + ".attack_bonus", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := powerAttackOverlay(tc.effect)
			o.Strict = []string{powerSubclass} // the stray fields are refused for the entry being written
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

func TestPowerAttackWithEveryFieldIsAccepted(t *testing.T) {
	t.Parallel()
	o := powerAttackOverlay(
		Effect{Type: "resource", Resource: powerUses, Max: "2", Recharge: "short_rest"},
		Effect{
			Type: "power_attack", AttackPenalty: 10, DamageBonus: 20, Weapon: ExtraDamageMeleeWeapon, OnlyProficient: true,
			WeaponProperties: []string{"weapon-property:heavy", "weapon-property:two-handed", "weapon-property:reach", "weapon-property:versatile"},
			Resource:         powerUses, When: "level() >= 3", TextPT: "Só com armas pesadas.",
		},
	)
	o.Strict = []string{powerSubclass}
	if _, err := loadForTest(t).With(o); err != nil {
		t.Fatalf("With: %v", err)
	}
	// Exactly four properties are the most it takes.
	four := powerAttackOverlay(Effect{Type: "power_attack", AttackPenalty: 1, DamageBonus: 1,
		WeaponProperties: []string{"weapon-property:ammunition", "weapon-property:finesse", "weapon-property:heavy", "weapon-property:light"}})
	if _, err := loadForTest(t).With(four); err != nil {
		t.Errorf("four properties: %v", err)
	}
	for _, tc := range []struct {
		name           string
		penalty, bonus int
	}{{"the lowest", 1, 1}, {"the highest", 10, 20}} {
		o := powerAttackOverlay(Effect{Type: "power_attack", AttackPenalty: tc.penalty, DamageBonus: tc.bonus})
		if _, err := loadForTest(t).With(o); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestAFeatureHasOnePowerAttack(t *testing.T) {
	t.Parallel()
	two := powerAttackOverlay(
		Effect{Type: "power_attack", AttackPenalty: 2, DamageBonus: 4},
		Effect{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10},
	)
	_, err := loadForTest(t).With(two)
	var oe *OverlayError
	if !errors.As(err, &oe) || oe.Field != "subclasses[0].levels[0].features[0].effects[1].type" || oe.Reason != ReasonEffect {
		t.Errorf("error = %v, want the second power_attack refused at its type", err)
	}
}

// The fields of power_attack belong to it: another type that carries one is refused for the entry being written.
func TestPowerAttackFieldsOnAnotherEffectTypeAreRefused(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	for field, e := range map[string]Effect{
		"attack_penalty":    {Type: "note", AttackPenalty: 5},
		"weapon_properties": {Type: "note", WeaponProperties: []string{"weapon-property:heavy"}},
		"only_proficient":   {Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", OnlyProficient: true},
		"damage_bonus":      {Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", DamageBonus: 2},
	} {
		o := powerAttackOverlay(e)
		o.Strict = []string{powerSubclass}
		_, err := loadForTest(t).With(o)
		var oe *OverlayError
		if !errors.As(err, &oe) || oe.Field != at+"."+field || oe.Reason != ReasonValue {
			t.Errorf("%s: error = %v, want it refused at the field", field, err)
		}
	}
}

func TestDeriveExposesThePowerAttacks(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay(
		Effect{Type: "resource", Resource: powerUses, Max: "2", Recharge: "short_rest"},
		Effect{
			Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, Weapon: ExtraDamageMeleeWeapon, OnlyProficient: true,
			WeaponProperties: []string{"weapon-property:heavy", "weapon-property:two-handed"}, Resource: powerUses, TextPT: "Só com armas pesadas.",
		},
	))
	d := Derive(powerBuild(5), c)
	if len(d.PowerAttacks) != 1 {
		t.Fatalf("power attacks = %+v, want one", d.PowerAttacks)
	}
	p := d.PowerAttacks[0]
	if p.Key != powerFeature || p.NamePT != "Troca Pesada" || p.AttackPenalty != 5 || p.DamageBonus != 10 || p.Weapon != ExtraDamageMeleeWeapon ||
		!p.OnlyProficient || p.Resource != powerUses || p.TextPT != "Só com armas pesadas." ||
		!slices.Equal(p.Properties, []string{"weapon-property:heavy", "weapon-property:two-handed"}) {
		t.Errorf("power attack = %+v", p)
	}
	if got := Derive(powerBuild(2), c).PowerAttacks; len(got) != 0 {
		t.Errorf("power attacks at level 2 = %+v, want none before the feature", got)
	}
}

func TestPowerAttackDefaultsToAnyWeaponWithNoFilter(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay())
	d := Derive(powerBuild(3), c)
	if len(d.PowerAttacks) != 1 {
		t.Fatalf("power attacks = %+v, want one", d.PowerAttacks)
	}
	p := d.PowerAttacks[0]
	if p.Weapon != ExtraDamageAnyWeapon || len(p.Properties) != 0 || p.OnlyProficient || p.Resource != "" {
		t.Errorf("power attack = %+v, want any weapon, no property, no proficiency, no resource", p)
	}
}

func TestPowerAttackWhenGatesTheEffect(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay(Effect{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, When: "level() >= 5"}))
	if got := Derive(powerBuild(4), c).PowerAttacks; len(got) != 0 {
		t.Errorf("power attacks at level 4 = %+v, want none while the when is false", got)
	}
	if got := Derive(powerBuild(5), c).PowerAttacks; len(got) != 1 {
		t.Errorf("power attacks at level 5 = %+v, want one", got)
	}
}

// A resource the character does not have at this level leaves the effect out, as for an extra_damage.
func TestPowerAttackOfAResourceTheCharacterLacksIsLeftOut(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay(
		Effect{Type: "resource", Resource: powerUses, Max: "2", Recharge: "short_rest", When: "level() >= 5"},
		Effect{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, Resource: powerUses},
	))
	if got := Derive(powerBuild(4), c).PowerAttacks; len(got) != 0 {
		t.Errorf("power attacks at level 4 = %+v, want none without the resource", got)
	}
	if got := Derive(powerBuild(5), c).PowerAttacks; len(got) != 1 || got[0].Resource != powerUses {
		t.Errorf("power attacks at level 5 = %+v, want one spending the resource", got)
	}
}

func TestWeaponLinesCarryTheirProperties(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay())
	d := Derive(powerBuild(3, "equipment:greatsword", "equipment:dagger"), c)
	byKey := map[string]Attack{}
	for _, a := range d.Attacks {
		byKey[a.Key] = a
	}
	for _, p := range []string{"weapon-property:heavy", "weapon-property:two-handed"} {
		if !slices.Contains(byKey["equipment:greatsword"].Properties, p) {
			t.Errorf("greatsword properties = %v, want %s", byKey["equipment:greatsword"].Properties, p)
		}
	}
	if !slices.Contains(byKey["equipment:dagger"].Properties, "weapon-property:finesse") {
		t.Errorf("dagger properties = %v, want finesse", byKey["equipment:dagger"].Properties)
	}
	if got := byKey[UnarmedStrikeKey].Properties; len(got) != 0 {
		t.Errorf("unarmed strike properties = %v, want none", got)
	}
}

func TestPowerAttackAllows(t *testing.T) {
	t.Parallel()
	heavy, twoHanded := "weapon-property:heavy", "weapon-property:two-handed"
	greatsword := Attack{Kind: "weapon", Melee: true, Proficient: true, Properties: []string{heavy, twoHanded, "weapon-property:other"}}
	longbow := Attack{Kind: "weapon", Proficient: true, Properties: []string{heavy, "weapon-property:ammunition"}}
	pseudo := Attack{Kind: "weapon", Melee: true, Proficient: true}
	unproficient := Attack{Kind: "weapon", Melee: true, Properties: []string{heavy, twoHanded}}
	spell := Attack{Kind: "spell", Melee: false}
	cases := []struct {
		name   string
		p      PowerAttack
		a      Attack
		ok     bool
		reason string // a part of the Portuguese reason, "" when allowed
	}{
		{"no filter, a melee weapon", PowerAttack{}, greatsword, true, ""},
		{"no filter, a ranged weapon", PowerAttack{Weapon: ExtraDamageAnyWeapon}, longbow, true, ""},
		{"melee, a melee weapon", PowerAttack{Weapon: ExtraDamageMeleeWeapon}, greatsword, true, ""},
		{"melee, a ranged weapon", PowerAttack{Weapon: ExtraDamageMeleeWeapon}, longbow, false, "arma de outro tipo"},
		{"ranged, a ranged weapon", PowerAttack{Weapon: ExtraDamageRangedWeapon}, longbow, true, ""},
		{"ranged, a melee weapon", PowerAttack{Weapon: ExtraDamageRangedWeapon}, greatsword, false, "arma de outro tipo"},
		{"a spell attack", PowerAttack{}, spell, false, "arma de outro tipo"},
		{"all of two properties, the weapon has both", PowerAttack{Properties: []string{heavy, twoHanded}}, greatsword, true, ""},
		{"all of two properties, the weapon has one", PowerAttack{Properties: []string{heavy, twoHanded}}, longbow, false, "sem a propriedade"},
		{"all of two properties, the weapon has none", PowerAttack{Properties: []string{heavy, twoHanded}}, Attack{Kind: "weapon", Melee: true}, false, "sem a propriedade"},
		{"a property of a pseudo weapon line", PowerAttack{Properties: []string{heavy}}, pseudo, false, "sem a propriedade"},
		{"no property filter matches a pseudo weapon line", PowerAttack{Weapon: ExtraDamageMeleeWeapon}, pseudo, true, ""},
		{"only proficient, a proficient line", PowerAttack{OnlyProficient: true}, greatsword, true, ""},
		{"only proficient, a line without proficiency", PowerAttack{OnlyProficient: true}, unproficient, false, "sem proficiência"},
		{"not only proficient, a line without proficiency", PowerAttack{}, unproficient, true, ""},
		{"the kind is read before the property", PowerAttack{Weapon: ExtraDamageRangedWeapon, Properties: []string{heavy}}, Attack{Kind: "weapon", Melee: true}, false, "arma de outro tipo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, why := tc.p.Allows(tc.a)
			if ok != tc.ok {
				t.Fatalf("Allows = %v (%q), want %v", ok, why, tc.ok)
			}
			if ok && why != "" {
				t.Errorf("an allowed attack gives the reason %q", why)
			}
			if !ok && !strings.Contains(why, tc.reason) {
				t.Errorf("reason = %q, want it to say %q", why, tc.reason)
			}
		})
	}
}

// The reason of a missing property names it in Portuguese when the effect carries the names.
func TestPowerAttackReasonNamesTheMissingProperty(t *testing.T) {
	t.Parallel()
	p := PowerAttack{Properties: []string{"weapon-property:heavy"}, PropertyNamesPT: []string{"Pesada"}}
	if ok, why := p.Allows(Attack{Kind: "weapon", Melee: true}); ok || why != "sem a propriedade Pesada" {
		t.Errorf("Allows = %v, %q, want a refusal that names Pesada", ok, why)
	}
}

func TestDerivedPowerAttackNamesItsProperties(t *testing.T) {
	c := withOverlay(t, powerAttackOverlay(Effect{
		Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, WeaponProperties: []string{"weapon-property:heavy", "weapon-property:two-handed"},
	}))
	p := Derive(powerBuild(3), c).PowerAttacks[0]
	if !slices.Equal(p.PropertyNamesPT, []string{"Pesada", "Duas mãos"}) {
		t.Errorf("property names = %v, want the Portuguese names in order", p.PropertyNamesPT)
	}
}

func TestPowerAttackIsOnTheMenu(t *testing.T) {
	menu := withOverlay(t, powerAttackOverlay()).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(ty MenuType) bool { return ty.Type == "power_attack" })
	if i < 0 {
		t.Fatal("no power_attack on the menu")
	}
	ty := menu.Types[i]
	if ty.NamePT != "Ataque poderoso" {
		t.Errorf("name = %q", ty.NamePT)
	}
	for _, name := range []string{"attack_penalty", "damage_bonus", "weapon", "weapon_properties", "only_proficient", "resource", "when", "text_pt"} {
		if !slices.ContainsFunc(ty.Fields, func(f MenuField) bool { return f.Name == name }) {
			t.Errorf("the menu type has no %s field", name)
		}
	}
	for _, f := range ty.Fields {
		switch f.Name {
		case "attack_penalty":
			if !f.Required || f.Min != 1 || f.Max != 10 {
				t.Errorf("attack_penalty = %+v, want required, 1 to 10", f)
			}
		case "damage_bonus":
			if !f.Required || f.Min != 1 || f.Max != 20 {
				t.Errorf("damage_bonus = %+v, want required, 1 to 20", f)
			}
		}
	}
	j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == ListWeaponProperties })
	if j < 0 {
		t.Fatal("no weapon property list on the menu")
	}
	for _, k := range []string{"weapon-property:heavy", "weapon-property:two-handed"} {
		if !slices.ContainsFunc(menu.Lists[j].Values, func(v MenuValue) bool { return v.Key == k }) {
			t.Errorf("the weapon property list has no %s", k)
		}
	}
	for _, v := range menu.Lists[j].Values {
		if !strings.HasPrefix(v.Key, "weapon-property:") {
			t.Errorf("the weapon property list has %q", v.Key)
		}
	}
}
