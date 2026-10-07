package rules

import (
	"strconv"
	"testing"
)

// realisticOverlay is a table at the limit: 300 entries, as a master who writes
// a whole setting would have: 10 classes of every casting kind with 20-level
// tables and a dozen features each, 30 subclasses (some of SRD classes), 20
// races with traits, 40 subraces, 40 backgrounds and 160 spells.
func realisticOverlay(t testing.TB, c *Content) Overlay {
	t.Helper()
	o := Overlay{Revision: 42}
	for i := range 10 {
		gk := genKinds[i%len(genKinds)]
		tc := genClass(c, gk)
		slug := "classe-" + strconv.Itoa(i)
		tc.Key, tc.NamePT = "class:"+slug+tableSuffix, "Classe "+strconv.Itoa(i)
		for j := range tc.Levels {
			tc.Levels[j].Features = genFeatures(slug, j+1)
		}
		tc.Casting.ListFrom = ""
		if gk.casting.Kind != CastingNone {
			tc.Casting.ListFrom = gk.casting.ListFrom
		}
		o.Classes = append(o.Classes, tc)
		for k := range 3 {
			o.Subclasses = append(o.Subclasses, TableSubclass{
				Key: "subclass:" + slug + "-" + strconv.Itoa(k) + tableSuffix, NamePT: "Caminho " + strconv.Itoa(k),
				Class: tc.Key,
				Levels: []TableSubclassLevel{
					{Level: 3, Features: []TableFeature{tf(slug+"-s"+strconv.Itoa(k)+"-3", "A", Effect{Type: "note", TextPT: "Nota."})}},
					{Level: 6, Features: []TableFeature{tf(slug+"-s"+strconv.Itoa(k)+"-6", "B", Effect{Type: "modifier", Target: "speed.walk", Mode: "add", Value: "5"})}},
					{Level: 10, Features: []TableFeature{tf(slug+"-s"+strconv.Itoa(k)+"-10", "C")}},
				},
			})
		}
	}
	for i := range 20 {
		slug := "raca-" + strconv.Itoa(i)
		o.Races = append(o.Races, TableRace{
			Key: "race:" + slug + tableSuffix, NamePT: "Raça " + strconv.Itoa(i),
			Size: "Medium", SpeedFt: 30, AbilityBonuses: map[Ability]int{CON: 1}, DarkvisionFt: 60,
			Languages: []string{"language:common"}, LanguageChoices: 1,
			Traits: []TableFeature{
				{Key: "trait:" + slug + "-a" + tableSuffix, NamePT: "A", DescPT: []string{"Texto."}, Effects: []Effect{{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.con"}, Tags: []string{"against:poison"}}}},
				{Key: "trait:" + slug + "-b" + tableSuffix, NamePT: "B", DescPT: []string{"Texto."}},
			},
		})
		for k := range 2 {
			o.Subraces = append(o.Subraces, TableSubrace{
				Key: "subrace:" + slug + "-" + strconv.Itoa(k) + tableSuffix, NamePT: "Sub-raça " + strconv.Itoa(k),
				Race: "race:" + slug + tableSuffix, AbilityBonuses: map[Ability]int{WIS: 1},
				Traits: []TableFeature{{Key: "trait:" + slug + "-" + strconv.Itoa(k) + tableSuffix, NamePT: "T", DescPT: []string{"Texto."}}},
			})
		}
	}
	_, _, bg := tableMisc()
	for i := range 40 {
		b := bg
		b.Key = "background:antecedente-" + strconv.Itoa(i) + tableSuffix
		b.Feature.Key = "background-feature:antecedente-" + strconv.Itoa(i) + tableSuffix
		o.Backgrounds = append(o.Backgrounds, b)
	}
	spells := tableTestSpells()
	for i := range 160 {
		s := spells[i%len(spells)]
		s.Key = "spell:magia-" + strconv.Itoa(i) + tableSuffix
		s.Classes = []string{"class:wizard", "class:classe-1" + tableSuffix}
		o.Spells = append(o.Spells, s)
	}
	return o
}

// TestRealisticOverlayIsValid keeps the benchmark honest: the overlay it runs
// is valid, and has exactly 300 entries.
func TestRealisticOverlayIsValid(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := realisticOverlay(t, srd)
	if n := len(o.Classes) + len(o.Subclasses) + len(o.Races) + len(o.Subraces) + len(o.Backgrounds) + len(o.Spells); n != MaxOverlayEntries {
		t.Fatalf("%d entries, want %d", n, MaxOverlayEntries)
	}
	if _, err := srd.With(o); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkWith measures one With of a 300-entry overlay, the cost of a content
// cache miss (ADR-0018: about 25 ms and 4 MB; eight cached contents in about
// 24 MB). Run it with
// `go test -run '^$' -bench 'BenchmarkWith' -benchmem ./internal/rules` (in
// backend/, without -race).
func BenchmarkWith(b *testing.B) {
	srd := loadForTest(b)
	o := realisticOverlay(b, srd)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := srd.With(o); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWithEmpty is the fixed part of a With: cloning the maps and
// rebuilding the catalog with nothing added.
func BenchmarkWithEmpty(b *testing.B) {
	srd := loadForTest(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := srd.With(Overlay{}); err != nil {
			b.Fatal(err)
		}
	}
}
