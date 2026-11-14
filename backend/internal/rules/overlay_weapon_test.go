package rules

import (
	"errors"
	"slices"
	"testing"
)

// #580 S2: a weapon of the table is an equipment row. Every name here is invented.

const (
	weaponHalberd = "weapon:alabarda-curta" + tableSuffix
	weaponSpear   = "weapon:lanca-longa" + tableSuffix
	weaponBlow    = "weapon:zarabatana-fina" + tableSuffix
)

func tableWeapons() []TableWeapon {
	return []TableWeapon{
		{
			TableEntry: TableEntry{Key: weaponHalberd, NamePT: "Alabarda Curta"}, DescPT: []string{"Uma haste com lâmina."}, Category: "martial",
			Damage: "1d10", DamageType: "damage-type:slashing", TwoHandedDamage: "1d12",
			Properties: []string{"weapon-property:versatile", "weapon-property:reach"}, WeightOz: 96, PriceCP: 2000,
		},
		{
			TableEntry: TableEntry{Key: weaponSpear, NamePT: "Lança Longa"}, Category: "simple", Damage: "1d6", DamageType: "damage-type:piercing",
			Properties: []string{"weapon-property:thrown"}, NormalRangeFt: 20, LongRangeFt: 60,
		},
		{
			TableEntry: TableEntry{Key: weaponBlow, NamePT: "Zarabatana Fina"}, Category: "simple", Ranged: true, Damage: "1", DamageType: "damage-type:piercing",
			Properties: []string{"weapon-property:ammunition", "weapon-property:loading"}, NormalRangeFt: 25, LongRangeFt: 100,
		},
	}
}

func weaponBuild(class string, weapons ...string) Build {
	b := torenLevelUp()
	b.Classes = []ClassLevel{{Class: class, Level: 1}}
	b.Armor, b.Shield, b.FeatureChoices, b.Weapons = "", false, nil, weapons
	return b
}

func TestATableWeaponIsAnEquipmentRowTheSheetAttacksWith(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, Overlay{Revision: 1, Weapons: tableWeapons()})

	var entry WeaponEntry
	for _, w := range c.Catalog().Weapons {
		if w.Key == weaponHalberd {
			entry = w
		}
	}
	if entry.NamePT != "Alabarda Curta" || entry.Category != "martial" || entry.Range != "melee" || entry.Damage != "1d10" || entry.TwoHandedDamage != "1d12" || entry.NormalRangeFt != 0 {
		t.Fatalf("catalog entry = %+v, want the table's martial melee weapon", entry)
	}
	fighter := Derive(weaponBuild("class:fighter", weaponHalberd, weaponSpear, weaponBlow), c)
	byKey := map[string]Attack{}
	for _, a := range fighter.Attacks {
		byKey[a.Key] = a
	}
	if a := byKey[weaponHalberd]; !a.Proficient || !a.Melee || a.DamageType != "damage-type:slashing" || !slices.Contains(a.Properties, "weapon-property:reach") {
		t.Errorf("a fighter's halberd = %+v, want a proficient melee line", a)
	}
	if a := byKey[weaponBlow]; !a.Proficient || a.Melee {
		t.Errorf("the blowgun = %+v, want a ranged line with the simple weapons' proficiency", a)
	}
	wizard := Derive(weaponBuild("class:wizard", weaponHalberd, weaponSpear), c)
	for _, a := range wizard.Attacks {
		if a.Key == weaponHalberd && a.Proficient {
			t.Errorf("a wizard is proficient with a martial table weapon: %+v", a)
		}
		if a.Key == weaponSpear && a.Proficient {
			t.Errorf("a wizard is proficient with a simple weapon it has no proficiency for: %+v", a)
		}
	}
	if err := Validate(weaponBuild("class:fighter", weaponHalberd), c); err != nil {
		t.Errorf("a table weapon in the weapon list is refused: %v", err)
	}
}

func TestATableWeaponAnArchivedOneStaysOnTheSheetAndIsMarked(t *testing.T) {
	t.Parallel()
	ws := tableWeapons()
	ws[0].Archived = true
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ws, Off: []string{weaponSpear}})
	marks := map[string][2]bool{}
	for _, w := range c.Catalog().Weapons {
		marks[w.Key] = [2]bool{w.Archived, w.Off}
	}
	if marks[weaponHalberd] != [2]bool{true, false} || marks[weaponSpear] != [2]bool{false, true} || marks["equipment:longsword"] != [2]bool{} {
		t.Errorf("marks = %v, want archived, off and unmarked", marks)
	}
	b := weaponBuild("class:fighter", weaponHalberd, weaponSpear)
	if got := c.ArchivedKeys(b); !slices.Equal(got, []string{weaponHalberd}) {
		t.Errorf("archived keys = %v, want the halberd", got)
	}
	if got := c.OffKeys(b); !slices.Equal(got, []string{weaponSpear}) {
		t.Errorf("off keys = %v, want the spear", got)
	}
	if !slices.ContainsFunc(Derive(b, c).Attacks, func(a Attack) bool { return a.Key == weaponHalberd }) {
		t.Errorf("the archived weapon left the sheet")
	}
}

func TestATableWeaponRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edit  func(w *TableWeapon)
		field string
	}{
		{"no category", func(w *TableWeapon) { w.Category = "" }, "weapons[0].category"},
		{"damage that is not dice", func(w *TableWeapon) { w.Damage = "1d7" }, "weapons[0].damage"},
		{"a damage type that does not exist", func(w *TableWeapon) { w.DamageType = "damage-type:nada" }, "weapons[0].damage_type"},
		{"two-handed damage without versatile", func(w *TableWeapon) { w.Properties = nil }, "weapons[0].two_handed_damage"},
		{"versatile without two-handed damage", func(w *TableWeapon) { w.TwoHandedDamage = "" }, "weapons[0].two_handed_damage"},
		{"a property that does not exist", func(w *TableWeapon) { w.Properties[1] = "weapon-property:nada" }, "weapons[0].properties[1]"},
		{"a property twice", func(w *TableWeapon) { w.Properties[1] = w.Properties[0] }, "weapons[0].properties[1]"},
		{"a range on a melee weapon", func(w *TableWeapon) { w.NormalRangeFt, w.LongRangeFt = 10, 20 }, "weapons[0].normal_range_ft"},
		{"a negative weight", func(w *TableWeapon) { w.WeightOz = -1 }, "weapons[0].weight_oz"},
		{"a price that is too high", func(w *TableWeapon) { w.PriceCP = MaxWeaponPriceCP + 1 }, "weapons[0].price_cp"},
		{"a key of another kind", func(w *TableWeapon) { w.Key = "feat:alabarda" + tableSuffix }, "weapons[0].key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := tableWeapons()
			tc.edit(&ws[0])
			_, err := loadForTest(t).With(Overlay{Revision: 1, Weapons: ws})
			var oe *OverlayError
			if !errors.As(err, &oe) || !hasWeaponViolation(oe, tc.field) {
				t.Errorf("want a violation at %s, got %v", tc.field, err)
			}
		})
	}
	// A thrown or ranged weapon needs both ranges, the long one at least the normal one.
	for name, edit := range map[string]func(w *TableWeapon){
		"thrown with no range": func(w *TableWeapon) { w.NormalRangeFt, w.LongRangeFt = 0, 0 },
		"long below normal":    func(w *TableWeapon) { w.LongRangeFt = 10 },
	} {
		ws := tableWeapons()
		edit(&ws[1])
		_, err := loadForTest(t).With(Overlay{Revision: 1, Weapons: ws})
		var oe *OverlayError
		if !errors.As(err, &oe) || (!hasWeaponViolation(oe, "weapons[1].normal_range_ft") && !hasWeaponViolation(oe, "weapons[1].long_range_ft")) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	// The key of the SRD's weapon is taken.
	ws := tableWeapons()
	ws[0].Key = "weapon:" + "longsword" + tableSuffix
	ws[1].Key = ws[0].Key
	if _, err := loadForTest(t).With(Overlay{Revision: 1, Weapons: ws[:2]}); err == nil {
		t.Errorf("two weapons with one key were accepted")
	}
}

func hasWeaponViolation(oe *OverlayError, field string) bool {
	return slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool { return v.Field == field })
}
