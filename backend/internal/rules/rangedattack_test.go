package rules

import (
	"errors"
	"testing"
)

// The feats of the ranged_attack tests: a sniper's spells (twice the range, past
// cover), a feat that triples the range, and a sharpshooter's weapons (past cover,
// no disadvantage at long range). The names are the table's; the engine reads only
// the effects.
const (
	spellSniperFeat  = "feat:atirador-de-magia" + tableSuffix
	tripleRangeFeat  = "feat:alcance-triplo" + tableSuffix
	sharpshooterFeat = "feat:atirador-de-elite" + tableSuffix
)

// sniperTable is a table with the three feats.
func sniperTable(t *testing.T) *Content {
	t.Helper()
	feat := func(key, name string, e Effect) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto de " + name + "."}, Effects: []Effect{e}}
	}
	c, err := loadForTest(t).With(Overlay{Revision: 1, Feats: []TableFeat{
		feat(spellSniperFeat, "Atirador de magia", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 2, IgnoreCover: true}),
		feat(tripleRangeFeat, "Alcance triplo", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 3}),
		feat(sharpshooterFeat, "Atirador de elite", Effect{Type: "ranged_attack", Applies: RangedAttackWeapon, IgnoreCover: true, NoLongRangeDisadvantage: true}),
	}})
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	return c
}

// TestRangedAttackEffectShapes: the table's ranged_attack is accepted in each of
// its shapes, on a feat and on a class feature, and refused at the field that is
// wrong: no applies, a multiplier on weapons or outside 2 to 3, the long range on
// spells, an effect that changes nothing, tags, a stray field, and its switches on
// another type.
func TestRangedAttackEffectShapes(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name       string
		effect     Effect
		field, why string // "" when it is accepted
	}{
		{"spells, twice the range and past cover", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 2, IgnoreCover: true}, "", ""},
		{"spells, three times the range", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 3}, "", ""},
		{"spells, past cover only", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, IgnoreCover: true}, "", ""},
		{"weapons, past cover and at long range", Effect{Type: "ranged_attack", Applies: RangedAttackWeapon, IgnoreCover: true, NoLongRangeDisadvantage: true}, "", ""},
		{"weapons, at long range only, at level 5", Effect{Type: "ranged_attack", Applies: RangedAttackWeapon, NoLongRangeDisadvantage: true, When: "level() >= 5"}, "", ""},
		{"no applies", Effect{Type: "ranged_attack", IgnoreCover: true}, ".applies", ReasonValue},
		{"applies that is not on the list", Effect{Type: "ranged_attack", Applies: "melee", IgnoreCover: true}, ".applies", ReasonValue},
		{"a multiplier on weapons", Effect{Type: "ranged_attack", Applies: RangedAttackWeapon, RangeMultiplier: 2}, ".range_multiplier", ReasonValue},
		{"a multiplier of 4", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 4}, ".range_multiplier", ReasonValue},
		{"a multiplier of 1", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 1}, ".range_multiplier", ReasonValue},
		{"a negative multiplier", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: -2}, ".range_multiplier", ReasonValue},
		{"the long range on spells", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, NoLongRangeDisadvantage: true}, ".no_long_range_disadvantage", ReasonValue},
		{"an effect that changes nothing", Effect{Type: "ranged_attack", Applies: RangedAttackSpell}, ".type", ReasonValue},
		{"tags", Effect{Type: "ranged_attack", Applies: RangedAttackWeapon, IgnoreCover: true, Tags: []string{"against:magic"}}, ".tags", ReasonValue},
		{"a stray count", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, IgnoreCover: true, Count: 2}, ".count", ReasonValue},
		{"a broken condition", Effect{Type: "ranged_attack", Applies: RangedAttackSpell, IgnoreCover: true, When: "level( >"}, ".when", ReasonFormula},
		{"ignore_cover on a modifier", Effect{Type: "modifier", Target: "ac", Mode: "add", Value: "1", IgnoreCover: true}, ".ignore_cover", ReasonValue},
		{"a multiplier on an extra attack", Effect{Type: "extra_attack", Count: 2, RangeMultiplier: 2}, ".range_multiplier", ReasonValue},
		{"the long range on a note", Effect{Type: "note", TextPT: "Mira longe.", NoLongRangeDisadvantage: true}, ".no_long_range_disadvantage", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := "feat:teste" + tableSuffix
			_, err := srd.With(Overlay{Revision: 1, Strict: []string{key}, Feats: []TableFeat{{
				TableEntry: TableEntry{Key: key, NamePT: "Teste"}, DescPT: []string{"Texto."}, Effects: []Effect{tc.effect},
			}}})
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				// A class feature may have it too: it is not a feat's alone.
				if _, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{tc.effect})}}); err != nil {
					t.Fatalf("on a class feature: With() error = %v", err)
				}
				return
			}
			oe, ok := errors.AsType[*OverlayError](err)
			if !ok {
				t.Fatalf("With() error = %v, want an OverlayError at %s", err, tc.field)
			}
			want := "feats[0].effects[0]" + tc.field
			found := false
			for _, v := range oe.Violations() {
				found = found || (v.Field == want && v.Reason == tc.why)
			}
			if !found {
				t.Errorf("violations = %+v, want one at %s (%s)", oe.Violations(), want, tc.why)
			}
		})
	}
}

// TestRangedAttackFieldsBelongToItsType: the SRD loader refuses the switches of a
// ranged_attack on any other type, and checks a ranged_attack as the table's editor
// does; a stored table effect with a stray switch has it stripped, not refused.
func TestRangedAttackFieldsBelongToItsType(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for name, e := range map[string]*Effect{
		"ignore_cover on a modifier":           {Type: "modifier", Target: "ac", Mode: "add", Value: "1", IgnoreCover: true},
		"a multiplier on a sense":              {Type: "sense", Sense: "darkvision", RangeFt: 60, RangeMultiplier: 2},
		"the long range on an extra attack":    {Type: "extra_attack", Count: 2, NoLongRangeDisadvantage: true},
		"a ranged_attack that changes nothing": {Type: "ranged_attack", Applies: RangedAttackWeapon},
		"a ranged_attack of a maneuver's kind": {Type: "ranged_attack", Applies: ManeuverDamage, IgnoreCover: true},
	} {
		if err := srd.c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	if err := srd.c.compileEffect("feature:x", &Effect{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 2}); err != nil {
		t.Errorf("a good ranged_attack: %v", err)
	}
	// Stored, not being written: the stray switch is dropped and the effect still works.
	stored := TableFeat{
		TableEntry: TableEntry{Key: "feat:guardado" + tableSuffix, NamePT: "Guardado"}, DescPT: []string{"Texto."},
		Effects: []Effect{{Type: "modifier", Target: "initiative", Mode: "add", Value: "1", IgnoreCover: true}},
	}
	if _, err := srd.With(Overlay{Revision: 1, Feats: []TableFeat{stored}}); err != nil {
		t.Errorf("a stored effect with a stray switch is refused: %v", err)
	}
}

// TestRangedAttackEffectsAddUpOnTheSheet: the sheet holds the largest range multiplier
// and whether any effect makes spell or weapon attacks ignore cover or keep their aim
// at long range; the attack cantrip's line reaches the multiplied range, and so does
// SpellRangeFt for a spell with an attack roll and a range in feet, never for a save,
// a touch or a character without a multiplier.
func TestRangedAttackEffectsAddUpOnTheSheet(t *testing.T) {
	t.Parallel()
	c := sniperTable(t)

	plain := Derive(pensantus(), c)
	if plain.SpellAttackRangeMultiplier != 0 || plain.SpellAttackIgnoresCover || plain.RangedWeaponIgnoresCover || plain.RangedWeaponNoLongRangeDisadvantage {
		t.Fatalf("a wizard with no feat: %+v", []any{plain.SpellAttackRangeMultiplier, plain.SpellAttackIgnoresCover, plain.RangedWeaponIgnoresCover, plain.RangedWeaponNoLongRangeDisadvantage})
	}
	if fb, _ := attackOf(plain, "spell:fire-bolt"); fb.RangeFt != 120 {
		t.Fatalf("Fire Bolt with no feat reaches %d ft, want 120", fb.RangeFt)
	}
	if got := plain.SpellRangeFt("ranged", RangeRanged, 120); got != 120 {
		t.Errorf("SpellRangeFt with no multiplier = %d, want 120", got)
	}

	b := pensantus()
	b.Feats = []string{spellSniperFeat}
	sniper := Derive(b, c)
	if sniper.SpellAttackRangeMultiplier != 2 || !sniper.SpellAttackIgnoresCover || sniper.RangedWeaponIgnoresCover || sniper.RangedWeaponNoLongRangeDisadvantage {
		t.Fatalf("the spell sniper: multiplier %d, spells past cover %v, weapons past cover %v, weapons at long range %v",
			sniper.SpellAttackRangeMultiplier, sniper.SpellAttackIgnoresCover, sniper.RangedWeaponIgnoresCover, sniper.RangedWeaponNoLongRangeDisadvantage)
	}
	if fb, _ := attackOf(sniper, "spell:fire-bolt"); fb.RangeFt != 240 {
		t.Errorf("Fire Bolt of the spell sniper reaches %d ft, want 240", fb.RangeFt)
	}
	if rf, _ := attackOf(sniper, "spell:ray-of-frost"); rf.RangeFt != 120 {
		t.Errorf("Ray of Frost of the spell sniper reaches %d ft, want 120 (twice its 60)", rf.RangeFt)
	}
	for _, tc := range []struct {
		name               string
		attackType, kind   string
		rangeFt, wantRange int
	}{
		{"a spell attack with a range", "ranged", RangeRanged, 120, 240},
		{"a melee spell attack with a range", "melee", RangeRanged, 30, 60},
		{"a saving throw spell", "", RangeRanged, 150, 150},
		{"a touch spell attack", "melee", RangeTouch, 0, 0},
		{"a self spell", "", RangeSelf, 0, 0},
	} {
		if got := sniper.SpellRangeFt(tc.attackType, tc.kind, tc.rangeFt); got != tc.wantRange {
			t.Errorf("%s: SpellRangeFt = %d, want %d", tc.name, got, tc.wantRange)
		}
	}

	// The largest multiplier counts; the switches of every feat add up.
	b.Feats = []string{spellSniperFeat, tripleRangeFeat, sharpshooterFeat}
	all := Derive(b, c)
	if all.SpellAttackRangeMultiplier != 3 || !all.SpellAttackIgnoresCover || !all.RangedWeaponIgnoresCover || !all.RangedWeaponNoLongRangeDisadvantage {
		t.Fatalf("three feats: multiplier %d, spells past cover %v, weapons past cover %v, weapons at long range %v",
			all.SpellAttackRangeMultiplier, all.SpellAttackIgnoresCover, all.RangedWeaponIgnoresCover, all.RangedWeaponNoLongRangeDisadvantage)
	}
	if fb, _ := attackOf(all, "spell:fire-bolt"); fb.RangeFt != 360 {
		t.Errorf("Fire Bolt with the triple range reaches %d ft, want 360", fb.RangeFt)
	}
	// The weapon feat alone changes no spell.
	b.Feats = []string{sharpshooterFeat}
	weapons := Derive(b, c)
	if weapons.SpellAttackRangeMultiplier != 0 || weapons.SpellAttackIgnoresCover || !weapons.RangedWeaponIgnoresCover || !weapons.RangedWeaponNoLongRangeDisadvantage {
		t.Errorf("the sharpshooter: %+v", []any{weapons.SpellAttackRangeMultiplier, weapons.SpellAttackIgnoresCover, weapons.RangedWeaponIgnoresCover, weapons.RangedWeaponNoLongRangeDisadvantage})
	}
	if fb, _ := attackOf(weapons, "spell:fire-bolt"); fb.RangeFt != 120 {
		t.Errorf("Fire Bolt of the sharpshooter reaches %d ft, want 120", fb.RangeFt)
	}
}

// TestRangedAttackHonoursItsCondition: an effect with a `when` that does not hold
// adds nothing to the sheet.
func TestRangedAttackHonoursItsCondition(t *testing.T) {
	t.Parallel()
	key := "feat:mira-tardia" + tableSuffix
	c, err := loadForTest(t).With(Overlay{Revision: 1, Feats: []TableFeat{{
		TableEntry: TableEntry{Key: key, NamePT: "Mira tardia"}, DescPT: []string{"Texto."},
		Effects: []Effect{{Type: "ranged_attack", Applies: RangedAttackSpell, RangeMultiplier: 2, When: "level() >= 5"}},
	}}})
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	b := pensantus() // level 3
	b.Feats = []string{key}
	if d := Derive(b, c); d.SpellAttackRangeMultiplier != 0 {
		t.Errorf("at level 3 the multiplier is %d, want none", d.SpellAttackRangeMultiplier)
	}
	b.Classes[0].Level = 5
	if d := Derive(b, c); d.SpellAttackRangeMultiplier != 2 {
		t.Errorf("at level 5 the multiplier is %d, want 2", d.SpellAttackRangeMultiplier)
	}
}
