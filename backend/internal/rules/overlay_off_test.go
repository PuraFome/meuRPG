package rules

import (
	"slices"
	"testing"
)

// The master's switches (RN-23, "Opções para os jogadores"): Overlay.Off turns SRD
// and table options off for the players. The engine marks them and answers who is
// hidden; the server filters what a player receives.

func TestOverlayOffMarksSRDAndTableOptions(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := genOverlay(t, srd)
	o.Revision = 3
	o.Off = []string{
		"class:wizard", "race:dwarf", "spell:fireball", "background:acolyte",
		"subclass:cavaleiro-runico@mesa", "not-a-key", "class:ghost", "feature:fighter-fighting-style-defense",
	}
	c := withOverlayOn(t, srd, o)

	for _, k := range []string{"class:wizard", "race:dwarf", "spell:fireball", "background:acolyte", "subclass:cavaleiro-runico@mesa"} {
		if !c.Off(k) || !c.Hidden(k) {
			t.Errorf("%s: Off = %v, Hidden = %v, want both", k, c.Off(k), c.Hidden(k))
		}
	}
	// What is no class, subclass, race, subrace, background or spell is ignored.
	for _, k := range []string{"not-a-key", "class:ghost", "feature:fighter-fighting-style-defense"} {
		if c.Off(k) {
			t.Errorf("%s is off: a key that is not switchable must be ignored", k)
		}
	}
	if c.Off("class:fighter") || c.Hidden("class:fighter") {
		t.Error("the fighter is on")
	}
	// The catalog carries the mark, the entry's own.
	var marked []string
	for _, e := range c.Catalog().Classes {
		if e.Off {
			marked = append(marked, e.Key)
		}
	}
	if !slices.Equal(marked, []string{"class:wizard"}) {
		t.Errorf("classes marked off = %v, want only the wizard", marked)
	}
	for _, e := range c.Catalog().Spells {
		if e.Off != (e.Key == "spell:fireball") {
			t.Errorf("spell %s: Off = %v", e.Key, e.Off)
		}
	}
	// The details of a spell carry it too (the entry is part of them).
	if d, ok := c.SpellDetails("spell:fireball"); !ok || !d.Spell.Off {
		t.Errorf("fireball details: %+v", d.Spell)
	}
	if d, ok := c.SpellDetails("spell:fire-bolt"); !ok || d.Spell.Off {
		t.Errorf("fire bolt details: %+v", d.Spell)
	}
	if !c.AnyHidden() {
		t.Error("AnyHidden = false")
	}
	if !c.Switchable("class:wizard") || !c.Switchable("subclass:cavaleiro-runico@mesa") || c.Switchable("feature:x") || c.Switchable("class:ghost") {
		t.Error("Switchable disagrees with the content")
	}
}

func TestOverlayOffHidesTheChildrenOfAnOffParent(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := genOverlay(t, srd)
	o.Off = []string{"class:fighter", "race:elf"}
	c := withOverlayOn(t, srd, o)
	// A subclass of an off class and a subrace of an off race are hidden, and keep
	// their own switch as it was.
	for _, k := range []string{"subclass:champion", "subclass:cavaleiro-runico@mesa", "subrace:high-elf"} {
		if !c.Hidden(k) {
			t.Errorf("%s is not hidden under its off parent", k)
		}
		if c.Off(k) {
			t.Errorf("%s has its own switch on", k)
		}
	}
	if c.Hidden("subclass:evocation") || c.Hidden("subrace:hill-dwarf") {
		t.Error("the children of an on parent are on")
	}
	// OffKeys names what a sheet uses that is off. A sheet that has the off class or
	// race judges its subclass and subrace by their own switch (question 80)...
	b := sweepBase(t, c, "class:fighter", "subclass:champion")
	b = sweepUp(t, c, b, "class:fighter", "subclass:champion", 3)
	b.Race, b.Subrace = "race:elf", "subrace:high-elf"
	if got := c.OffKeys(b); !slices.Equal(got, []string{"class:fighter", "race:elf"}) {
		t.Errorf("OffKeys = %v, want only the class and the race", got)
	}
	// ...while a build that lacks the parent has its child named too.
	lacking := b
	lacking.Classes = []ClassLevel{{Class: "class:wizard", Level: 3, Subclass: "subclass:champion"}}
	lacking.Race, lacking.Subrace = "race:human", "subrace:high-elf"
	if got := c.OffKeys(lacking); !slices.Equal(got, []string{"subclass:champion", "subrace:high-elf"}) {
		t.Errorf("OffKeys without the parents = %v, want the subclass and the subrace", got)
	}
	// An off key never makes a sheet invalid: it keeps working (question 80).
	if err := Validate(b, c); err != nil {
		t.Errorf("Validate of a sheet with off options: %v", err)
	}
}

func TestOverlayOffLeavesTheBaseAndOtherContentsAlone(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	before := fingerprint(t, srd)
	o := Overlay{Revision: 1, Off: []string{"class:wizard", "spell:fireball"}}
	c, err := srd.With(o) // only switches, no entries
	if err != nil {
		t.Fatalf("With(only switches): %v", err)
	}
	if !c.Off("class:wizard") || c.TableRevision() != 1 {
		t.Errorf("Off = %v, revision = %d", c.Off("class:wizard"), c.TableRevision())
	}
	if srd.Off("class:wizard") || srd.AnyHidden() {
		t.Error("the SRD content was switched off")
	}
	if got := fingerprint(t, srd); got != before {
		t.Error("With(Off) changed the base content")
	}
	other, err := srd.With(Overlay{Revision: 2})
	if err != nil || other.Off("class:wizard") {
		t.Errorf("another content sees the switches: err = %v", err)
	}
}

func TestOffSubclassInTheLevelUpOffer(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := genOverlay(t, srd)
	o.Off = []string{"subclass:cavaleiro-runico@mesa"}
	c := withOverlayOn(t, srd, o)
	b := sweepBase(t, c, "class:fighter", "subclass:champion")
	b = sweepUp(t, c, b, "class:fighter", "subclass:champion", 2)
	offer, err := LevelUpOptions(b, "class:fighter", c)
	if err != nil || !offer.SubclassDue {
		t.Fatalf("offer %+v, err %v", offer, err)
	}
	off := map[string]bool{}
	for _, s := range offer.Subclasses {
		off[s.Key] = s.Off
	}
	if !off["subclass:cavaleiro-runico@mesa"] || off["subclass:champion"] || off["subclass:trapaceiro-mistico@mesa"] {
		t.Errorf("off marks = %v", off)
	}
	// With the class off, the offer (for a character that has the class) still judges
	// each subclass by its own switch.
	o.Off = []string{"class:fighter"}
	c = withOverlayOn(t, srd, o)
	if offer, err = LevelUpOptions(b, "class:fighter", c); err != nil {
		t.Fatal(err)
	}
	for _, s := range offer.Subclasses {
		if s.Off {
			t.Errorf("subclass %s is marked off by its class's switch", s.Key)
		}
	}
}

func TestHiddenSaysTheChildOfAnArchivedParentIsHidden(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := fullOverlay(t, srd)
	for i := range o.Classes {
		if o.Classes[i].Key == "class:gen-full-prepared@mesa" {
			o.Classes[i].Archived = true
		}
	}
	o.Races[0].Archived = true
	c := withOverlayOn(t, srd, o)
	for _, k := range []string{"subclass:gen-full-prepared-a@mesa", "subrace:da-colina-nevoenta@mesa"} {
		if !c.Hidden(k) || c.Off(k) {
			t.Errorf("%s: Hidden = %v, Off = %v, want hidden by its retired parent, its own switch untouched", k, c.Hidden(k), c.Off(k))
		}
	}
	if c.Hidden("subclass:champion") {
		t.Error("an SRD subclass is hidden")
	}
}
