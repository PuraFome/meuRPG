package rules

import (
	"errors"
	"slices"
	"strconv"
	"testing"
)

// A table class's own spell list can name SRD spells (casting.srd_spell_keys):
// they join the table spells that name the class. An Artificer-like class is the
// case: a half caster that prepares and whose list is made of SRD spells.

// artificerSpells are the SRD spells on the list of the Artificer-like class of
// these tests: cantrips and spells of the 1st to the 3rd circle.
var artificerSpells = []string{
	"spell:guidance", "spell:mending", "spell:light", "spell:resistance",
	"spell:cure-wounds", "spell:healing-word", "spell:detect-magic", "spell:shield-of-faith",
	"spell:aid", "spell:lesser-restoration", "spell:protection-from-poison", "spell:spiritual-weapon",
	"spell:revivify", "spell:dispel-magic", "spell:remove-curse", "spell:beacon-of-hope",
}

// artificerClass is a full caster that prepares, with a list of its own made of
// SRD spells only.
func artificerClass(base *Content) TableClass {
	gk := genKinds[1]
	gk.casting.ListFrom = ""
	tc := genClass(base, gk)
	tc.Key = "class:artifice" + tableSuffix
	tc.Casting.SRDSpells = slices.Clone(artificerSpells)
	return tc
}

// catalogSpell is the catalog entry of a spell.
func catalogSpell(c *Content, key string) (SpellEntry, bool) {
	for _, e := range c.Catalog().Spells {
		if e.Key == key {
			return e, true
		}
	}
	return SpellEntry{}, false
}

func artificerContent(t *testing.T) (*Content, string) {
	t.Helper()
	base := loadForTest(t)
	tc := artificerClass(base)
	return withOverlayOn(t, base, Overlay{Revision: 1, Classes: []TableClass{tc}}), tc.Key
}

func TestTableClassListNamesSRDSpells(t *testing.T) {
	t.Parallel()
	c, class := artificerContent(t)

	// The Magias page lists them for the class, and only them.
	got := spellKeys(c.ListSpells(SpellFilter{Class: class}))
	slices.Sort(got)
	want := slices.Clone(artificerSpells)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the class's list = %v, want %v", got, want)
	}
	for _, key := range []string{"spell:cure-wounds", "spell:aid", "spell:revivify", "spell:guidance"} {
		e, ok := catalogSpell(c, key)
		if !ok || !slices.Contains(e.Classes, class) {
			t.Errorf("%s: the catalog entry does not name the class: %+v", key, e)
		}
	}
	// The SRD's own lists do not change.
	if e, _ := catalogSpell(c, "spell:cure-wounds"); !slices.Contains(e.Classes, "class:cleric") {
		t.Errorf("Cure Wounds lost the cleric: %v", e.Classes)
	}
	if e, _ := catalogSpell(c, "spell:fireball"); slices.Contains(e.Classes, class) {
		t.Error("Fireball is on the class's list")
	}
	// The SRD content the overlay was built on is not written.
	base := loadForTest(t)
	if e, _ := catalogSpell(base, "spell:cure-wounds"); slices.Contains(e.Classes, class) {
		t.Error("the base content names the table class")
	}
}

func TestTableClassSRDSpellsAreOnTheSheet(t *testing.T) {
	t.Parallel()
	c, class := artificerContent(t)
	b := standard(class, 5)
	b.Cantrips = []string{"spell:guidance", "spell:mending", "spell:light"}
	b.SpellsPrepared = []string{"spell:cure-wounds", "spell:aid", "spell:revivify"}
	d := Derive(b, c)
	for _, i := range d.Issues {
		if i.Code == IssueSpellNotOnList {
			t.Errorf("a spell on the class's list is refused: %+v", i)
		}
	}
	for _, key := range append(slices.Clone(b.Cantrips), b.SpellsPrepared...) {
		if !slices.ContainsFunc(d.Spells, func(s CharacterSpell) bool { return s.Spell.Key == key }) {
			t.Errorf("%s is not on the sheet", key)
		}
	}

	// A spell outside the list is still refused.
	b.SpellsPrepared = []string{"spell:cure-wounds", "spell:magic-missile"}
	if !slices.ContainsFunc(Derive(b, c).Issues, func(i Issue) bool { return i.Code == IssueSpellNotOnList }) {
		t.Error("Magic Missile is not on the class's list and raised no issue")
	}
}

func TestTableClassSRDSpellsAreOfferedAtLevelUp(t *testing.T) {
	t.Parallel()
	c, class := artificerContent(t)
	b := sweepBase(t, c, class, "")
	o, err := LevelUpOptions(b, class, c)
	if err != nil {
		t.Fatalf("LevelUpOptions: %v", err)
	}
	if o.SpellList != class {
		t.Errorf("the offer's spell list = %q, want the class", o.SpellList)
	}
	// The sweep picks the spells from the class's list, level by level.
	sweepUp(t, c, b, class, "", 9)

	// A spell of another class's list is refused when the sheet levels up.
	ch := satisfy(t, c, b, class, "")
	after, err := ApplyLevelUp(b, ch, c)
	if err != nil {
		t.Fatalf("ApplyLevelUp: %v", err)
	}
	after.Cantrips = append(slices.Clone(after.Cantrips), "spell:fire-bolt")
	if err := CheckLevelUp(b, after, c); err == nil {
		t.Error("a wizard cantrip was accepted on the class's list")
	}
}

// srdSpellsRefusals are the entries casting.srd_spell_keys refuses, with the
// field and the reason.
func TestTableClassSRDSpellsAreChecked(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name          string
		edit          func(tc *TableClass)
		field, reason string
	}{
		{"a spell that is not there", func(tc *TableClass) { tc.Casting.SRDSpells[1] = "spell:fantasma" }, "classes[0].casting.srd_spell_keys[1]", ReasonReference},
		{"a table spell", func(tc *TableClass) { tc.Casting.SRDSpells[2] = "spell:raio-de-teste" + tableSuffix }, "classes[0].casting.srd_spell_keys[2]", ReasonReference},
		{"a key that is not a spell", func(tc *TableClass) { tc.Casting.SRDSpells[0] = "class:wizard" }, "classes[0].casting.srd_spell_keys[0]", ReasonReference},
		{"a spell twice", func(tc *TableClass) { tc.Casting.SRDSpells[3] = tc.Casting.SRDSpells[0] }, "classes[0].casting.srd_spell_keys[3]", ReasonValue},
		{"a list that is reused", func(tc *TableClass) { tc.Casting.ListFrom = "class:cleric" }, "classes[0].casting.srd_spell_keys", ReasonCasting},
		{"a class that does not cast", func(tc *TableClass) { tc.Casting = TableCasting{SRDSpells: []string{"spell:aid"}} }, "classes[0].casting.srd_spell_keys", ReasonCasting},
		{"over the limit", func(tc *TableClass) {
			tc.Casting.SRDSpells = nil
			for i := range MaxClassSRDSpells + 1 {
				tc.Casting.SRDSpells = append(tc.Casting.SRDSpells, "spell:s"+strconv.Itoa(i))
			}
		}, "classes[0].casting.srd_spell_keys", ReasonLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			class := artificerClass(srd)
			tc.edit(&class)
			_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{class}, Strict: []string{class.Key}})
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("err = %v, want an OverlayError", err)
			}
			if oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("field %q reason %q, want %q %q (%v)", oe.Field, oe.Reason, tc.field, tc.reason, err)
			}
		})
	}

	t.Run("a third caster's subclass", func(t *testing.T) {
		t.Parallel()
		sub := thirdCaster(srd, "terco-lista", "class:fighter", "class:wizard", PreparationKnown)
		sub.Casting.SRDSpells = []string{"spell:aid"}
		_, err := srd.With(Overlay{Revision: 1, Subclasses: []TableSubclass{sub}, Strict: []string{sub.Key}})
		var oe *OverlayError
		if !errors.As(err, &oe) || oe.Field != "subclasses[0].casting.srd_spell_keys" || oe.Reason != ReasonCasting {
			t.Errorf("err = %v, want the casting refused at srd_spell_keys", err)
		}
	})
}
