package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The table's features for a class. Every name is invented; the classes and the features they replace are the SRD's.

const (
	trackerGaze   = "classfeature:olhar-de-rastreador@mesa"
	favoredEnemy1 = "feature:favored-enemy-1-type"
)

// trackerGazeFeature is an invented ranger feature at level 1 that takes the place of Favored Enemy.
func trackerGazeFeature() TableClassFeature {
	return TableClassFeature{
		TableEntry: TableEntry{Key: trackerGaze, NamePT: "Olhar de Rastreador"}, DescPT: []string{"Você lê rastros como outros leem cartas."},
		Class: "class:ranger", Level: 1, Replaces: favoredEnemy1,
		Effects: []Effect{{Type: "proficiency", Proficiency: "skill:survival"}},
	}
}

func featureKeys(d Derived) []string {
	out := make([]string, len(d.Features))
	for i, f := range d.Features {
		out[i] = f.Key
	}
	return out
}

func cfOverlay(fs ...TableClassFeature) Overlay { return Overlay{Revision: 1, ClassFeatures: fs} }

func ranger(level int) Build {
	return choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: level})
}

func TestAClassFeatureReplacesTheFeatureOfTheSRDClass(t *testing.T) {
	srd := loadForTest(t)
	before := Derive(ranger(1), srd)
	if !hasFeature(before, favoredEnemy1) {
		t.Fatalf("the plain ranger has no Favored Enemy: %v", featureKeys(before))
	}
	if ch := srd.Choices(ranger(1)); !slices.Contains(choiceKeys(ch), "feature:favored-enemy-1-type") {
		t.Fatalf("choices of the plain ranger = %v, want the Favored Enemy's", choiceKeys(ch))
	}

	c := withOverlay(t, cfOverlay(trackerGazeFeature()))
	d := Derive(ranger(1), c)
	if hasFeature(d, favoredEnemy1) || !hasFeature(d, trackerGaze) {
		t.Errorf("features = %v, want the table's feature and no Favored Enemy", featureKeys(d))
	}
	i := slices.IndexFunc(d.Features, func(f Feature) bool { return f.Key == trackerGaze })
	if f := d.Features[i]; f.NamePT != "Olhar de Rastreador" || f.Level != 1 || !strings.Contains(f.SourcePT, "1") {
		t.Errorf("feature = %+v, want the name, level 1 and the class as its source", f)
	}
	if s := skillOf(d, "skill:survival"); s.Proficiency == ProficiencyNone {
		t.Errorf("survival = %+v, want the proficiency of the feature", s)
	}
	// What the replaced feature asked of the player is gone with it.
	for _, k := range choiceKeys(c.Choices(ranger(1))) {
		if strings.Contains(k, "favored-enemy-1") {
			t.Errorf("a choice of the replaced feature is still asked: %s", k)
		}
	}
	// The class's other features, and its later Favored Enemy, stay.
	later := Derive(ranger(6), c)
	if !hasFeature(later, "feature:favored-enemy-2-types") || !hasFeature(later, "feature:ranger-fighting-style") {
		t.Errorf("at level 6 the class lost its other features")
	}
}

func TestAClassFeatureComesAtItsLevelAndOnATableClass(t *testing.T) {
	late := trackerGazeFeature()
	late.Replaces, late.Level, late.Class = "", 3, "class:fighter"
	late.Effects = []Effect{{Type: "sense", Sense: "darkvision", RangeFt: 30}}
	c := withOverlay(t, cfOverlay(late))
	fighter := func(level int) Build { return choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: level}, "feature:fighter-fighting-style-defense") }
	if hasFeature(Derive(fighter(2), c), trackerGaze) {
		t.Error("the feature is there before its level")
	}
	d := Derive(fighter(3), c)
	if !hasFeature(d, trackerGaze) || !slices.ContainsFunc(d.Senses, func(s Sense) bool { return s.Key == "darkvision" }) {
		t.Errorf("at level 3: features %v senses %+v, want the feature and its darkvision", featureKeys(d), d.Senses)
	}

	// A class of the table takes one too.
	tc := classFromDefaults(DefaultTable{Kind: CastingNone, Rows: loadDefaultRows()}, "cavaleiro")
	cf := late
	cf.Class, cf.Level = tc.Key, 2
	o := Overlay{Revision: 1, Classes: []TableClass{tc}, ClassFeatures: []TableClassFeature{cf}}
	tcContent := withOverlay(t, o)
	b := choiceBuild("race:human", "", ClassLevel{Class: tc.Key, Level: 2})
	if !hasFeature(Derive(b, tcContent), trackerGaze) {
		t.Error("a table class does not get the feature")
	}
}

func TestASwitchedOffClassFeatureChangesNothing(t *testing.T) {
	o := cfOverlay(trackerGazeFeature())
	o.Off = []string{trackerGaze}
	c := withOverlay(t, o)
	d := Derive(ranger(1), c)
	if !hasFeature(d, favoredEnemy1) || hasFeature(d, trackerGaze) {
		t.Errorf("features = %v, want the class as before", featureKeys(d))
	}
	if got := c.ClassFeatures(); len(got) != 1 || !got[0].Off || got[0].Class != "class:ranger" || got[0].Level != 1 {
		t.Errorf("ClassFeatures() = %+v, want the one entry, off", got)
	}
	// Archived is not off: the class keeps the feature (the master retires it from the lists).
	arch := trackerGazeFeature()
	arch.Archived = true
	if !hasFeature(Derive(ranger(1), withOverlay(t, cfOverlay(arch))), trackerGaze) {
		t.Error("an archived class feature stopped applying")
	}
}

func TestAClassFeatureMayAskTheSameChoicesAsAnyFeature(t *testing.T) {
	f := trackerGazeFeature()
	f.Replaces = ""
	f.Effects = []Effect{{Type: "choice", Choice: "skill", Count: 1}}
	c := withOverlay(t, cfOverlay(f))
	if d := Derive(ranger(1), c); !slices.ContainsFunc(d.Features, func(x Feature) bool { return x.Key == trackerGaze }) {
		t.Fatalf("features = %v", featureKeys(d))
	}
}

func TestAClassFeatureRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name  string
		edit  func(f *TableClassFeature)
		field string
	}{
		{"an unknown class", func(f *TableClassFeature) { f.Class = "class:nao-existe" }, ".class_key"},
		{"level 0", func(f *TableClassFeature) { f.Level = 0 }, ".level"},
		{"level 21", func(f *TableClassFeature) { f.Level = 21 }, ".level"},
		{"a replaced feature the class lacks", func(f *TableClassFeature) { f.Replaces = "feature:second-wind" }, ".replaces_feature_key"},
		{"a replaced feature that is no feature", func(f *TableClassFeature) { f.Replaces = "spell:light" }, ".replaces_feature_key"},
		{"a feature that comes earlier", func(f *TableClassFeature) { f.Level = 6 }, ".replaces_feature_key"},
		{"itself", func(f *TableClassFeature) { f.Replaces = trackerGaze }, ".replaces_feature_key"},
	}
	for _, tc := range cases {
		f := trackerGazeFeature()
		tc.edit(&f)
		_, err := srd.With(cfOverlay(f))
		oe, ok := errors.AsType[*OverlayError](err)
		if !ok {
			t.Errorf("%s: err = %v, want an overlay error", tc.name, err)
			continue
		}
		found := false
		for _, v := range oe.Violations() {
			found = found || strings.HasSuffix(v.Field, tc.field)
		}
		if !found {
			t.Errorf("%s: refused at %+v, want the field %s", tc.name, oe.Violations()[0], tc.field)
		}
	}
	// Two features that replace the same one: the second is refused.
	a, b := trackerGazeFeature(), trackerGazeFeature()
	b.Key, b.NamePT = "classfeature:outro-olhar@mesa", "Outro Olhar"
	if _, err := srd.With(cfOverlay(a, b)); err == nil {
		t.Error("a feature replaced twice was accepted")
	}
}

func TestOneClassFeatureMayReplaceAnother(t *testing.T) {
	base := trackerGazeFeature()
	base.Replaces = ""
	next := trackerGazeFeature()
	next.Key, next.NamePT, next.Replaces = "classfeature:segundo-olhar@mesa", "Segundo Olhar", trackerGaze
	next.Effects = nil
	c := withOverlay(t, cfOverlay(base, next))
	d := Derive(ranger(1), c)
	if hasFeature(d, trackerGaze) || !hasFeature(d, next.Key) {
		t.Errorf("features = %v, want only the second feature", featureKeys(d))
	}
}
