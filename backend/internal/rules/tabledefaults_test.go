package rules

import (
	"slices"
	"strings"
	"testing"
)

// defaultTable finds the default table of a kind and preparation.
func defaultTable(t *testing.T, d TableDefaults, kind, preparation string) DefaultTable {
	t.Helper()
	for _, tab := range d.Tables {
		if tab.Kind == kind && tab.Preparation == preparation {
			return tab
		}
	}
	t.Fatalf("no default table for %q %q", kind, preparation)
	return DefaultTable{}
}

// TestTableDefaultsFollowTheSRDTables: the numbers the class editor starts from
// are the SRD's class tables, read from the content: the proficiency bonus, the
// Ability Score Improvement levels, and each casting kind's slots.
func TestTableDefaultsFollowTheSRDTables(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d := c.TableDefaults()

	if len(d.ProfBonus) != MaxLevel || d.SubclassLevel != 3 {
		t.Fatalf("profs = %v, subclass level = %d", d.ProfBonus, d.SubclassLevel)
	}
	for i, rows := range c.c.classLevels["class:wizard"] {
		if d.ProfBonus[i] != rows.ProfBonus {
			t.Errorf("proficiency bonus at level %d = %d, SRD has %d", i+1, d.ProfBonus[i], rows.ProfBonus)
		}
	}
	// The ASI levels are where the SRD's classes with the common table have them.
	var srdASI []int
	for _, row := range c.c.classLevels["class:wizard"] {
		for _, fk := range row.Features {
			if strings.Contains(fk, "-ability-score-improvement-") {
				srdASI = append(srdASI, row.Level)
			}
		}
	}
	if !slices.Equal(d.ASILevels, srdASI) {
		t.Errorf("ASI levels = %v, SRD wizard has %v", d.ASILevels, srdASI)
	}

	// The kinds the editor offers, in order.
	var kinds []string
	for _, tab := range d.Tables {
		kinds = append(kinds, tab.Kind+"/"+tab.Preparation)
	}
	want := []string{"/", "full/prepared", "full/known", "half/prepared", "half/known", "pact/known", "pact/prepared", "third/known", "third/prepared"}
	if !slices.Equal(kinds, want) {
		t.Fatalf("tables = %v, want %v", kinds, want)
	}

	for _, tab := range d.Tables {
		if len(tab.Rows) != MaxLevel {
			t.Fatalf("%s/%s has %d rows", tab.Kind, tab.Preparation, len(tab.Rows))
		}
		for i, r := range tab.Rows {
			if r.Level != i+1 || r.ProfBonus != d.ProfBonus[i] {
				t.Errorf("%s/%s row %d = level %d bonus %d", tab.Kind, tab.Preparation, i+1, r.Level, r.ProfBonus)
			}
			if tab.Kind != CastingNone && r.Level < tab.StartLevel && (r.CantripsKnown != 0 || r.SpellsKnown != 0 || r.Slots != [9]int{}) {
				t.Errorf("%s/%s level %d: casting columns before the start level %d", tab.Kind, tab.Preparation, r.Level, tab.StartLevel)
			}
			if tab.Kind == CastingNone && (r.CantripsKnown != 0 || r.SpellsKnown != 0 || r.Slots != [9]int{}) {
				t.Errorf("a class that does not cast has casting columns at level %d", r.Level)
			}
			if tab.Preparation == PreparationPrepared && r.SpellsKnown != 0 {
				t.Errorf("%s/%s level %d: a class that prepares has no spells known", tab.Kind, tab.Preparation, r.Level)
			}
			if tab.Kind != CastingNone && r.Level >= tab.StartLevel {
				if r.Slots == [9]int{} {
					t.Errorf("%s/%s level %d: no slots", tab.Kind, tab.Preparation, r.Level)
				}
				if tab.Preparation == PreparationKnown && r.SpellsKnown == 0 {
					t.Errorf("%s/%s level %d: a class that knows its spells needs some", tab.Kind, tab.Preparation, r.Level)
				}
			}
		}
	}

	slotsOf := func(class string, level int) [9]int {
		return c.c.classLevels[class][level-1].Spellcasting.Slots
	}
	for _, tc := range []struct {
		kind, prep, class string
		from              int
	}{
		{CastingFull, PreparationPrepared, "class:wizard", 1},
		{CastingFull, PreparationKnown, "class:wizard", 1},
		{CastingHalf, PreparationPrepared, "class:paladin", 2},
		{CastingHalf, PreparationKnown, "class:paladin", 2},
		{CastingPact, PreparationKnown, "class:warlock", 1},
		{CastingPact, PreparationPrepared, "class:warlock", 1},
	} {
		tab := defaultTable(t, d, tc.kind, tc.prep)
		for _, r := range tab.Rows[tc.from-1:] {
			if got := slotsOf(tc.class, r.Level); r.Slots != got {
				t.Errorf("%s/%s level %d: slots %v, SRD %s has %v", tc.kind, tc.prep, r.Level, r.Slots, tc.class, got)
			}
		}
	}
	// The SRD's own spells-known and cantrip columns for the kinds that know them.
	for level, want := range map[int][2]int{1: {4, 2}, 10: {6, 11}, 20: {6, 15}} {
		r := defaultTable(t, d, CastingFull, PreparationKnown).Rows[level-1]
		if r.CantripsKnown != want[0] || r.SpellsKnown != want[1] {
			t.Errorf("full/known level %d: cantrips %d, spells %d, SRD sorcerer has %v", level, r.CantripsKnown, r.SpellsKnown, want)
		}
	}
	// Pact magic: one level of slots, all of it (the Warlock's: 1 slot at level 1, 2 from level 2, 3 from 11, 4 from 17).
	pact := defaultTable(t, d, CastingPact, PreparationKnown)
	for level, want := range map[int][2]int{1: {1, 1}, 2: {2, 1}, 5: {2, 3}, 11: {3, 5}, 17: {4, 5}} { // {slots, their circle}
		for i, n := range pact.Rows[level-1].Slots {
			switch {
			case n != 0 && (n != want[0] || i+1 != want[1]):
				t.Errorf("pact level %d: %d slots of circle %d, want %d of %d", level, n, i+1, want[0], want[1])
			case n == 0 && i+1 == want[1]:
				t.Errorf("pact level %d: no slots of circle %d", level, want[1])
			}
		}
	}
	// A third caster: the full table at a third of the level, rounded up.
	third := defaultTable(t, d, CastingThird, PreparationKnown)
	for _, r := range third.Rows[2:] {
		if want := slotsOf("class:wizard", (r.Level+2)/3); r.Slots != want {
			t.Errorf("third level %d: slots %v, want the full table at %d: %v", r.Level, r.Slots, (r.Level+2)/3, want)
		}
	}
	if r := third.Rows[2]; r.Slots != [9]int{2} || r.CantripsKnown != 2 || r.SpellsKnown != 3 {
		t.Errorf("third level 3 = %+v, want 2 slots of the 1st circle, 2 cantrips and 3 spells", r)
	}
	if r := third.Rows[6]; r.Slots != [9]int{4, 2} {
		t.Errorf("third level 7 = %v, want 4 and 2 slots", r.Slots)
	}
	if r := third.Rows[19]; r.Slots != [9]int{4, 3, 3, 1} || r.CantripsKnown != 3 {
		t.Errorf("third level 20 = %+v, want 4/3/3/1 and 3 cantrips", r)
	}
}

// classFromDefaults is a table class whose table is a default one, with the
// features of every effect of the menu (genFeatures).
func classFromDefaults(tab DefaultTable, slug string) TableClass {
	tc := TableClass{
		Key: "class:" + slug + tableSuffix, NamePT: "Padrão " + slug,
		HitDie:        8,
		SavingThrows:  []Ability{CON, WIS},
		SkillChoose:   2,
		SkillFrom:     []string{"skill:arcana", "skill:history", "skill:medicine", "skill:nature", "skill:perception", "skill:survival"},
		Proficiencies: []string{"proficiency:light-armor", "proficiency:simple-weapons"},
		SubclassLevel: 3,
		Levels:        make([]TableClassLevel, MaxLevel),
	}
	if tab.Kind != CastingNone {
		tc.Casting = TableCasting{Kind: tab.Kind, Ability: WIS, Preparation: tab.Preparation, ListFrom: "class:cleric"}
		if tab.Preparation == PreparationKnown {
			tc.Casting.ListFrom = "class:sorcerer"
		}
	}
	for i, r := range tab.Rows {
		tc.Levels[i] = TableClassLevel{ProfBonus: r.ProfBonus, Features: genFeatures(slug, r.Level), CantripsKnown: r.CantripsKnown, SpellsKnown: r.SpellsKnown, Slots: r.Slots}
	}
	return tc
}

// TestTableDefaultsMakeValidClasses: a class whose table is, untouched, each
// default is accepted by With and goes from level 1 to 20 with every level-up
// satisfying what LevelUpOptions asks for (the sweep of ADR-0018, section 12),
// and a third caster's default table is a valid subclass of the SRD Fighter. The
// editor can therefore save what it starts from.
func TestTableDefaultsMakeValidClasses(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	d := srd.TableDefaults()
	var o Overlay
	var swept [][2]string
	for _, tab := range d.Tables {
		if tab.Kind == CastingThird {
			continue
		}
		slug := "padrao-" + tab.Kind + "-" + tab.Preparation
		tc := classFromDefaults(tab, slug)
		sub := TableSubclass{
			Key: "subclass:" + slug + "-a" + tableSuffix, NamePT: "Caminho", Class: tc.Key,
			Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{tf(slug+"-a3", "A3")}}},
		}
		o.Classes, o.Subclasses = append(o.Classes, tc), append(o.Subclasses, sub)
		swept = append(swept, [2]string{tc.Key, sub.Key})
	}
	for _, tab := range d.Tables {
		if tab.Kind != CastingThird {
			continue
		}
		slug := "terco-" + tab.Preparation
		sub := TableSubclass{
			Key: "subclass:" + slug + tableSuffix, NamePT: "Terço " + tab.Preparation, Class: "class:fighter",
			Casting: &TableCasting{Kind: CastingThird, Ability: INT, Preparation: tab.Preparation, ListFrom: "class:wizard", StartLevel: tab.StartLevel},
		}
		for _, r := range tab.Rows[tab.StartLevel-1:] {
			sub.Levels = append(sub.Levels, TableSubclassLevel{Level: r.Level, CantripsKnown: r.CantripsKnown, SpellsKnown: r.SpellsKnown, Slots: r.Slots})
		}
		o.Subclasses = append(o.Subclasses, sub)
		swept = append(swept, [2]string{"class:fighter", sub.Key})
	}
	c := withOverlayOn(t, srd, o)
	if len(swept) != 9 {
		t.Fatalf("swept %d, want 9", len(swept))
	}
	for _, s := range swept {
		t.Run(s[1], func(t *testing.T) {
			t.Parallel()
			b := sweepBase(t, c, s[0], s[1])
			b = sweepUp(t, c, b, s[0], s[1], MaxLevel)
			if got := Derive(b, c).TotalLevel; got != MaxLevel {
				t.Errorf("total level = %d", got)
			}
		})
	}
}
