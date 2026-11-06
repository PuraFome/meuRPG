package rules

import (
	"slices"
	"testing"
)

// A trait, feature, background feature or option is as hidden to the players as the entry it
// belongs to (RN-23): the race, subrace, class, subclass or background, retired or switched off.

const (
	ownerRaceTrait     = "trait:resistente" + tableSuffix
	ownerRaceOption    = "trait:rio-a" + tableSuffix
	ownerRaceOptionSrc = "trait:forca-do-rio" + tableSuffix
)

// ownerFixture is a table with a race (a trait, and a trait with an option), a subrace, a background,
// a class and a subclass of the SRD Fighter with an option list.
type ownerFixture struct {
	overlay                                                      Overlay
	race, subrace, background, class, subclass                   string
	classFeature, subclassFeature, subclassOption, backgroundOne string
}

func newOwnerFixture(srd *Content) ownerFixture {
	race, sub, bg := tableMisc()
	race.Traits = append(race.Traits, TableFeature{
		Key: ownerRaceOptionSrc, NamePT: "Força do rio", Options: []TableFeature{{Key: ownerRaceOption, NamePT: "Rio A"}},
	})
	class := genClass(srd, genKinds[0])
	posture := postureOverlay()
	f := ownerFixture{
		race: race.Key, subrace: sub.Key, background: bg.Key, class: class.Key, subclass: postureSubclass,
		classFeature: class.Levels[0].Features[0].Key, subclassFeature: postureFeature, subclassOption: postureHeron, backgroundOne: bg.Feature.Key,
	}
	f.overlay = Overlay{
		Revision: 1, Races: []TableRace{race}, Subraces: []TableSubrace{sub}, Backgrounds: []TableBackground{bg},
		Classes: []TableClass{class}, Subclasses: posture.Subclasses,
	}
	return f
}

func TestHiddenFollowsTheOwnerOfAFeatureKey(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	f := newOwnerFixture(srd)
	on := withOverlayOn(t, srd, f.overlay) // the control: nothing hidden
	cases := []struct {
		name string
		// off is the owner switched off, and keys the keys of the kind that must then be hidden.
		off  string
		keys []string
	}{
		{"trait of a race", f.race, []string{ownerRaceTrait}},
		{"option of a trait of a race", f.race, []string{ownerRaceOptionSrc, ownerRaceOption}},
		{"trait of a subrace, under its race", f.race, []string{"trait:olhar-firme" + tableSuffix}},
		{"trait of a subrace", f.subrace, []string{"trait:olhar-firme" + tableSuffix}},
		{"feature of a class", f.class, []string{f.classFeature}},
		{"feature of a subclass", f.subclass, []string{f.subclassFeature}},
		{"option of a feature of a subclass", f.subclass, []string{f.subclassOption}},
		{"feature of a subclass, under its class", "class:fighter", []string{f.subclassFeature, f.subclassOption}},
		{"feature of an SRD class", "class:fighter", []string{"feature:second-wind"}},
		{"background feature", f.background, []string{f.backgroundOne}},
	}
	for _, tc := range cases {
		for _, how := range []string{"off", "archived"} {
			t.Run(tc.name+"/"+how, func(t *testing.T) {
				t.Parallel()
				o := newOwnerFixture(srd).overlay
				if how == "off" {
					o.Off = []string{tc.off}
				} else if !archiveOwner(&o, tc.off) {
					t.Skip("an SRD entry is switched off, not retired")
				}
				c := withOverlayOn(t, srd, o)
				for _, k := range tc.keys {
					if !c.Hidden(k) {
						t.Errorf("Hidden(%s) = false with %s %s", k, tc.off, how)
					}
				}
				// The control: with the owner on, the same keys are read.
				for _, k := range tc.keys {
					if on.Hidden(k) {
						t.Errorf("Hidden(%s) = true with nothing hidden", k)
					}
				}
			})
		}
	}
}

// archiveOwner retires the table entry key in o, or says it is none of the table's.
func archiveOwner(o *Overlay, key string) bool {
	for i := range o.Races {
		if o.Races[i].Key == key {
			o.Races[i].Archived = true
			return true
		}
	}
	for i := range o.Subraces {
		if o.Subraces[i].Key == key {
			o.Subraces[i].Archived = true
			return true
		}
	}
	for i := range o.Backgrounds {
		if o.Backgrounds[i].Key == key {
			o.Backgrounds[i].Archived = true
			return true
		}
	}
	for i := range o.Classes {
		if o.Classes[i].Key == key {
			o.Classes[i].Archived = true
			return true
		}
	}
	for i := range o.Subclasses {
		if o.Subclasses[i].Key == key {
			o.Subclasses[i].Archived = true
			return true
		}
	}
	return false
}

func TestHiddenLeavesTheKeysOfVisibleOwnersAlone(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	f := newOwnerFixture(srd)
	o := f.overlay
	o.Off = []string{f.race}
	c := withOverlayOn(t, srd, o)
	// The race is off: only what belongs to it goes.
	for _, k := range []string{f.classFeature, f.subclassFeature, f.subclassOption, f.backgroundOne, "feature:second-wind", "feat:grappler"} {
		if c.Hidden(k) {
			t.Errorf("Hidden(%s) = true with only the race off", k)
		}
	}
	if !c.Hidden(ownerRaceTrait) {
		t.Errorf("Hidden(%s) = false with its race off", ownerRaceTrait)
	}
}

func TestHiddenSharedTraitGoesWithAllItsOwners(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	const shared = "trait:darkvision"
	owners := srd.Owners(shared)
	if len(owners) < 2 {
		t.Fatalf("Owners(%s) = %v, want several races", shared, owners)
	}
	some := withOverlayOn(t, srd, Overlay{Revision: 1, Off: owners[:1]})
	if some.Hidden(shared) {
		t.Errorf("Hidden(%s) = true with one of %d owners off", shared, len(owners))
	}
	all := withOverlayOn(t, srd, Overlay{Revision: 1, Off: slices.Clone(owners)})
	if !all.Hidden(shared) {
		t.Errorf("Hidden(%s) = false with all of %v off", shared, owners)
	}
}
