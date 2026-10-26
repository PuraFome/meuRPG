package rules

import (
	"errors"
	"slices"
	"testing"
)

// A table subclass's spells are always prepared (a domain's) or, when the
// subclass marks them expanded (SpellsExpanded), they only join the class's list
// to choose from, as the SRD's Fiend patron does.

const expandedSub = "subclass:patrono" + tableSuffix

// expandedContent is a class that knows its spells (the sorcerer's list) with a
// subclass whose spells are Bless (class level 3) and Spiritual Weapon (5), two
// spells that are not on that list.
func expandedContent(t *testing.T, expanded bool) (*Content, string) {
	t.Helper()
	base := loadForTest(t)
	tc := genClass(base, genKinds[2])
	sub := TableSubclass{
		Key: expandedSub, NamePT: "Patrono", Class: tc.Key,
		AlwaysPrepared: []TableAlwaysPrepared{{ClassLevel: 3, Spell: "spell:bless"}, {ClassLevel: 5, Spell: "spell:spiritual-weapon"}},
		SpellsExpanded: expanded,
	}
	return withOverlayOn(t, base, Overlay{Revision: 1, Classes: []TableClass{tc}, Subclasses: []TableSubclass{sub}}), tc.Key
}

func expandedBuild(class string, level int) Build {
	b := standard(class, level)
	b.Classes[0].Subclass = expandedSub
	return b
}

func sheetHas(d Derived, key string) (CharacterSpell, bool) {
	for _, s := range d.Spells {
		if s.Spell.Key == key {
			return s, true
		}
	}
	return CharacterSpell{}, false
}

func TestSubclassSpellsAreGivenUnlessExpanded(t *testing.T) {
	t.Parallel()
	for _, expanded := range []bool{false, true} {
		c, class := expandedContent(t, expanded)
		d := Derive(expandedBuild(class, 5), c)
		for _, key := range []string{"spell:bless", "spell:spiritual-weapon"} {
			s, on := sheetHas(d, key)
			if on != !expanded || (on && !s.Prepared) {
				t.Errorf("expanded=%v: %s on the sheet = %v (prepared %v), want it given only when not expanded", expanded, key, on, s.Prepared)
			}
		}
		entry := slices.IndexFunc(c.Catalog().Subclasses, func(s SubclassEntry) bool { return s.Key == expandedSub })
		if got := len(c.Catalog().Subclasses[entry].AlwaysPrepared); got != map[bool]int{false: 2, true: 0}[expanded] {
			t.Errorf("expanded=%v: the catalog lists %d always-prepared spells", expanded, got)
		}
	}
}

func TestExpandedSubclassSpellsAreOnTheListToChooseFrom(t *testing.T) {
	t.Parallel()
	c, class := expandedContent(t, true)
	b := expandedBuild(class, 5)
	b.Cantrips = nil
	b.SpellsKnown = []string{"spell:bless", "spell:spiritual-weapon"}
	for _, i := range Derive(b, c).Issues {
		if i.Code == IssueSpellNotOnList {
			t.Errorf("an expanded spell is refused: %+v", i)
		}
	}
	// They are known, so they count against the spells known, and they are castable.
	if s, on := sheetHas(Derive(b, c), "spell:bless"); !on || !s.Prepared {
		t.Errorf("a chosen expanded spell is missing or not castable: %+v %v", s, on)
	}
	// Without the subclass they are not on the class's list.
	other := standard(class, 5)
	other.SpellsKnown = []string{"spell:bless"}
	if !slices.ContainsFunc(Derive(other, c).Issues, func(i Issue) bool { return i.Code == IssueSpellNotOnList }) {
		t.Error("Bless is on the class's list without the subclass")
	}
	// A spell of neither list is still refused.
	b.SpellsKnown = []string{"spell:cure-wounds"}
	if !slices.ContainsFunc(Derive(b, c).Issues, func(i Issue) bool { return i.Code == IssueSpellNotOnList }) {
		t.Error("Cure Wounds is accepted with the subclass")
	}

	// The picker's source: the patron spells up to the class level.
	at := func(level int) []string {
		var out []string
		for _, src := range c.SpellSources(expandedBuild(class, level)) {
			for _, p := range src.Patron {
				out = append(out, p.Spell)
			}
		}
		return out
	}
	if got := at(3); !slices.Equal(got, []string{"spell:bless"}) {
		t.Errorf("patron spells at class level 3 = %v, want Bless", got)
	}
	if got := at(5); !slices.Equal(got, []string{"spell:bless", "spell:spiritual-weapon"}) {
		t.Errorf("patron spells at class level 5 = %v, want both", got)
	}
	cNot, _ := expandedContent(t, false)
	if got := cNot.SpellSources(expandedBuild(class, 5)); len(got) != 0 {
		t.Errorf("sources of a subclass whose spells are given = %v, want none", got)
	}
}

func TestExpandedSubclassSpellsAtLevelUp(t *testing.T) {
	t.Parallel()
	for _, expanded := range []bool{false, true} {
		c, class := expandedContent(t, expanded)
		b := sweepUp(t, c, sweepBase(t, c, class, ""), class, expandedSub, 3)
		after, err := ApplyLevelUp(b, satisfy(t, c, b, class, expandedSub), c)
		if err != nil {
			t.Fatalf("expanded=%v: ApplyLevelUp: %v", expanded, err)
		}
		i := slices.IndexFunc(after.SpellsKnown, func(k string) bool { return !slices.Contains(b.SpellsKnown, k) })
		if i < 0 {
			t.Fatalf("expanded=%v: the level gives no new spell known", expanded)
		}
		// Bless is on neither the class's list nor, unless expanded, the subclass's list to choose from.
		after.SpellsKnown[i] = "spell:bless"
		err = CheckLevelUp(b, after, c)
		if expanded && err != nil {
			t.Errorf("an expanded subclass spell is refused as a new spell known: %v", err)
		}
		if !expanded && err == nil {
			t.Error("a spell the subclass gives (not expanded) is accepted as a spell learned off the list")
		}
	}
}

func TestExpandedFlagIsChecked(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	tc := genClass(base, genKinds[2])
	cases := []struct {
		name          string
		sub           TableSubclass
		field, reason string
	}{
		{"no spells", TableSubclass{Key: expandedSub, NamePT: "Patrono", Class: tc.Key, SpellsExpanded: true}, "subclasses[0].spells_expanded", ReasonValue},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := base.With(Overlay{Revision: 1, Classes: []TableClass{tc}, Subclasses: []TableSubclass{tt.sub}, Strict: []string{tt.sub.Key}})
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tt.field || oe.Reason != tt.reason {
				t.Errorf("err = %v, want %s (%s)", err, tt.field, tt.reason)
			}
		})
	}
}
