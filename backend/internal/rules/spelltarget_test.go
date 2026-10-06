package rules

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
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
		// The database has the 40 ft of its height: ours says the 3 m of its radius.
		"spell:flame-strike": {SpellTarget{Kind: TargetArea, Shape: ShapeCylinder, SizeFt: 10, Label: "Cilindro de 3 m de raio"}, "Cilindro de 3 m de raio"},
		"spell:cure-wounds":  {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		"spell:fire-bolt":    {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		"spell:shield":       {SpellTarget{Kind: TargetSelf}, "Só quem conjura"},
		"spell:mage-armor":   {SpellTarget{Kind: TargetCreature}, "Uma criatura"},
		// The text says "an additional creature" for each circle above.
		"spell:hold-person": {SpellTarget{Kind: TargetCreature, PerSlotLevel: 1}, "Uma criatura"},
		// "Up to three creatures", and one more for each circle: the number is ours.
		"spell:bless": {SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 1}, "Várias criaturas"},
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

// TestEverySRDSpellHasATarget: no spell of the SRD is left without one, and a spell the
// database gives an area for is an area unless the hand-written file says otherwise.
func TestEverySRDSpellHasATarget(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	areas := 0
	for _, e := range c.Catalog().Spells {
		d, _ := c.SpellDetails(e.Key)
		if d.Target.Kind == "" || d.Target.LabelPT() == "" {
			t.Errorf("%s has no target: %+v", e.Key, d.Target)
		}
		_, overridden := c.c.srdTargets[e.Key]
		if sp := c.c.spells[e.Key]; !overridden && (sp.AreaType != "") != d.Target.IsArea() {
			t.Errorf("%s: the database area %q and the target %+v disagree", e.Key, sp.AreaType, d.Target)
		}
		if d.Target.IsArea() {
			areas++
		}
	}
	// 88 in the database, 7 of them not areas for us, and 4 that the database lacks.
	if areas != 85 {
		t.Errorf("%d SRD spells are areas, want 85", areas)
	}
}

// structuredOnly are the SRD spells where the database's area_of_effect and the old
// reading of the text disagree and no override says otherwise: the text did not call
// them an area (a wall, a point), so they used to take one target. The structured data
// wins; the text stays the fallback for the others.
var structuredOnly = []string{
	"spell:blade-barrier", "spell:control-water", "spell:forcecage", "spell:guardian-of-faith", "spell:hallow",
	"spell:prismatic-wall", "spell:private-sanctum", "spell:storm-of-vengeance", "spell:wall-of-fire", "spell:wall-of-thorns",
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
		if _, overridden := c.c.srdTargets[k]; overridden {
			continue
		}
		if s.AreaType != "" && !textArea(s) {
			got = append(got, k)
		}
		if s.AreaType == "" && textArea(s) && !c.c.srdTarget(s).AnyNumber() {
			t.Errorf("%s: the text says area but the target is %+v", k, c.c.srdTarget(s))
		}
	}
	if !slices.Equal(got, structuredOnly) {
		t.Errorf("the structured area and the text disagree on %v, want %v", got, structuredOnly)
	}
}

// TestSpellTargetOverrides pins effects/spell_targets.json: the hand-written targets
// of the spells the structured area or the text gets wrong.
func TestSpellTargetOverrides(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	kindOf := map[string]string{}
	for key, tg := range c.c.srdTargets {
		kindOf[key] = tg.Kind
	}
	want := func(kind string, keys ...string) {
		t.Helper()
		for _, k := range keys {
			if kindOf[k] != kind {
				t.Errorf("%s is %q, want %q", k, kindOf[k], kind)
			}
			delete(kindOf, k)
		}
	}
	want(TargetCreature, "spell:telekinesis")
	want(TargetSelf, "spell:detect-evil-and-good", "spell:detect-magic", "spell:detect-poison-and-disease", "spell:speak-with-plants",
		"spell:branding-smite", "spell:divine-favor", "spell:flame-blade", "spell:contact-other-plane", "spell:magic-jar", "spell:scrying", "spell:wish")
	want(TargetNone, "spell:arcane-eye", "spell:magnificent-mansion", "spell:dancing-lights", "spell:druidcraft", "spell:fabricate",
		"spell:light", "spell:major-image", "spell:minor-illusion")
	want(TargetArea, "spell:flame-strike", "spell:forbiddance", "spell:mirage-arcane", "spell:teleportation-circle", "spell:guards-and-wards",
		"spell:fire-storm", "spell:hypnotic-pattern", "spell:plant-growth", "spell:purify-food-and-drink", "spell:glyph-of-warding")
	want(TargetCreatures, "spell:bless", "spell:bane", "spell:mass-healing-word", "spell:prayer-of-healing", "spell:mass-suggestion",
		"spell:telepathic-bond", "spell:water-walk", "spell:wind-walk", "spell:astral-projection", "spell:heroes-feast")
	if len(kindOf) != 0 {
		t.Errorf("entries nobody pinned: %v", kindOf)
	}
	for key, count := range map[string]int{
		"spell:bless": 3, "spell:bane": 3, "spell:mass-healing-word": 6, "spell:prayer-of-healing": 6, "spell:mass-suggestion": 12,
		"spell:telepathic-bond": 8, "spell:water-walk": 10, "spell:wind-walk": 11, "spell:astral-projection": 9, "spell:heroes-feast": 12,
	} {
		if got := c.c.srdTargets[key].Count; got != count {
			t.Errorf("%s takes %d, want %d", key, got, count)
		}
	}
	if tg := c.c.srdTargets["spell:bless"]; tg.PerSlotLevel != 1 || c.c.srdTargets["spell:bane"].PerSlotLevel != 1 {
		t.Errorf("Bless and Bane take one more for each circle: %+v", tg)
	}
	// What stays as the text says: any number, and the summons untouched.
	for _, key := range []string{"spell:compulsion", "spell:enthrall", "spell:mass-heal", "spell:dispel-evil-and-good", "spell:eyebite", "spell:animate-dead", "spell:create-undead"} {
		if d, _ := c.SpellDetails(key); !d.Target.AnyNumber() {
			t.Errorf("%s = %+v, want any number of creatures", key, d.Target)
		}
	}
	// Nobody to pick for these, and a label of our own where the database's area misleads.
	for key, label := range map[string]string{
		"spell:detect-magic": "Só quem conjura", "spell:arcane-eye": "Nenhuma criatura", "spell:light": "Nenhuma criatura",
		"spell:forbiddance": "Uma área de cerca de 3.700 m²", "spell:teleportation-circle": "Círculo de 3 m de diâmetro",
		"spell:mirage-arcane": "Terreno de até 1,6 km de lado", "spell:wind-walk": "Você e até 10 criaturas",
		"spell:hypnotic-pattern": "Cubo de 9 m", "spell:plant-growth": "Esfera de 30 m",
	} {
		d, _ := c.SpellDetails(key)
		if d.Target.LabelPT() != label {
			t.Errorf("%s label = %q, want %q", key, d.Target.LabelPT(), label)
		}
	}
	if d, _ := c.SpellDetails("spell:detect-magic"); !d.Target.CasterOnly() {
		t.Error("Detect Magic has somebody to pick")
	}
}

// TestSpellTargetOverridesAreChecked: the loader refuses what is not an SRD spell, an
// unknown kind and a bad area.
func TestSpellTargetOverridesAreChecked(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	load := func(body string) error {
		return c.c.loadSpellTargets(fstest.MapFS{"effects/spell_targets.json": {Data: []byte(body)}})
	}
	if err := load(`{"spells":{"spell:bless":{"kind":"creatures","count":3}}}`); err != nil {
		t.Fatalf("a good entry: %v", err)
	}
	for name, body := range map[string]string{
		"not an SRD spell":      `{"spells":{"spell:raio@mesa":{"kind":"self"}}}`,
		"an unknown kind":       `{"spells":{"spell:bless":{"kind":"party"}}}`,
		"a self with a count":   `{"spells":{"spell:bless":{"kind":"self","count":2}}}`,
		"creatures of none":     `{"spells":{"spell:bless":{"kind":"creatures"}}}`,
		"an area off the grid":  `{"spells":{"spell:bless":{"kind":"area","shape":"cone","size_ft":12}}}`,
		"an area with no shape": `{"spells":{"spell:bless":{"kind":"area","size_ft":10}}}`,
		"a long label":          `{"spells":{"spell:bless":{"kind":"self","label_pt":"` + strings.Repeat("a", 81) + `"}}}`,
		"an unknown field":      `{"spells":{"spell:bless":{"kind":"self","range":3}}}`,
	} {
		if err := load(body); err == nil {
			t.Errorf("%s: accepted", name)
		}
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
		"self and one creature":         {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetCreature}, false},
		"self and creatures":            {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetCreatures, Count: 2}, false},
		"one creature, more per circle": {TableRange{Kind: RangeRanged, DistanceFt: 60}, SpellTarget{Kind: TargetCreature, PerSlotLevel: 1}, true},
		"one creature, a bad count":     {TableRange{Kind: RangeRanged, DistanceFt: 60}, SpellTarget{Kind: TargetCreature, PerSlotLevel: 11}, false},
		"a table spell may not be none": {TableRange{Kind: RangeRanged, DistanceFt: 60}, SpellTarget{Kind: TargetNone}, false},
		"self and an area":              {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetArea, Shape: ShapeCube, SizeFt: 10}, true},
		"self and the caster":           {TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetSelf}, true},
		"touch and one creature":        {TableRange{Kind: RangeTouch}, SpellTarget{Kind: TargetCreature}, true},
		"touch and the caster":          {TableRange{Kind: RangeTouch}, SpellTarget{Kind: TargetSelf}, false},
		"a distance and the area":       {TableRange{Kind: RangeRanged, DistanceFt: 60}, SpellTarget{Kind: TargetArea, Shape: ShapeSphere, SizeFt: 20}, true},
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
