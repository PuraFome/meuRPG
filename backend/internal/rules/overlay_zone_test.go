package rules

import (
	"errors"
	"strings"
	"testing"
)

// The area a table spell keeps on the map (RN-37): a closed set of fields, checked like the other table effects,
// one sentence for each refusal, at the field it is about.

// zoneSpell is a neutral fixture: a leveled spell of a cube placed at a point, with damage and a save for half.
func zoneSpell() TableSpell {
	ts := tableTestSpells()[2] // a level 2 concentration spell with a save
	ts.Key, ts.NamePT = "spell:zona-de-teste"+tableSuffix, "Zona de Teste"
	ts.Target = SpellTarget{Kind: TargetArea, Shape: ShapeCube, SizeFt: 10}
	ts.Save = &SpellSave{Ability: CON, OnSuccess: "half"}
	ts.Damage = []TableSpellDamage{{Type: "damage-type:force", Dice: "2d6", PerSlotLevel: "1d6"}}
	ts.Zone = &TableSpellZone{Trigger: ZoneBoth}
	return ts
}

func TestATableSpellKeepsItsAreaAsAZone(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	o := fullOverlay(t, base)
	o.Spells = []TableSpell{zoneSpell()}
	c, err := base.With(o)
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	rule, ok := c.SpellZone("spell:zona-de-teste" + tableSuffix)
	if !ok || rule.Trigger != ZoneBoth || rule.FollowsCaster {
		t.Errorf("SpellZone = %+v, %v; want both triggers, placed at a point", rule, ok)
	}
	if _, ok := c.SpellZone("spell:fireball"); ok {
		t.Error("an SRD spell has a table zone")
	}
}

func TestATableZoneIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	for _, tc := range []struct {
		name  string
		edit  func(*TableSpell)
		field string
	}{
		{"no trigger", func(s *TableSpell) { s.Zone.Trigger = "" }, ".zone.trigger"},
		{"an unknown trigger", func(s *TableSpell) { s.Zone.Trigger = "whenever" }, ".zone.trigger"},
		{"a cone", func(s *TableSpell) { s.Target = SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15} }, ".zone"},
		{"a target that is not an area", func(s *TableSpell) { s.Target = SpellTarget{Kind: TargetCreature} }, ".zone"},
		{"no damage", func(s *TableSpell) { s.Damage, s.Save = nil, nil }, ".zone"},
		{"two damage types", func(s *TableSpell) {
			s.Damage = append(s.Damage, TableSpellDamage{Type: "damage-type:fire", Dice: "1d4"})
		}, ".zone"},
		{"a spell attack", func(s *TableSpell) { s.Save, s.Attack = nil, "ranged" }, ".zone"},
		{"healing", func(s *TableSpell) { s.Heal = &TableSpellHeal{Dice: "1d4"} }, ".zone"},
		{"no time", func(s *TableSpell) { s.Concentration, s.Duration = false, TableDuration{Kind: DurationInstantaneous} }, ".zone"},
		{"a cantrip", func(s *TableSpell) {
			s.Level = 0
			s.Concentration = false
			s.Damage = []TableSpellDamage{{Type: "damage-type:force", Dice: "1d6", PerTier: "1d6"}}
		}, ".zone"},
		{"following the caster at a distance", func(s *TableSpell) { s.Zone.FollowsCaster = true }, ".zone.follows_caster"},
		{"range Self that does not follow", func(s *TableSpell) { s.Range = TableRange{Kind: RangeSelf} }, ".zone.follows_caster"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := zoneSpell()
			tc.edit(&ts)
			o := fullOverlay(t, base)
			o.Spells = []TableSpell{ts}
			_, err := base.With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("With() error = %v, want a refusal", err)
			}
			for _, v := range oe.Violations() {
				if strings.HasSuffix(v.Field, tc.field) {
					return
				}
			}
			t.Errorf("no violation at %s: %v", tc.field, err)
		})
	}
}

func TestATableZoneThatFollowsTheCasterNeedsRangeSelf(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	ts := zoneSpell()
	ts.Range = TableRange{Kind: RangeSelf}
	ts.Target = SpellTarget{Kind: TargetArea, Shape: ShapeSphere, SizeFt: 15}
	ts.Zone.FollowsCaster = true
	o := fullOverlay(t, base)
	o.Spells = []TableSpell{ts}
	c, err := base.With(o)
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	if rule, ok := c.SpellZone(ts.Key); !ok || !rule.FollowsCaster {
		t.Errorf("SpellZone = %+v, %v; want one that follows the caster", rule, ok)
	}
}
