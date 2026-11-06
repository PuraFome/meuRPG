package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

func modCreature(mods ...TableCreatureDamageMod) TableCreature {
	c := packLeaderCreature()
	c.DamageMods = mods
	return c
}

func dmod(kind, qualifier string, types ...string) TableCreatureDamageMod {
	return TableCreatureDamageMod{Kind: kind, DamageTypes: types, Qualifier: qualifier}
}

const (
	dmPierce = "damage-type:piercing"
	dmSlash  = "damage-type:slashing"
	dmFire   = "damage-type:fire"
)

func TestACreatureDamageModRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	good := dmod("resist", "", dmFire)
	many := make([]TableCreatureDamageMod, MaxCreatureDamageMods+1)
	for i := range many {
		many[i] = good
	}
	cases := []struct {
		name          string
		mods          []TableCreatureDamageMod
		field, reason string
	}{
		{"a qualifier on a non-SRD type", []TableCreatureDamageMod{dmod("resist", "nonmagical", "spell:light")}, ".damage_mods[0].damage_types[0]", ReasonReference},
		{"a qualifier on an invalid kind", []TableCreatureDamageMod{dmod("heal", "nonmagical", dmPierce)}, ".damage_mods[0].kind", ReasonValue},
		{"13 entries", many, ".damage_mods", ReasonLimit},
		{"an empty kind", []TableCreatureDamageMod{good, dmod("", "", dmFire)}, ".damage_mods[1].kind", ReasonValue},
		{"an unknown kind", []TableCreatureDamageMod{dmod("heal", "", dmFire)}, ".damage_mods[0].kind", ReasonValue},
		{"no damage type", []TableCreatureDamageMod{dmod("resist", "")}, ".damage_mods[0].damage_types", ReasonValue},
		{"a key that is not a damage type", []TableCreatureDamageMod{dmod("resist", "", "spell:light")}, ".damage_mods[0].damage_types[0]", ReasonReference},
		{"a damage type twice", []TableCreatureDamageMod{good, dmod("immune", "", dmFire, dmFire)}, ".damage_mods[1].damage_types[1]", ReasonValue},
		{"an unknown qualifier", []TableCreatureDamageMod{good, dmod("resist", "silvered", dmPierce)}, ".damage_mods[1].qualifier", ReasonValue},
		{"a qualifier on a vulnerability", []TableCreatureDamageMod{good, dmod("vulnerable", "nonmagical", dmPierce)}, ".damage_mods[1].qualifier", ReasonValue},
		{"a qualifier on fire", []TableCreatureDamageMod{good, dmod("resist", "nonmagical", dmFire)}, ".damage_mods[1].qualifier", ReasonValue},
		{"a qualifier on fire among physical", []TableCreatureDamageMod{dmod("immune", "nonmagical", dmPierce, dmFire)}, ".damage_mods[0].qualifier", ReasonValue},
	}
	for _, tc := range cases {
		_, err := srd.With(creatureOverlay(modCreature(tc.mods...)))
		oe, ok := errors.AsType[*OverlayError](err)
		if !ok {
			t.Errorf("%s: err = %v, want an overlay error", tc.name, err)
			continue
		}
		if !slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool { return strings.HasSuffix(v.Field, tc.field) && v.Reason == tc.reason }) {
			t.Errorf("%s: refused at %+v, want %s (%s)", tc.name, oe.Violations(), tc.field, tc.reason)
		}
		// One bad entry is one violation: a bad kind or bad types do not add a second one on the qualifier.
		if strings.Contains(tc.name, "qualifier on a non-SRD") || strings.Contains(tc.name, "qualifier on an invalid kind") {
			if n := len(oe.Violations()); n != 1 {
				t.Errorf("%s: %d violations %+v, want exactly 1", tc.name, n, oe.Violations())
			}
		}
	}
	accepted := modCreature(
		dmod("resist", "nonmagical", dmPierce, dmSlash), dmod("immune", "nonmagical_unless_silvered", dmSlash),
		dmod("resist", "nonmagical_unless_adamantine", dmPierce), dmod("vulnerable", "", dmFire), dmod("immune", "", dmFire, "damage-type:cold"),
	)
	if _, err := srd.With(creatureOverlay(accepted)); err != nil {
		t.Errorf("valid entries were refused: %v", err)
	}
	if _, err := srd.With(creatureOverlay(modCreature(many[:MaxCreatureDamageMods]...))); err != nil {
		t.Errorf("12 entries were refused: %v", err)
	}
}

// A table creature's entries are added to its base's, with the SRD's qualifier and a note, so the stat block
// shows them and the combat reads them.
func TestACreatureDamageModsAreAddedToTheBases(t *testing.T) {
	t.Parallel()
	cr := modCreature(
		dmod("resist", "nonmagical_unless_silvered", dmPierce, dmSlash), dmod("vulnerable", "", dmFire), dmod("immune", "nonmagical", "damage-type:bludgeoning"),
	)
	c := withOverlay(t, creatureOverlay(cr))
	base, _ := c.CreatureByKey(cr.BaseKey)
	got, ok := c.CreatureByKey(cr.Key)
	if !ok {
		t.Fatal("no creature")
	}
	if len(got.Resistances) != len(base.Resistances)+1 || len(got.Vulnerabilities) != len(base.Vulnerabilities)+1 || len(got.Immunities) != len(base.Immunities)+1 {
		t.Fatalf("lists = %d/%d/%d, base %d/%d/%d; want one more each", len(got.Resistances), len(got.Vulnerabilities), len(got.Immunities),
			len(base.Resistances), len(base.Vulnerabilities), len(base.Immunities))
	}
	r := got.Resistances[len(got.Resistances)-1]
	if r.Qualifier != srd51.QualifierNonmagicalUnlessSilvered || r.Note != "from nonmagical weapons that aren't silvered" ||
		len(r.Types) != 2 || r.Types[0].Key != dmPierce || r.Types[0].NamePT == "" {
		t.Errorf("the resistance = %+v", r)
	}
	if v := got.Vulnerabilities[len(got.Vulnerabilities)-1]; v.Qualifier != "" || v.Note != "" || v.Types[0].Key != dmFire {
		t.Errorf("the vulnerability = %+v", v)
	}
	if i := got.Immunities[len(got.Immunities)-1]; i.Qualifier != srd51.QualifierNonmagical {
		t.Errorf("the immunity = %+v", i)
	}
	// The base is left as it was.
	if again, _ := c.CreatureByKey(cr.BaseKey); len(again.Resistances) != len(base.Resistances) {
		t.Errorf("the base's resistances changed")
	}
}
