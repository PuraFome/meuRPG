package rules

import (
	"slices"
	"testing"
)

// The targets of the spells (MR-025, MR-045, RN-23): worked out from the
// 5e-database's structured area_of_effect for the SRD, the master's own for a
// table spell. These tests need no database.

// TestSRDSpellTargets: a sample of the SRD, as the "Magias" page writes it.
func TestSRDSpellTargets(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, want := range map[string]struct {
		target SpellTarget
		label  string
	}{
		"spell:burning-hands":  {SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15}, "Cone de 4,5 m"},
		"spell:fireball":       {SpellTarget{Kind: TargetArea, Shape: ShapeSphere, SizeFt: 20}, "Esfera de 6 m"},
		"spell:lightning-bolt": {SpellTarget{Kind: TargetArea, Shape: ShapeLine, SizeFt: 100}, "Linha de 30 m"},
		"spell:cone-of-cold":   {SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 60}, "Cone de 18 m"},
		"spell:thunderwave":    {SpellTarget{Kind: TargetArea, Shape: ShapeCube, SizeFt: 15}, "Cubo de 4,5 m"},
		"spell:flame-strike":   {SpellTarget{Kind: TargetArea, Shape: ShapeCylinder, SizeFt: 40}, "Cilindro de 12 m"},
		"spell:cure-wounds":    {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		"spell:fire-bolt":      {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		"spell:shield":         {SpellTarget{Kind: TargetSelf}, "Só quem conjura"},
		"spell:mage-armor":     {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		// The text says "an additional creature" for each circle above.
		"spell:hold-person": {SpellTarget{Kind: TargetCreature, PerSlotLevel: 1}, "Uma criatura"},
		// The text says "up to three creatures", which the database has no number for.
		"spell:bless": {SpellTarget{Kind: TargetCreatures, PerSlotLevel: 1}, "Várias criaturas"},
		// A dart or a ray each.
		"spell:magic-missile": {SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 1}, "Várias criaturas"},
		"spell:scorching-ray": {SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 1}, "Várias criaturas"},
	} {
		d, ok := c.SpellDetails(key)
		if !ok {
			t.Errorf("%s is not in the content", key)
			continue
		}
		if d.Target != want.target {
			t.Errorf("%s target = %+v, want %+v", key, d.Target, want.target)
		}
		if got := d.Target.LabelPT(); got != want.label {
			t.Errorf("%s label = %q, want %q", key, got, want.label)
		}
	}
}

// TestEverySRDSpellHasATarget: no spell of the SRD is left without one, and a spell
// the database gives an area for is always an area.
func TestEverySRDSpellHasATarget(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	areas := 0
	for _, e := range c.Catalog().Spells {
		d, _ := c.SpellDetails(e.Key)
		if d.Target.Kind == "" || d.Target.LabelPT() == "" {
			t.Errorf("%s has no target: %+v", e.Key, d.Target)
		}
		if sp := c.c.spells[e.Key]; (sp.AreaType != "") != d.Target.IsArea() {
			t.Errorf("%s: the database area %q and the target %+v disagree", e.Key, sp.AreaType, d.Target)
		}
		if d.Target.IsArea() {
			areas++
		}
	}
	if areas != 88 {
		t.Errorf("%d SRD spells are areas, the database says 88", areas)
	}
}

// structuredOnly are the SRD spells where the database's area_of_effect and the old
// reading of the text disagree: the text did not call them an area (a point, a
// wall, a spell that "fills" a sphere, a sense of the caster), so they used to take
// one target. The structured data wins; the text stays the fallback for the others.
var structuredOnly = []string{
	"spell:arcane-eye", "spell:blade-barrier", "spell:control-water", "spell:detect-evil-and-good", "spell:detect-magic",
	"spell:detect-poison-and-disease", "spell:forbiddance", "spell:forcecage", "spell:guardian-of-faith", "spell:hallow",
	"spell:magnificent-mansion", "spell:mirage-arcane", "spell:prismatic-wall", "spell:private-sanctum", "spell:speak-with-plants",
	"spell:storm-of-vengeance", "spell:telekinesis", "spell:teleportation-circle", "spell:wall-of-fire", "spell:wall-of-thorns",
	"spell:wind-wall",
}

// TestStructuredAreasAgainstTheText lists where the two readings disagree, so a
// change of the data or of the patterns shows up.
func TestStructuredAreasAgainstTheText(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	var got []string
	for _, k := range sortedKeys(c.c.spells) {
		s := c.c.spells[k]
		if s.AreaType != "" && !textArea(s) {
			got = append(got, k)
		}
		if s.AreaType == "" && textArea(s) && !srdTarget(s).AnyNumber() {
			t.Errorf("%s: the text says area but the target is %+v", k, srdTarget(s))
		}
	}
	if !slices.Equal(got, structuredOnly) {
		t.Errorf("the structured area and the text disagree on %v, want %v", got, structuredOnly)
	}
}

// TestSpellTargetMaxTargets: how many creatures a spell takes at a slot level.
func TestSpellTargetMaxTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		target       SpellTarget
		slot, level  int
		want         int
		wantAny      bool
		wantIsAnArea bool
	}{
		{"one creature", SpellTarget{Kind: TargetCreature}, 3, 1, 1, false, false},
		{"one and one more per circle", SpellTarget{Kind: TargetCreature, PerSlotLevel: 1}, 4, 2, 3, false, false},
		{"several at the spell's circle", SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 2}, 1, 1, 3, false, false},
		{"several, two more per circle", SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 2}, 3, 1, 7, false, false},
		{"a cantrip", SpellTarget{Kind: TargetCreatures, Count: 2}, 0, 0, 2, false, false},
		{"as many as the text says", SpellTarget{Kind: TargetCreatures}, 2, 1, 0, true, false},
		{"an area", SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15}, 1, 1, 0, true, true},
		{"the caster", SpellTarget{Kind: TargetSelf}, 1, 1, 0, false, false},
	} {
		if got := tc.target.MaxTargets(tc.slot, tc.level); got != tc.want {
			t.Errorf("%s: MaxTargets(%d, %d) = %d, want %d", tc.name, tc.slot, tc.level, got, tc.want)
		}
		if tc.target.AnyNumber() != tc.wantAny || tc.target.IsArea() != tc.wantIsAnArea {
			t.Errorf("%s: AnyNumber = %v, IsArea = %v", tc.name, tc.target.AnyNumber(), tc.target.IsArea())
		}
	}
}

// TestMetersPT: feet in the table's meters (5 ft = 1,5 m), with the comma.
func TestMetersPT(t *testing.T) {
	t.Parallel()
	for feet, want := range map[int]string{
		5: "1,5 m", 10: "3 m", 15: "4,5 m", 20: "6 m", 25: "7,5 m", 60: "18 m", 100: "30 m", 150: "45 m", 300: "90 m",
		5280: "1,6 km", 40000: "12 km", 2500: "750 m",
	} {
		if got := MetersPT(feet); got != want {
			t.Errorf("MetersPT(%d) = %q, want %q", feet, got, want)
		}
	}
}

// TestSpellTargetLabels: the text of each kind, with every shape.
func TestSpellTargetLabels(t *testing.T) {
	t.Parallel()
	for target, want := range map[SpellTarget]string{
		{Kind: TargetCreature}:                               "Uma criatura",
		{Kind: TargetCreatures, Count: 2}:                    "Várias criaturas",
		{Kind: TargetSelf}:                                   "Só quem conjura",
		{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15}:     "Cone de 4,5 m",
		{Kind: TargetArea, Shape: ShapeCube, SizeFt: 10}:     "Cubo de 3 m",
		{Kind: TargetArea, Shape: ShapeCylinder, SizeFt: 20}: "Cilindro de 6 m",
		{Kind: TargetArea, Shape: ShapeLine, SizeFt: 30}:     "Linha de 9 m",
		{Kind: TargetArea, Shape: ShapeSphere, SizeFt: 5}:    "Esfera de 1,5 m",
		{}: "",
	} {
		if got := target.LabelPT(); got != want {
			t.Errorf("%+v label = %q, want %q", target, got, want)
		}
	}
}

// TestTableSpellTargetsAreTheMasters: a table spell keeps the target the master wrote,
// and the range Pessoal and Toque are accepted (E10-01 state 4b).
func TestTableSpellTargetsAreTheMasters(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	c := withOverlayOn(t, base, fullOverlay(t, base))
	for key, want := range map[string]struct {
		target SpellTarget
		label  string
		rng    string
	}{
		"spell:cone-de-teste@mesa": {SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15}, "Cone de 4,5 m", RangeSelf},
		"spell:par-de-teste@mesa":  {SpellTarget{Kind: TargetCreatures, Count: 2, PerSlotLevel: 1}, "Várias criaturas", RangeRanged},
		"spell:cura-de-teste@mesa": {SpellTarget{Kind: TargetCreature}, "Uma criatura", RangeTouch},
	} {
		d, ok := c.SpellDetails(key)
		if !ok {
			t.Fatalf("%s is not in the content", key)
		}
		if d.Target != want.target || d.Target.LabelPT() != want.label || d.Range.Kind != want.rng {
			t.Errorf("%s = %+v %q range %s, want %+v %q range %s", key, d.Target, d.Target.LabelPT(), d.Range.Kind, want.target, want.label, want.rng)
		}
	}

	// A spell that only affects the caster: Pessoal, and nothing else.
	self := tableTestSpells()[0]
	self.Key, self.NamePT = "spell:vigia@mesa", "Vigia"
	self.Attack, self.Damage = "", nil
	self.Range, self.Target = TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetSelf}
	c = withOverlayOn(t, base, Overlay{Revision: 1, Spells: []TableSpell{self}})
	if d, _ := c.SpellDetails("spell:vigia@mesa"); d.Target.LabelPT() != "Só quem conjura" || d.Range.Kind != RangeSelf || d.Range.Raw != "Self" {
		t.Errorf("a self spell = %+v range %+v", d.Target, d.Range)
	}
}

// TestTableSpellRangeAndTargetMustAgree: Pessoal reaches the caster or an area that
// comes out of the caster, never a creature picked.
func TestTableSpellRangeAndTargetMustAgree(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	for name, tc := range map[string]struct {
		rng    TableRange
		target SpellTarget
		ok     bool
	}{
		"self and one creature":   {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetCreature}, false},
		"self and creatures":      {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetCreatures, Count: 2}, false},
		"self and an area":        {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetArea, Shape: ShapeCube, SizeFt: 10}, true},
		"self and the caster":     {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetSelf}, true},
		"touch and one creature":  {TableRange{Kind: RangeTouch}, SpellTarget{Kind: TargetCreature}, true},
		"touch and the caster":    {TableRange{Kind: RangeTouch}, SpellTarget{Kind: TargetSelf}, false},
		"a distance and the area": {TableRange{Kind: RangeRanged, DistanceFt: 60}, SpellTarget{Kind: TargetArea, Shape: ShapeSphere, SizeFt: 20}, true},
	} {
		s := tableTestSpells()[3] // the heal, a touch spell
		s.Attack, s.Heal = "", nil
		s.Range, s.Target = tc.rng, tc.target
		_, err := base.With(Overlay{Revision: 1, Spells: []TableSpell{s}})
		if (err == nil) != tc.ok {
			t.Errorf("%s: error = %v, want ok = %v", name, err, tc.ok)
		}
	}
}
