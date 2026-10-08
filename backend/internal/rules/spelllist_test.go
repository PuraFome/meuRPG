package rules

import (
	"slices"
	"testing"
)

// The "Magias" page's list (MR-045): the search, the filters and the lists of a table
// class and of a third caster. No database.

func spellKeys(list []SpellEntry) []string {
	var out []string
	for _, e := range list {
		out = append(out, e.Key)
	}
	return out
}

func TestListSpellsSearch(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	c := withOverlayOn(t, base, fullOverlay(t, base))
	for _, tc := range []struct {
		query string
		want  []string // keys that must be in the answer
	}{
		{"mãos flamejantes", []string{"spell:burning-hands"}},
		{"maos", []string{"spell:burning-hands"}},               // without the accent
		{"BURNING", []string{"spell:burning-hands"}},            // the SRD's English name, any case
		{"raio de teste", []string{"spell:raio-de-teste@mesa"}}, // the table's spell
		{"  escudo arcano ", []string{"spell:shield"}},          // spaces around
	} {
		got := spellKeys(c.ListSpells(SpellFilter{Query: tc.query}))
		for _, key := range tc.want {
			if !slices.Contains(got, key) {
				t.Errorf("query %q: %s is not in %v", tc.query, key, got)
			}
		}
	}
	if got := c.ListSpells(SpellFilter{Query: "zzz"}); len(got) != 0 {
		t.Errorf("query zzz = %v, want none", spellKeys(got))
	}
	// Sorted by the Portuguese name, as the catalog is.
	all := c.ListSpells(SpellFilter{})
	if len(all) != len(c.Catalog().Spells) || len(all) != 319+len(tableTestSpells()) {
		t.Errorf("no filter lists %d spells, the catalog has %d", len(all), len(c.Catalog().Spells))
	}
	if !slices.IsSortedFunc(all, func(a, b SpellEntry) int { return comparePT(a.NamePT, b.NamePT) }) {
		t.Error("the list is not sorted by the Portuguese name")
	}
}

func TestListSpellsFilters(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	c := withOverlayOn(t, base, fullOverlay(t, base))

	// By class: the SRD's list, and a table spell that names it.
	wizard := spellKeys(c.ListSpells(SpellFilter{Class: "class:wizard"}))
	for _, key := range []string{"spell:fireball", "spell:raio-de-teste@mesa", "spell:cone-de-teste@mesa"} {
		if !slices.Contains(wizard, key) {
			t.Errorf("the wizard's list has no %s", key)
		}
	}
	if slices.Contains(wizard, "spell:cura-de-teste@mesa") || slices.Contains(wizard, "spell:bless") {
		t.Error("the wizard's list has a cleric spell")
	}

	// A table class's list: the one it reuses (the cleric's) and what names it.
	gen := spellKeys(c.ListSpells(SpellFilter{Class: "class:gen-full-prepared@mesa"}))
	for _, key := range []string{"spell:bless", "spell:cura-de-teste@mesa", "spell:par-de-teste@mesa", "spell:cone-de-teste@mesa"} {
		if !slices.Contains(gen, key) {
			t.Errorf("the table class's list has no %s", key)
		}
	}
	if slices.Contains(gen, "spell:fireball") {
		t.Error("the table class (a cleric's list) has the wizard's Fireball")
	}

	// A third caster's subclass reads the list of the class it casts from.
	third := spellKeys(c.ListSpells(SpellFilter{Class: "subclass:cavaleiro-runico@mesa"}))
	if !slices.Equal(third, wizard) {
		t.Errorf("the third caster's list has %d spells, the wizard's %d", len(third), len(wizard))
	}
	// A class that casts nothing, a subclass that does not cast and an unknown key have no list.
	for _, key := range []string{"class:fighter", "class:barbarian", "subclass:champion", "spell:fireball", "class:nope"} {
		if got := c.ListSpells(SpellFilter{Class: key}); got != nil {
			t.Errorf("filter by %s lists %d spells, want none", key, len(got))
		}
	}

	// By circle and school.
	for _, e := range c.ListSpells(SpellFilter{Levels: []int{0, 9}}) {
		if e.Level != 0 && e.Level != 9 {
			t.Errorf("%s is circle %d", e.Key, e.Level)
		}
	}
	cantrips := spellKeys(c.ListSpells(SpellFilter{Levels: []int{0}, Class: "class:wizard"}))
	if !slices.Contains(cantrips, "spell:fire-bolt") || !slices.Contains(cantrips, "spell:faisca-de-teste@mesa") || slices.Contains(cantrips, "spell:fireball") {
		t.Errorf("wizard cantrips = %v", cantrips)
	}
	for _, e := range c.ListSpells(SpellFilter{Schools: []string{"school:abjuration", "school:divination"}}) {
		if e.School != "school:abjuration" && e.School != "school:divination" {
			t.Errorf("%s is %s", e.Key, e.School)
		}
	}
	// The filters add up.
	both := c.ListSpells(SpellFilter{Class: "class:wizard", Levels: []int{3}, Schools: []string{"school:evocation"}, Query: "fire"})
	if keys := spellKeys(both); !slices.Contains(keys, "spell:fireball") || slices.Contains(keys, "spell:fire-bolt") {
		t.Errorf("wizard 3rd evocation \"fire\" = %v", keys)
	}
}

// TestListSpellsLearnable: "Só as que posso aprender", the lists a character reads up to
// the highest circle of each, multiclass included.
func TestListSpellsLearnable(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	c := withOverlayOn(t, base, fullOverlay(t, base))
	// Maga 3 / Clériga 1: the wizard up to the 2nd, the cleric the 1st.
	got := c.ListSpells(SpellFilter{Learnable: []SpellAccess{{List: "class:wizard", MaxLevel: 2}, {List: "class:cleric", MaxLevel: 1}}})
	keys := spellKeys(got)
	for _, want := range []string{"spell:shield", "spell:web", "spell:bless", "spell:cure-wounds", "spell:raio-de-teste@mesa", "spell:par-de-teste@mesa", "spell:cura-de-teste@mesa"} {
		if !slices.Contains(keys, want) {
			t.Errorf("learnable has no %s", want)
		}
	}
	for _, not := range []string{"spell:fireball", "spell:cone-de-teste@mesa", "spell:spiritual-weapon", "spell:vampiric-touch"} {
		if slices.Contains(keys, not) {
			t.Errorf("learnable has %s", not)
		}
	}
	// Spiritual Weapon is a cleric's 2nd circle: out with a level-1 cleric, in with a level-3 one.
	if slices.Contains(keys, "spell:spiritual-weapon") {
		t.Error("a level-1 cleric learns Spiritual Weapon")
	}
	if got := spellKeys(c.ListSpells(SpellFilter{Learnable: []SpellAccess{{List: "class:cleric", MaxLevel: 2}}})); !slices.Contains(got, "spell:spiritual-weapon") {
		t.Error("a cleric that casts the 2nd circle does not learn Spiritual Weapon")
	}
	// A character that casts nothing learns nothing: an empty list is not "any".
	if got := c.ListSpells(SpellFilter{Learnable: []SpellAccess{}}); len(got) != 0 {
		t.Errorf("a character with no casting class learns %d spells", len(got))
	}
	// A class that only knows cantrips (circle 0) learns the cantrips.
	for _, e := range c.ListSpells(SpellFilter{Learnable: []SpellAccess{{List: "class:wizard"}}}) {
		if e.Level != 0 {
			t.Errorf("%s is circle %d", e.Key, e.Level)
		}
	}
}

// TestListSpellsHidden: what the caller hides is left out (a player never reads an
// archived table spell, and later the master's switches).
func TestListSpellsHidden(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	o := fullOverlay(t, base)
	for i := range o.Spells {
		o.Spells[i].Archived = o.Spells[i].Key == "spell:raio-de-teste@mesa"
	}
	c := withOverlayOn(t, base, o)
	if got := spellKeys(c.ListSpells(SpellFilter{Query: "raio de teste"})); !slices.Equal(got, []string{"spell:raio-de-teste@mesa"}) {
		t.Fatalf("without hiding, the master finds %v", got)
	}
	if got := c.ListSpells(SpellFilter{Query: "raio de teste", Hidden: c.Archived}); len(got) != 0 {
		t.Errorf("with the archived hidden, a player finds %v", spellKeys(got))
	}
}

// TestFiendPatronSpellsAreChosenNotGiven: the Fiend's Expanded Spell List only adds
// its spells to the warlock list, to choose from when the warlock learns a spell
// (SRD 5.1); the Life domain's spells, on the other hand, are always prepared.
func TestFiendPatronSpellsAreChosenNotGiven(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	patron := []string{"spell:command", "spell:burning-hands", "spell:blindness-deafness", "spell:scorching-ray", "spell:fireball", "spell:stinking-cloud"}
	has := func(d Derived, key string) (CharacterSpell, bool) {
		for _, s := range d.Spells {
			if s.Spell.Key == key {
				return s, true
			}
		}
		return CharacterSpell{}, false
	}

	b := standard("class:warlock", 5)
	b.Classes[0].Subclass = "subclass:fiend"
	d := Derive(b, c)
	for _, key := range patron {
		if s, ok := has(d, key); ok {
			t.Errorf("a Fiend warlock 5 who chose nothing has %s on the sheet (prepared %v)", key, s.Prepared)
		}
	}

	// Chosen among the spells known, a patron spell is a legal pick that counts.
	spellIssues := func(d Derived) []Issue {
		return slices.DeleteFunc(slices.Clone(d.Issues), func(i Issue) bool {
			return i.Code != IssueSpellNotOnList && i.Code != IssueSpellCount && i.Code != IssueSpellLevel && i.Code != IssueUnknownKey
		})
	}
	b.SpellsKnown = []string{"spell:burning-hands", "spell:command", "spell:charm-person", "spell:hellish-rebuke"}
	d = Derive(b, c)
	if is := spellIssues(d); len(is) != 0 {
		t.Errorf("Fiend spells picked among the known ones are refused: %+v", is)
	}
	if s, ok := has(d, "spell:burning-hands"); !ok || !s.Prepared {
		t.Errorf("a chosen Fiend spell is missing from the sheet or not castable: %+v %v", s, ok)
	}
	b.SpellsKnown = append(b.SpellsKnown, "spell:fireball", "spell:scorching-ray", "spell:expeditious-retreat")
	if d = Derive(b, c); !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueSpellCount }) {
		t.Errorf("seven spells known at warlock 5 (knows 6) raised no count issue: %+v", d.Issues)
	}

	// A domain's spells stay always prepared.
	cl := standard("class:cleric", 3)
	cl.Classes[0].Subclass = "subclass:life"
	if s, ok := has(Derive(cl, c), "spell:cure-wounds"); !ok || !s.Prepared {
		t.Errorf("a Life cleric 3 lacks cure wounds always prepared: %+v %v", s, ok)
	}
}
