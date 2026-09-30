package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// loadForTest loads the embedded content once per test.
func loadForTest(t testing.TB) *Content {
	t.Helper()
	c, err := LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	return c
}

// TestSnapshot checks the embedded snapshot against its manifest: the
// pinned commit is recorded, every generated file still has the hash the
// importer wrote (so nobody edited data/ by hand), the effects revision
// matches the effects files, and the NOTICE carries the exact attribution.
func TestSnapshot(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	m := c.c.manifest

	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(m.SourceCommit) {
		t.Errorf("manifest source_commit = %q, want a full commit SHA", m.SourceCommit)
	}
	if m.SourceRepo != "https://github.com/5e-bits/5e-srd-api" {
		t.Errorf("manifest source_repo = %q", m.SourceRepo)
	}
	if want := "srd51@" + m.SourceCommit[:12]; m.SnapshotVersion != want {
		t.Errorf("snapshot_version = %q, want %q", m.SnapshotVersion, want)
	}
	if !regexp.MustCompile(`^srd51@[0-9a-f]{12}\+fx\.[1-9][0-9]*$`).MatchString(c.Version()) {
		t.Errorf("Version() = %q, want srd51@<sha12>+fx.<n>", c.Version())
	}

	t.Run("every generated file matches the manifest", func(t *testing.T) {
		files, err := fs.Glob(srd51.Files, "data/*.json")
		if err != nil {
			t.Fatal(err)
		}
		listed := map[string]string{}
		for _, o := range m.Outputs {
			listed[o.Name] = o.SHA256
		}
		for _, f := range files {
			name := path.Base(f)
			if name == "manifest.json" {
				continue
			}
			b, err := fs.ReadFile(srd51.Files, f)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(b)
			want, ok := listed[name]
			switch {
			case !ok:
				t.Errorf("%s is not in the manifest: generate data/ with cmd/srdimport", name)
			case hex.EncodeToString(sum[:]) != want:
				t.Errorf("%s changed after cmd/srdimport wrote it: data/ is generated, never edited by hand", name)
			}
			delete(listed, name)
		}
		for name := range listed {
			t.Errorf("manifest lists %s, which is missing", name)
		}
		if len(m.Inputs) == 0 {
			t.Error("manifest has no input hashes")
		}
	})

	t.Run("effects revision matches the effects files", func(t *testing.T) {
		var rev effectsRevision
		if err := readJSON(srd51.Files, "effects/revision.json", &rev); err != nil {
			t.Fatal(err)
		}
		got := effectsHash(t)
		if rev.SHA256 != got {
			t.Errorf("effects/ changed: content versions never change in place (ADR-0008), so raise \"revision\" in effects/revision.json to %d and set \"sha256\" to %q", rev.Revision+1, got)
		}
		if !strings.HasSuffix(c.Version(), "+fx."+strconv.Itoa(rev.Revision)) {
			t.Errorf("Version() = %q does not end in fx.%d", c.Version(), rev.Revision)
		}
	})

	t.Run("NOTICE has the attribution", func(t *testing.T) {
		b, err := os.ReadFile("../../../NOTICE")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), srd51.Attribution) {
			t.Error("NOTICE must contain srd51.Attribution byte for byte")
		}
		if c.Catalog().Attribution != srd51.Attribution {
			t.Error("Catalog().Attribution differs from srd51.Attribution")
		}
		if !strings.Contains(string(b), m.SourceCommit) {
			t.Error("NOTICE must name the pinned 5e-srd-api commit")
		}
	})
}

// effectsHash is the sha256 of every file in effects/ but revision.json, by
// name and content, in name order.
func effectsHash(t testing.TB) string {
	t.Helper()
	files, err := fs.Glob(srd51.Files, "effects/*.json")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	h := sha256.New()
	for _, f := range files {
		if path.Base(f) == "revision.json" {
			continue
		}
		b, err := fs.ReadFile(srd51.Files, f)
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(path.Base(f)))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestEffectsCoverLevels1To5 makes sure every feature a class or SRD
// subclass gives at levels 1 to 5, every racial trait and every background
// feature was reviewed: each has an effects entry, even if only a note
// ("shown as text, the master decides").
func TestEffectsCoverLevels1To5(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	for _, key := range sortedKeys(c.features) {
		if f := c.features[key]; f.Level <= 5 && len(c.effects[key]) == 0 {
			t.Errorf("%s (level %d) has no effects entry", key, f.Level)
		}
	}
	for _, key := range sortedKeys(c.traits) {
		if len(c.effects[key]) == 0 {
			t.Errorf("%s has no effects entry", key)
		}
	}
	for _, b := range c.backgrounds {
		if len(c.effects[b.Feature.Key]) == 0 {
			t.Errorf("%s has no effects entry", b.Feature.Key)
		}
	}
	for key := range c.classes {
		if _, ok := c.casting[key]; !ok && slices.Contains([]string{"class:bard", "class:cleric", "class:druid", "class:paladin", "class:ranger", "class:sorcerer", "class:warlock", "class:wizard"}, key) {
			t.Errorf("%s casts spells but has no spellcasting effect", key)
		}
	}
}

// TestNamesPT checks that everything the sheet and the editor name has our
// Portuguese name.
func TestNamesPT(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	var keys []string
	for _, a := range AllAbilities() {
		keys = append(keys, string(a))
	}
	for _, m := range []map[string]bool{
		keySet(c.skills), keySet(c.races), keySet(c.subraces), keySet(c.classes), keySet(c.subclasses),
		keySet(c.backgrounds), keySet(c.equipment), keySet(c.spells), keySet(c.languages), keySet(c.traits),
		keySet(c.named),
	} {
		keys = append(keys, sortedKeys(m)...)
	}
	for _, key := range sortedKeys(c.features) {
		if c.features[key].Level <= 5 {
			keys = append(keys, key)
		}
	}
	for _, key := range sortedKeys(c.proficiencies) {
		if p := c.proficiencies[key]; p.Kind == "armor" || p.Kind == "weapon" {
			keys = append(keys, key)
		}
	}
	for _, s := range senses {
		keys = append(keys, "sense:"+s)
	}
	for _, key := range keys {
		if _, ok := c.namesPT[key]; !ok {
			t.Errorf("%s has no Portuguese name in effects/names_pt.json", key)
		}
	}
}

func keySet[T any](m map[string]T) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// TestReferences checks that every key the snapshot points at exists, so
// the engine never has to guess. 5e-database is a community compilation;
// a broken reference in a newer commit shows up here, not on a sheet.
func TestReferences(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	check := func(from, key string) {
		t.Helper()
		if !c.exists(key) {
			t.Errorf("%s points at unknown %s", from, key)
		}
	}
	for _, r := range c.races {
		for _, k := range slices.Concat(r.Traits, r.Subraces, r.Languages) {
			check(r.Key, k)
		}
	}
	for _, s := range c.subraces {
		check(s.Key, s.Race)
		for _, k := range s.Traits {
			check(s.Key, k)
		}
	}
	for _, tr := range c.traits {
		for _, k := range slices.Concat(tr.Proficiencies, tr.ProficiencyOptions, tr.Options) {
			check(tr.Key, k)
		}
	}
	for _, cl := range c.classes {
		for _, k := range slices.Concat(cl.Proficiencies, cl.SkillChoices.From, cl.Subclasses, cl.Multiclass.Proficiencies) {
			check(cl.Key, k)
		}
		if cl.SubclassLevel < 1 {
			t.Errorf("%s has no subclass level", cl.Key)
		}
	}
	for _, rows := range c.classLevels {
		for _, row := range rows {
			for _, k := range row.Features {
				check(row.Class, k)
			}
		}
	}
	for _, rows := range c.subclassLevels {
		for _, row := range rows {
			for _, k := range row.Features {
				check(row.Subclass, k)
			}
		}
	}
	for _, s := range c.subclasses {
		check(s.Key, s.Class)
		for _, sp := range s.Spells {
			check(s.Key, sp.Spell)
		}
	}
	for _, f := range c.features {
		for _, k := range f.Options {
			check(f.Key, k)
		}
	}
	for _, b := range c.backgrounds {
		for _, k := range slices.Concat(b.Skills, b.Proficiencies) {
			check(b.Key, k)
		}
	}
	for _, s := range c.spells {
		check(s.Key, s.School)
		for _, k := range s.Classes {
			check(s.Key, k)
		}
	}
	for _, e := range c.equipment {
		if w := e.Weapon; w != nil {
			if w.DamageType != "" { // the net deals no damage
				check(e.Key, w.DamageType)
			}
			for _, k := range w.Properties {
				check(e.Key, k)
			}
		}
	}
}

// TestCatalog checks what the editor gets.
func TestCatalog(t *testing.T) {
	t.Parallel()
	cat := loadForTest(t).Catalog()
	counts := map[string][2]int{
		"races":       {len(cat.Races), 9},
		"subraces":    {len(cat.Subraces), 4},
		"classes":     {len(cat.Classes), 12},
		"subclasses":  {len(cat.Subclasses), 12},
		"backgrounds": {len(cat.Backgrounds), 1},
		"skills":      {len(cat.Skills), 18},
		"armor":       {len(cat.Armor), 12},
		"weapons":     {len(cat.Weapons), 37},
		"spells":      {len(cat.Spells), 319},
		"abilities":   {len(cat.Abilities), 6},
	}
	for name, n := range counts {
		if n[0] != n[1] {
			t.Errorf("Catalog has %d %s, want %d", n[0], name, n[1])
		}
	}
	for _, a := range cat.Armor {
		if a.Category == "shield" {
			t.Error("Catalog.Armor must not list the shield")
		}
	}
	if !slices.IsSortedFunc(cat.Spells, func(a, b SpellEntry) int { return comparePT(a.NamePT, b.NamePT) }) {
		t.Error("Catalog.Spells is not sorted by Portuguese name")
	}
	want := map[string]string{
		"class:wizard": PreparationSpellbook, "class:cleric": PreparationPrepared, "class:sorcerer": PreparationKnown,
		"class:warlock": PreparationKnown, "class:paladin": PreparationPrepared, "class:fighter": "",
	}
	for _, cl := range cat.Classes {
		if w, ok := want[cl.Key]; ok && cl.SpellPreparation != w {
			t.Errorf("%s SpellPreparation = %q, want %q", cl.Key, cl.SpellPreparation, w)
		}
		if cl.Key == "class:paladin" && cl.SpellcastingLevel != 2 {
			t.Errorf("paladin SpellcastingLevel = %d, want 2", cl.SpellcastingLevel)
		}
		if cl.Key == "class:wizard" && (cl.SubclassLevel != 2 || cl.HitDie != 6 || cl.SkillChoices != 2 || cl.NamePT != "Mago") {
			t.Errorf("wizard entry = %+v", cl)
		}
	}
	for _, s := range cat.Spells {
		if s.Key == "spell:fire-bolt" && (s.School != "school:evocation" || s.SchoolNamePT != "Evocação" || s.Level != 0) {
			t.Errorf("fire bolt entry = %+v", s)
		}
	}
	if cat.Abilities[3].AbbreviationPT != "INT" || cat.Abilities[4].AbbreviationPT != "SAB" || cat.Abilities[0].NamePT != "Força" {
		t.Errorf("abilities = %+v", cat.Abilities)
	}
}
