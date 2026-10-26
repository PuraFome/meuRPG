package rules

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// scaleSizes are the entry counts the scale tests and benchmarks measure: the
// first limit of a table, what the table's books come to, two steps between, and
// the limit.
var scaleSizes = []int{300, 450, 600, 800, 1000}

// bookOverlay is a table of n entries as the books make it: mostly spells (with
// class lists, damage, saves and areas), then subclasses of SRD and table
// classes with features and effects, subraces, races with traits, feats,
// backgrounds and a few classes with 20-level tables. Names, keys, texts and
// numbers vary from entry to entry, so nothing is degenerate. The shares follow
// n, so the same call gives 300, 450 or 1000 entries.
func bookOverlay(t testing.TB, c *Content, n int) Overlay {
	t.Helper()
	o := Overlay{Revision: 7}
	classes, subclasses, races := n/70, n/9, n/14
	subraces, backgrounds, feats := n/12, n/20, n/12
	spells := n - classes - subclasses - races - subraces - backgrounds - feats

	srdClasses := []string{"class:barbarian", "class:bard", "class:cleric", "class:druid", "class:fighter", "class:monk", "class:paladin", "class:ranger", "class:rogue", "class:sorcerer", "class:warlock", "class:wizard"}
	var tableClasses []string
	for i := range classes {
		gk := genKinds[i%len(genKinds)]
		tc := genClass(c, gk)
		slug := "classe-" + strconv.Itoa(i)
		tc.Key, tc.NamePT = "class:"+slug+tableSuffix, "Classe "+strconv.Itoa(i)
		for j := range tc.Levels {
			tc.Levels[j].Features = genFeatures(slug, j+1)
		}
		o.Classes = append(o.Classes, tc)
		tableClasses = append(tableClasses, tc.Key)
	}
	for i := range subclasses {
		parent := srdClasses[i%len(srdClasses)]
		if len(tableClasses) > 0 && i%4 == 0 {
			parent = tableClasses[i%len(tableClasses)]
		}
		slug := "caminho-" + strconv.Itoa(i)
		o.Subclasses = append(o.Subclasses, TableSubclass{
			TableEntry: TableEntry{Key: "subclass:" + slug + tableSuffix, NamePT: "Caminho " + strconv.Itoa(i)},
			Class:      parent,
			Levels: []TableSubclassLevel{
				{Level: 3, Features: []TableFeature{tf(slug+"-a", "A "+strconv.Itoa(i), Effect{Type: "note", TextPT: "Nota do caminho " + strconv.Itoa(i) + "."})}},
				{Level: 6, Features: []TableFeature{tf(slug+"-b", "B "+strconv.Itoa(i), Effect{Type: "modifier", Target: "speed.walk", Mode: "add", Value: strconv.Itoa(5 * (1 + i%3))})}},
				{Level: 10, Features: []TableFeature{tf(slug+"-c", "C "+strconv.Itoa(i))}},
				{Level: 14, Features: []TableFeature{tf(slug+"-d", "D "+strconv.Itoa(i), Effect{Type: "proficiency", Proficiency: "skill:survival"})}},
			},
		})
	}
	race0, sub0, bg0 := tableMisc()
	for i := range races {
		r := race0
		slug := "raca-" + strconv.Itoa(i)
		r.Key, r.NamePT = "race:"+slug+tableSuffix, "Raça "+strconv.Itoa(i)
		r.SpeedFt = 25 + 5*(i%3)
		r.Traits = []TableFeature{
			{Key: "trait:" + slug + "-a" + tableSuffix, NamePT: "A", DescPT: bookText(i, 2), Effects: race0.Traits[0].Effects},
			{Key: "trait:" + slug + "-b" + tableSuffix, NamePT: "B", DescPT: bookText(i+1, 1)},
			{Key: "trait:" + slug + "-c" + tableSuffix, NamePT: "C", DescPT: bookText(i+2, 1), Effects: []Effect{{Type: "proficiency", Proficiency: "skill:perception"}}},
		}
		o.Races = append(o.Races, r)
	}
	for i := range subraces {
		s := sub0
		slug := "subraca-" + strconv.Itoa(i)
		s.Key, s.NamePT = "subrace:"+slug+tableSuffix, "Sub-raça "+strconv.Itoa(i)
		s.Race = o.Races[i%len(o.Races)].Key
		s.Traits = []TableFeature{{Key: "trait:" + slug + tableSuffix, NamePT: "T", DescPT: bookText(i, 1)}}
		o.Subraces = append(o.Subraces, s)
	}
	for i := range backgrounds {
		b := bg0
		b.Key, b.NamePT = "background:antecedente-"+strconv.Itoa(i)+tableSuffix, "Antecedente "+strconv.Itoa(i)
		b.Feature.Key = "background-feature:antecedente-" + strconv.Itoa(i) + tableSuffix
		o.Backgrounds = append(o.Backgrounds, b)
	}
	for i := range feats {
		f := TableFeat{
			TableEntry: TableEntry{Key: "feat:talento-" + strconv.Itoa(i) + tableSuffix, NamePT: "Talento " + strconv.Itoa(i)},
			DescPT:     bookText(i, 2),
			Effects:    []Effect{{Type: "modifier", Target: "initiative", Mode: "add", Value: strconv.Itoa(1 + i%3)}},
		}
		if i%3 == 0 {
			f.Prerequisite = FeatPrerequisite{Minimums: map[Ability]int{CON: 13}}
		}
		o.Feats = append(o.Feats, f)
	}
	o.Spells = bookSpells(spells, srdClasses, tableClasses)
	return o
}

// bookSpells are n spells with class lists of the SRD's classes and the table's.
func bookSpells(n int, srdClasses, tableClasses []string) []TableSpell {
	base := tableTestSpells()
	var out []TableSpell
	for i := range n {
		s := base[i%len(base)]
		slug := "magia-" + strconv.Itoa(i)
		s.Key, s.NamePT = "spell:"+slug+tableSuffix, "Magia "+strconv.Itoa(i)
		s.DescPT = bookText(i, 3)
		s.Classes = []string{srdClasses[(i+3)%len(srdClasses)]}
		if s.Classes[0] != "class:wizard" {
			s.Classes = append(s.Classes, "class:wizard")
		}
		if len(tableClasses) > 0 && i%5 == 0 {
			s.Classes = append(s.Classes, tableClasses[i%len(tableClasses)])
		}
		if s.Level > 0 {
			s.Level = 1 + i%9
		}
		out = append(out, s)
	}
	return out
}

// bookText is paragraphs of about 300 characters, each one different.
func bookText(seed, paragraphs int) []string {
	out := make([]string, paragraphs)
	for i := range out {
		out[i] = strings.Repeat(fmt.Sprintf("Texto %d do livro, parágrafo %d: a criatura alvo faz o que a descrição manda. ", seed, i), 4)
	}
	return out
}

func entryCount(o Overlay) int {
	return len(o.Classes) + len(o.Subclasses) + len(o.Races) + len(o.Subraces) + len(o.Backgrounds) + len(o.Spells) + len(o.Feats)
}

// TestBookOverlayIsValid keeps the scale benchmarks honest: the overlay of each
// size is valid and has exactly that many entries, and the spells are most of
// them.
func TestBookOverlayIsValid(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for _, n := range scaleSizes {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			o := bookOverlay(t, srd, n)
			if got := entryCount(o); got != n {
				t.Fatalf("%d entries, want %d", got, n)
			}
			if len(o.Spells)*2 < n {
				t.Fatalf("%d spells of %d entries: not mostly spells", len(o.Spells), n)
			}
			if _, err := srd.With(o); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// BenchmarkWithEntries measures one With of the books' mix at each size: the
// cost of a content cache miss as the table grows. Run it with
// `go test -run '^$' -bench 'BenchmarkWithEntries' -benchmem -benchtime 2s -count 5 ./internal/rules`
// (in backend/, without -race).
func BenchmarkWithEntries(b *testing.B) {
	srd := loadForTest(b)
	for _, n := range scaleSizes {
		o := bookOverlay(b, srd, n)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := srd.With(o); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestWithMemoryByEntries measures what a With of the books' mix keeps alive at
// each size, minus the shared SRD, with MEURPG_MEASURE=1 and -v.
func TestWithMemoryByEntries(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	srd := loadForTest(t)
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	const copies = 4
	for _, n := range scaleSizes {
		o := bookOverlay(t, srd, n)
		keep := make([]*Content, 0, copies)
		before := heap()
		for range copies {
			c, err := srd.With(o)
			if err != nil {
				t.Fatal(err)
			}
			keep = append(keep, c)
		}
		after := heap()
		t.Logf("%d entries: a content keeps %.2f MB", n, float64(after-before)/copies/1e6)
		runtime.KeepAlive(keep)
	}
}
