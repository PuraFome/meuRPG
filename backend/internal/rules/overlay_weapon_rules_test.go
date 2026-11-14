package rules

import "testing"

// #580 S2: the rules a table weapon gets by being an equipment row, each read through a sheet's attack line or armor class.

const (
	wFinesse = "weapon:florete-fino" + tableSuffix
	wLight   = "weapon:faca-curta" + tableSuffix
	wHeavy   = "weapon:maca-de-guerra" + tableSuffix
	wBow     = "weapon:arco-curto-do-vale" + tableSuffix
	wThrown  = "weapon:azagaia-fina" + tableSuffix
	wFlail   = "weapon:mangual-longo" + tableSuffix
)

func ruleWeapons() []TableWeapon {
	const p = "weapon-property:"
	return []TableWeapon{
		{TableEntry: TableEntry{Key: wFinesse, NamePT: "Florete Fino"}, Category: "martial", Damage: "1d8", DamageType: "damage-type:piercing", Properties: []string{p + "finesse"}},
		{TableEntry: TableEntry{Key: wLight, NamePT: "Faca Curta"}, Category: "simple", Damage: "1d4", DamageType: "damage-type:piercing", Properties: []string{p + "light"}},
		{TableEntry: TableEntry{Key: wHeavy, NamePT: "Maça de Guerra"}, Category: "martial", Damage: "2d6", DamageType: "damage-type:bludgeoning", Properties: []string{p + "heavy", p + "two-handed"}},
		{TableEntry: TableEntry{Key: wBow, NamePT: "Arco Curto do Vale"}, Category: "simple", Ranged: true, Damage: "1d6", DamageType: "damage-type:piercing", Properties: []string{p + "ammunition"}, NormalRangeFt: 80, LongRangeFt: 320},
		{TableEntry: TableEntry{Key: wThrown, NamePT: "Azagaia Fina"}, Category: "simple", Damage: "1d6", DamageType: "damage-type:piercing", Properties: []string{p + "thrown"}, NormalRangeFt: 30, LongRangeFt: 120},
		{TableEntry: TableEntry{Key: wFlail, NamePT: "Mangual Longo"}, Category: "martial", Damage: "1d8", DamageType: "damage-type:bludgeoning", Properties: []string{p + "reach"}},
	}
}

func tableLine(t *testing.T, d Derived, key string) Attack {
	t.Helper()
	for _, a := range d.Attacks {
		if a.Key == key {
			return a
		}
	}
	t.Fatalf("no attack line for %s in %+v", key, d.Attacks)
	return Attack{}
}

func ruleSheet(str, dex int, weapons ...string) Build {
	b := weaponBuild("class:fighter", weapons...)
	b.BaseScores = map[Ability]int{STR: str, DEX: dex, CON: 14, INT: 8, WIS: 10, CHA: 10}
	return b
}

func TestATableWeaponAttacksWithTheAbilityItsPropertiesPick(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ruleWeapons()})
	for _, tc := range []struct {
		name     string
		str, dex int
		weapon   string
		want     Ability
	}{
		{"a finesse weapon takes the higher, DEX", 10, 16, wFinesse, DEX},
		{"a finesse weapon takes the higher, STR", 16, 10, wFinesse, STR},
		{"a ranged weapon uses DEX even with a higher STR", 18, 10, wBow, DEX},
		{"a plain melee weapon uses STR even with a higher DEX", 10, 18, wLight, STR},
		{"a thrown melee weapon uses STR", 10, 18, wThrown, STR},
	} {
		d := Derive(ruleSheet(tc.str, tc.dex, tc.weapon), c)
		if got := tableLine(t, d, tc.weapon).Ability; got != tc.want {
			t.Errorf("%s: attacks with %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestATableWeaponsDamageAndVersatileLineAndTheShield(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, Overlay{Revision: 1, Weapons: tableWeapons()})
	with := ruleSheet(16, 10, weaponHalberd)
	with.Shield = true
	without := with
	without.Shield = false
	for _, b := range []Build{with, without} {
		a := tableLine(t, Derive(b, c), weaponHalberd)
		if a.Damage != "1d10+3" || a.VersatileDamage != "1d12+3" {
			t.Errorf("shield %v: damage %q and versatile %q, want 1d10+3 and 1d12+3", b.Shield, a.Damage, a.VersatileDamage)
		}
	}
	if w, wo := Derive(with, c).ArmorClass, Derive(without, c).ArmorClass; w != wo+2 {
		t.Errorf("a versatile weapon with a shield: AC %d, without %d, want the shield to count (+2)", w, wo)
	}
}

func TestATableWeaponsReachAndRange(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ruleWeapons()})
	d := Derive(ruleSheet(12, 12, wLight, wFlail, wBow, wThrown), c)
	for key, want := range map[string][2]int{wLight: {5, 0}, wFlail: {10, 0}, wBow: {80, 320}, wThrown: {30, 120}} {
		a := tableLine(t, d, key)
		if a.RangeFt != want[0] || a.LongRangeFt != want[1] {
			t.Errorf("%s: range %d/%d, want %d/%d", key, a.RangeFt, a.LongRangeFt, want[0], want[1])
		}
	}
	bad := ruleWeapons()[3]
	bad.NormalRangeFt, bad.LongRangeFt = 0, 0
	_, err := loadForTest(t).With(Overlay{Revision: 1, Weapons: []TableWeapon{bad}})
	violationAt(t, err, "weapons[0].normal_range_ft", ReasonValue)
}

func TestATableWeaponTakesPartInTwoWeaponFightingAndDropsTheShieldWhenTwoHanded(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ruleWeapons()})
	b := ruleSheet(12, 14, wLight, wFinesse)
	b.FeatureChoices = []string{"feature:fighter-fighting-style-two-weapon-fighting"}
	d := Derive(b, c)
	if !d.TwoWeaponFighting || !tableLine(t, d, wLight).Light || tableLine(t, d, wFinesse).Light {
		t.Errorf("two-weapon fighting %v, light line %v, finesse line light %v: want the style on, the light weapon light and the other not",
			d.TwoWeaponFighting, tableLine(t, d, wLight).Light, tableLine(t, d, wFinesse).Light)
	}

	armed := ruleSheet(12, 14, wHeavy)
	armed.Shield = true
	got, bare := Derive(armed, c), armed
	bare.Weapons = nil
	if !got.ShieldIdle || got.ArmorClass != Derive(bare, c).ArmorClass-2 {
		t.Errorf("a two-handed table weapon with a shield: idle %v, AC %d; want the shield idle and AC without it (%d)", got.ShieldIdle, got.ArmorClass, Derive(bare, c).ArmorClass-2)
	}
	if !tableLine(t, got, wHeavy).TwoHanded {
		t.Errorf("the heavy weapon's line is not two-handed")
	}
}

func TestAPowerAttacksPropertyFilterMatchesATableWeapon(t *testing.T) {
	t.Parallel()
	const key = "feat:estilo-de-haste" + tableSuffix
	o := Overlay{Revision: 1, Weapons: ruleWeapons(), Strict: []string{key}}
	o.Feats = []TableFeat{{
		TableEntry: TableEntry{Key: key, NamePT: "Estilo de Haste"}, DescPT: []string{"Texto."},
		Effects: []Effect{{Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, Weapon: ExtraDamageMeleeWeapon, WeaponProperties: []string{"weapon-property:reach"}}},
	}}
	c := withOverlay(t, o)
	b := ruleSheet(12, 12, wFlail, wLight, wBow)
	b.Feats = []string{key}
	d := Derive(b, c)
	if len(d.PowerAttacks) != 1 {
		t.Fatalf("power attacks = %+v, want the feat's one", d.PowerAttacks)
	}
	p := d.PowerAttacks[0]
	for weapon, want := range map[string]bool{wFlail: true, wLight: false, wBow: false} {
		if ok, why := p.Allows(tableLine(t, d, weapon)); ok != want {
			t.Errorf("the trade on %s = %v (%s), want %v: only the table weapon with the property goes with it", weapon, ok, why, want)
		}
	}
}
