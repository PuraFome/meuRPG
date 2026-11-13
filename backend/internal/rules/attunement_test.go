package rules

import "testing"

// Every restriction of the catalog has a matcher, and the classes it names exist: the next catalog update that adds a
// restriction this table cannot read fails the load (checkAttunementRules) and this test.
func TestEveryCatalogRestrictionHasAMatcher(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	seen := map[string]bool{}
	for _, m := range c.c.magicItems {
		by := m.AttunementBy
		if by == "" || seen[by] {
			continue
		}
		seen[by] = true
		rule, ok := parseAttunementRule(by)
		if !ok {
			t.Errorf("%q has no matcher", by)
		}
		for _, class := range rule.classes {
			if _, ok := c.c.classes[class]; !ok {
				t.Errorf("%q names the class %s the content lacks", by, class)
			}
		}
	}
	if len(seen) != 13 {
		t.Errorf("%d distinct restrictions in the catalog, want 13: add the new one to the table and to this count", len(seen))
	}
	if _, ok := parseAttunementRule("by a gnome riding a goat"); ok {
		t.Error("a text outside the table parsed")
	}
}

func TestAttunementRestrictionsAreMatchedAgainstTheCharacter(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	cases := []struct {
		name  string
		item  string
		build func(*Build)
		facts AttunementFacts
		want  Restriction
	}{
		{"paladin item, a paladin", "item:holy-avenger", func(b *Build) { b.Classes = []ClassLevel{{Class: "class:paladin", Level: 5}} }, AttunementFacts{}, RestrictionMet},
		{"paladin item, a fighter", "item:holy-avenger", func(*Build) {}, AttunementFacts{}, RestrictionNotMet},
		{"a class among three, a cleric", "item:necklace-of-prayer-beads", func(b *Build) { b.Classes = []ClassLevel{{Class: "class:cleric", Level: 1}} }, AttunementFacts{}, RestrictionMet},
		{"spellcaster item, a wizard", "item:wand-of-web", func(b *Build) { b.Classes = []ClassLevel{{Class: "class:wizard", Level: 1}} }, AttunementFacts{}, RestrictionMet},
		{"spellcaster item, a plain fighter", "item:wand-of-web", func(*Build) {}, AttunementFacts{}, RestrictionNotMet},
		{"dwarf item, a human", "item:dwarven-thrower", func(*Build) {}, AttunementFacts{}, RestrictionNotMet},
		{"dwarf item, a dwarf", "item:dwarven-thrower", func(b *Build) { b.Race = "race:dwarf" }, AttunementFacts{}, RestrictionMet},
		{"good item, a good character", "item:talisman-of-pure-good", func(*Build) {}, AttunementFacts{Alignment: "chaotic_good"}, RestrictionMet},
		{"good item, a neutral one", "item:talisman-of-pure-good", func(*Build) {}, AttunementFacts{Alignment: "neutral"}, RestrictionNotMet},
		{"good item, alignment unset", "item:talisman-of-pure-good", func(*Build) {}, AttunementFacts{}, RestrictionUnknown},
		{"evil item, an evil character", "item:talisman-of-ultimate-evil", func(*Build) {}, AttunementFacts{Alignment: "lawful_evil"}, RestrictionMet},
		{"a circumstance is not checked", "item:ring-of-shooting-stars", func(*Build) {}, AttunementFacts{}, RestrictionMet},
		{"no restriction", "item:ring-of-spell-storing", func(*Build) {}, AttunementFacts{}, RestrictionMet},
	}
	for _, tt := range cases {
		b := standard("class:fighter", 1)
		tt.build(&b)
		d := Derive(b, c)
		requires, _, got, ok := c.AttunementOf(tt.item, b, d, tt.facts)
		if !ok || !requires || got != tt.want {
			t.Errorf("%s: requires %v, met %v, ok %v; want met %v", tt.name, requires, got, ok, tt.want)
		}
	}
	if _, _, _, ok := c.AttunementOf("item:ring-of-nothing", Build{}, Derived{}, AttunementFacts{}); ok {
		t.Error("an unknown key is an item")
	}
	if _, _, _, ok := c.AttunementOf("item:wand-of-the-war-mage", Build{}, Derived{}, AttunementFacts{}); ok {
		t.Error("a family is an item")
	}
}
