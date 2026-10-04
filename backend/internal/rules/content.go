package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// content is the loaded rules content, indexed by key. It is built once by
// load and never changed afterwards, so Derive can read it from many
// requests at once.
type content struct {
	version  string
	manifest srd51.Manifest

	abilities     map[Ability]srd51.AbilityScore
	skills        map[string]*srd51.Skill
	skillOrder    []string
	races         map[string]*srd51.Race
	subraces      map[string]*srd51.Subrace
	traits        map[string]*srd51.Trait
	classes       map[string]*srd51.Class
	subclasses    map[string]*srd51.Subclass
	features      map[string]*srd51.Feature
	backgrounds   map[string]*srd51.Background
	proficiencies map[string]*srd51.Proficiency
	equipment     map[string]*srd51.Equipment
	spells        map[string]*srd51.Spell
	languages     map[string]*srd51.Language
	named         map[string]*srd51.Named

	// classLevels[class][n-1] is row n of the class table, and
	// subclassLevels[subclass][n] the subclass row at class level n.
	classLevels    map[string][]*srd51.Level
	subclassLevels map[string]map[int]*srd51.Level

	// effects are the hand-written effects, by the key they belong to.
	effects map[string][]*Effect
	// spellEffects are the spells that read hit points, by spell key
	// (effects/spells.json).
	spellEffects map[string]spellEffectDef
	// traps are the trap presets and the SRD's severity tables
	// (effects/traps.json), and lights the light presets (effects/lights.json).
	traps  traps
	lights []LightPreset
	// standardActions are the actions every character has.
	standardActions []Action
	// levelXP[n-1] is the XP to reach level n, and ratings the SRD's challenge
	// ratings with their XP (effects/advancement.json).
	levelXP []int
	ratings []ChallengeRating
	// casting is each casting class's spellcasting effect, and the class
	// level it starts at.
	casting map[string]classCasting

	namesPT map[string]string
	namesEN map[string]string

	compiler *formula.Compiler
	catalog  Catalog
	// spellEntries are the Catalog's spells by key.
	spellEntries map[string]SpellEntry
	// spellDetails are the structured details of each spell, by key.
	spellDetails map[string]*SpellDetails
	// optionParents maps an option that 5e-database lists only in its
	// parent's options (no "parent" field) to that parent.
	optionParents map[string]string
}

type classCasting struct {
	effect *Effect
	level  int
}

// abilityIndex maps each ability to its position on the sheet.
var abilityIndex = map[Ability]int{STR: 0, DEX: 1, CON: 2, INT: 3, WIS: 4, CHA: 5}

// effectsRevision is effects/revision.json: the n of "fx.<n>" and the hash
// of every other file in effects/ when n was set.
type effectsRevision struct {
	Revision int    `json:"revision"`
	SHA256   string `json:"sha256"`
}

func loadSRD() (*Content, error) {
	c, err := load(srd51.Files)
	if err != nil {
		return nil, fmt.Errorf("rules: loading the SRD 5.1 content: %w", err)
	}
	return &Content{c: c}, nil
}

// load reads a snapshot (data/*.json) and its effects (effects/*.json) from
// fsys, indexes them and compiles every formula.
func load(fsys fs.FS) (*content, error) {
	c := &content{
		abilities:      map[Ability]srd51.AbilityScore{},
		skills:         map[string]*srd51.Skill{},
		races:          map[string]*srd51.Race{},
		subraces:       map[string]*srd51.Subrace{},
		traits:         map[string]*srd51.Trait{},
		classes:        map[string]*srd51.Class{},
		subclasses:     map[string]*srd51.Subclass{},
		features:       map[string]*srd51.Feature{},
		backgrounds:    map[string]*srd51.Background{},
		proficiencies:  map[string]*srd51.Proficiency{},
		equipment:      map[string]*srd51.Equipment{},
		spells:         map[string]*srd51.Spell{},
		languages:      map[string]*srd51.Language{},
		named:          map[string]*srd51.Named{},
		classLevels:    map[string][]*srd51.Level{},
		subclassLevels: map[string]map[int]*srd51.Level{},
		effects:        map[string][]*Effect{},
		casting:        map[string]classCasting{},
		namesPT:        map[string]string{},
		namesEN:        map[string]string{},
		optionParents:  map[string]string{},
	}
	if err := readJSON(fsys, "data/manifest.json", &c.manifest); err != nil {
		return nil, err
	}
	if c.manifest.SnapshotVersion == "" {
		return nil, fmt.Errorf("data/manifest.json has no snapshot_version")
	}
	if err := c.loadData(fsys); err != nil {
		return nil, err
	}
	if err := c.indexLevels(fsys); err != nil {
		return nil, err
	}
	if err := c.applyCorrections(fsys); err != nil {
		return nil, err
	}

	var rev effectsRevision
	if err := readJSON(fsys, "effects/revision.json", &rev); err != nil {
		return nil, err
	}
	if rev.Revision < 1 {
		return nil, fmt.Errorf("effects/revision.json: revision must be at least 1")
	}
	c.version = c.manifest.SnapshotVersion + "+fx." + strconv.Itoa(rev.Revision)

	var names struct {
		Names map[string]string `json:"names"`
	}
	if err := readJSON(fsys, "effects/names_pt.json", &names); err != nil {
		return nil, err
	}
	for k, v := range names.Names {
		if !c.exists(k) && !strings.HasPrefix(k, "sense:") && !strings.HasPrefix(k, "resource:") && !strings.HasPrefix(k, "trap:") && !strings.HasPrefix(k, "light:") {
			return nil, fmt.Errorf("effects/names_pt.json: unknown key %q", k)
		}
		c.namesPT[k] = v
	}

	classIndexes := make([]string, 0, len(c.classes))
	for k := range c.classes {
		classIndexes = append(classIndexes, strings.TrimPrefix(k, "class:"))
	}
	slices.Sort(classIndexes)
	c.compiler = formula.NewCompiler(classIndexes)
	if err := c.loadEffects(fsys); err != nil {
		return nil, err
	}
	if err := c.loadStandardActions(fsys); err != nil {
		return nil, err
	}
	if err := c.loadAdvancement(fsys); err != nil {
		return nil, err
	}
	if err := c.loadSpellEffects(fsys); err != nil {
		return nil, err
	}
	if err := c.loadTraps(fsys); err != nil {
		return nil, err
	}
	if err := c.loadLights(fsys); err != nil {
		return nil, err
	}
	if err := c.indexCasting(); err != nil {
		return nil, err
	}
	c.buildCatalog()
	return c, nil
}

func readJSON(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// index reads a data file into a map by key, refusing duplicate keys.
func index[T any](fsys fs.FS, name string, key func(*T) string, into map[string]*T, namesEN map[string]string, nameOf func(*T) string) error {
	var rows []T
	if err := readJSON(fsys, "data/"+name, &rows); err != nil {
		return err
	}
	for i := range rows {
		r := &rows[i]
		k := key(r)
		if _, dup := into[k]; dup {
			return fmt.Errorf("data/%s: duplicate key %q", name, k)
		}
		into[k] = r
		namesEN[k] = nameOf(r)
	}
	return nil
}

func (c *content) loadData(fsys fs.FS) error {
	var abilities []srd51.AbilityScore
	if err := readJSON(fsys, "data/abilities.json", &abilities); err != nil {
		return err
	}
	for _, a := range abilities {
		if _, ok := abilityIndex[Ability(a.Key)]; !ok {
			return fmt.Errorf("data/abilities.json: unknown ability %q", a.Key)
		}
		c.abilities[Ability(a.Key)] = a
		c.namesEN[a.Key] = a.FullName
	}
	if len(c.abilities) != len(abilityIndex) {
		return fmt.Errorf("data/abilities.json: want %d abilities, got %d", len(abilityIndex), len(c.abilities))
	}

	errs := []error{
		index(fsys, "skills.json", func(s *srd51.Skill) string { return s.Key }, c.skills, c.namesEN, func(s *srd51.Skill) string { return s.Name }),
		index(fsys, "races.json", func(r *srd51.Race) string { return r.Key }, c.races, c.namesEN, func(r *srd51.Race) string { return r.Name }),
		index(fsys, "subraces.json", func(r *srd51.Subrace) string { return r.Key }, c.subraces, c.namesEN, func(r *srd51.Subrace) string { return r.Name }),
		index(fsys, "traits.json", func(r *srd51.Trait) string { return r.Key }, c.traits, c.namesEN, func(r *srd51.Trait) string { return r.Name }),
		index(fsys, "classes.json", func(r *srd51.Class) string { return r.Key }, c.classes, c.namesEN, func(r *srd51.Class) string { return r.Name }),
		index(fsys, "subclasses.json", func(r *srd51.Subclass) string { return r.Key }, c.subclasses, c.namesEN, func(r *srd51.Subclass) string { return r.Name }),
		index(fsys, "features.json", func(r *srd51.Feature) string { return r.Key }, c.features, c.namesEN, func(r *srd51.Feature) string { return r.Name }),
		index(fsys, "backgrounds.json", func(r *srd51.Background) string { return r.Key }, c.backgrounds, c.namesEN, func(r *srd51.Background) string { return r.Name }),
		index(fsys, "proficiencies.json", func(r *srd51.Proficiency) string { return r.Key }, c.proficiencies, c.namesEN, func(r *srd51.Proficiency) string { return r.Name }),
		index(fsys, "equipment.json", func(r *srd51.Equipment) string { return r.Key }, c.equipment, c.namesEN, func(r *srd51.Equipment) string { return r.Name }),
		index(fsys, "spells.json", func(r *srd51.Spell) string { return r.Key }, c.spells, c.namesEN, func(r *srd51.Spell) string { return r.Name }),
		index(fsys, "languages.json", func(r *srd51.Language) string { return r.Key }, c.languages, c.namesEN, func(r *srd51.Language) string { return r.Name }),
	}
	for _, name := range []string{"damage-types.json", "magic-schools.json", "weapon-properties.json", "conditions.json"} {
		errs = append(errs, index(fsys, name, func(r *srd51.Named) string { return r.Key }, c.named, c.namesEN, func(r *srd51.Named) string { return r.Name }))
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	for _, b := range c.backgrounds {
		c.namesEN[b.Feature.Key] = b.Feature.Name
	}
	for k := range c.skills {
		c.skillOrder = append(c.skillOrder, k)
	}
	slices.Sort(c.skillOrder)
	for _, k := range sortedKeys(c.features) {
		for _, o := range c.features[k].Options {
			if f, ok := c.features[o]; ok && f.Parent == "" {
				c.optionParents[o] = k
			}
		}
	}
	return nil
}

func (c *content) indexLevels(fsys fs.FS) error {
	var rows []srd51.Level
	if err := readJSON(fsys, "data/levels.json", &rows); err != nil {
		return err
	}
	for i := range rows {
		r := &rows[i]
		if r.Level < 1 || r.Level > MaxLevel {
			return fmt.Errorf("data/levels.json: %s level %d out of range", r.Class, r.Level)
		}
		if r.Subclass != "" {
			if c.subclassLevels[r.Subclass] == nil {
				c.subclassLevels[r.Subclass] = map[int]*srd51.Level{}
			}
			c.subclassLevels[r.Subclass][r.Level] = r
			continue
		}
		if c.classLevels[r.Class] == nil {
			c.classLevels[r.Class] = make([]*srd51.Level, MaxLevel)
		}
		c.classLevels[r.Class][r.Level-1] = r
	}
	for key := range c.classes {
		rows := c.classLevels[key]
		if len(rows) != MaxLevel || slices.Contains(rows, nil) {
			return fmt.Errorf("data/levels.json: class %s does not have all %d levels", key, MaxLevel)
		}
	}
	return nil
}

// loadEffects reads every effects file except names_pt.json, revision.json,
// standard_actions.json, advancement.json, spells.json, traps.json and lights.json (tables, not effects), checks
// and compiles each effect.
func (c *content) loadEffects(fsys fs.FS) error {
	files, err := fs.Glob(fsys, "effects/*.json")
	if err != nil {
		return err
	}
	for _, name := range files {
		switch path.Base(name) {
		case "names_pt.json", "revision.json", "standard_actions.json", "advancement.json", "spells.json", "corrections.json", "traps.json", "lights.json":
			continue
		}
		var f struct {
			Effects map[string][]*Effect `json:"effects"`
		}
		if err := readJSON(fsys, name, &f); err != nil {
			return err
		}
		keys := make([]string, 0, len(f.Effects))
		for k := range f.Effects {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, key := range keys {
			if _, dup := c.effects[key]; dup {
				return fmt.Errorf("%s: effects for %q are also in another file", name, key)
			}
			if !c.effectOwnerExists(key) {
				return fmt.Errorf("%s: effects for unknown key %q", name, key)
			}
			for _, e := range f.Effects[key] {
				if err := c.compileEffect(key, e); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
			c.effects[key] = f.Effects[key]
		}
	}
	return nil
}

// effectOwnerExists says whether effects may hang on key: a feature, a
// trait, a background or its feature, a race, subrace, class or subclass.
func (c *content) effectOwnerExists(key string) bool {
	switch {
	case strings.HasPrefix(key, "background-feature:"):
		for _, b := range c.backgrounds {
			if b.Feature.Key == key {
				return true
			}
		}
		return false
	case strings.HasPrefix(key, "feature:"), strings.HasPrefix(key, "trait:"),
		strings.HasPrefix(key, "background:"), strings.HasPrefix(key, "race:"),
		strings.HasPrefix(key, "subrace:"), strings.HasPrefix(key, "class:"),
		strings.HasPrefix(key, "subclass:"):
		return c.exists(key)
	}
	return false
}

// indexCasting finds, for each class, the feature with the spellcasting
// effect and the class level it comes at.
func (c *content) indexCasting() error {
	for classKey, rows := range c.classLevels {
		for _, row := range rows {
			for _, fk := range row.Features {
				for _, e := range c.effects[fk] {
					if e.Type != "spellcasting" {
						continue
					}
					if _, dup := c.casting[classKey]; dup {
						return fmt.Errorf("class %s has two spellcasting effects", classKey)
					}
					c.casting[classKey] = classCasting{effect: e, level: row.Level}
				}
			}
		}
	}
	return nil
}

// exists says whether key is any known content key.
func (c *content) exists(key string) bool {
	if _, ok := abilityIndex[Ability(key)]; ok {
		return true
	}
	_, ok := c.namesEN[key]
	return ok
}

// namePT is the Portuguese name of key, or its English name, or "".
func (c *content) namePT(key string) string {
	if n, ok := c.namesPT[key]; ok {
		return n
	}
	return c.namesEN[key]
}

// ability returns the Ability for an index such as "int", and whether it
// is one.
func ability(index string) (Ability, bool) {
	a := Ability(index)
	_, ok := abilityIndex[a]
	return a, ok
}

func abilityMap(m map[string]int) map[Ability]int {
	out := make(map[Ability]int, len(m))
	for k, v := range m {
		if a, ok := ability(k); ok {
			out[a] = v
		}
	}
	return out
}

func (c *content) buildCatalog() {
	cat := Catalog{ContentVersion: c.version, Attribution: srd51.Attribution}
	for _, k := range sortedKeys(c.races) {
		r := c.races[k]
		cat.Races = append(cat.Races, RaceEntry{
			Key: k, Name: r.Name, NamePT: c.namePT(k), SpeedFt: r.SpeedFt, Size: r.Size,
			AbilityBonuses: abilityMap(r.AbilityBonuses), Subraces: r.Subraces,
		})
	}
	for _, k := range sortedKeys(c.subraces) {
		s := c.subraces[k]
		cat.Subraces = append(cat.Subraces, SubraceEntry{
			Key: k, Name: s.Name, NamePT: c.namePT(k), Race: s.Race, AbilityBonuses: abilityMap(s.AbilityBonuses),
		})
	}
	for _, k := range sortedKeys(c.classes) {
		cl := c.classes[k]
		e := ClassEntry{
			Key: k, Name: cl.Name, NamePT: c.namePT(k), HitDie: cl.HitDie,
			SkillChoices: cl.SkillChoices.Choose, SkillOptions: cl.SkillChoices.From,
			SubclassLevel: cl.SubclassLevel, Subclasses: cl.Subclasses,
		}
		for _, s := range cl.SavingThrows {
			if a, ok := ability(s); ok {
				e.SavingThrows = append(e.SavingThrows, a)
			}
		}
		if cast, ok := c.casting[k]; ok {
			e.SpellcastingAbility = Ability(cast.effect.Ability)
			e.PreparesSpells = cast.effect.Prepares
			e.SpellPreparation = preparation(cast.effect)
			e.SpellcastingLevel = cast.level
			for _, row := range c.classLevels[k] {
				highest := 0
				if row != nil && row.Spellcasting != nil {
					highest = MaxSpellLevelFromSlots(row.Spellcasting.Slots)
				}
				e.MaxSpellLevelByLevel = append(e.MaxSpellLevelByLevel, highest)
			}
		}
		cat.Classes = append(cat.Classes, e)
	}
	for _, k := range sortedKeys(c.subclasses) {
		s := c.subclasses[k]
		cat.Subclasses = append(cat.Subclasses, SubclassEntry{Key: k, Name: s.Name, NamePT: c.namePT(k), Class: s.Class})
	}
	for _, k := range sortedKeys(c.backgrounds) {
		b := c.backgrounds[k]
		cat.Backgrounds = append(cat.Backgrounds, BackgroundEntry{Key: k, Name: b.Name, NamePT: c.namePT(k), SkillProficiencies: b.Skills})
	}
	for _, k := range c.skillOrder {
		s := c.skills[k]
		cat.Skills = append(cat.Skills, SkillEntry{Key: k, Name: s.Name, NamePT: c.namePT(k), Ability: Ability(s.Ability)})
	}
	for _, k := range sortedKeys(c.equipment) {
		e := c.equipment[k]
		switch {
		case e.Armor != nil && e.Armor.Category != "shield":
			a := e.Armor
			cat.Armor = append(cat.Armor, ArmorEntry{
				Key: k, Name: e.Name, NamePT: c.namePT(k), Category: a.Category, BaseAC: a.BaseAC,
				DexBonus: a.DexBonus, MaxDexBonus: a.MaxDexBonus, StrMinimum: a.StrMinimum,
				StealthDisadvantage: a.StealthDisadvantage,
			})
		case e.Weapon != nil:
			w := e.Weapon
			normal, long := w.NormalRangeFt, w.LongRangeFt
			if w.ThrowNormalFt > 0 {
				normal, long = w.ThrowNormalFt, w.ThrowLongFt
			}
			if w.Range == "melee" && w.ThrowNormalFt == 0 {
				normal, long = 0, 0
			}
			cat.Weapons = append(cat.Weapons, WeaponEntry{
				Key: k, Name: e.Name, NamePT: c.namePT(k), Category: w.Category, Range: w.Range,
				Damage: w.Damage, DamageType: w.DamageType, TwoHandedDamage: w.TwoHandedDamage,
				Properties: w.Properties, NormalRangeFt: normal, LongRangeFt: long,
			})
		}
	}
	c.spellEntries = map[string]SpellEntry{}
	c.spellDetails = map[string]*SpellDetails{}
	for _, k := range sortedKeys(c.spells) {
		s := c.spells[k]
		e := SpellEntry{
			Key: k, Name: s.Name, NamePT: c.namePT(k), Level: s.Level,
			School: s.School, SchoolNamePT: c.namePT(s.School),
			Classes: s.Classes, Ritual: s.Ritual, Concentration: s.Concentration,
			CastingTime: parseCastingTime(s.CastingTime),
		}
		cat.Spells = append(cat.Spells, e)
		c.spellEntries[k] = e
		c.spellDetails[k] = c.buildSpellDetails(s, e)
	}
	for _, a := range AllAbilities() {
		cat.Abilities = append(cat.Abilities, AbilityEntry{
			Ability: a, Name: c.abilities[a].FullName, NamePT: c.namePT(string(a)), AbbreviationPT: abbreviationPT[a],
		})
	}

	// Every list but the abilities is sorted by Portuguese name, as the
	// editor shows it.
	sortPT(cat.Races, func(e RaceEntry) string { return e.NamePT })
	sortPT(cat.Subraces, func(e SubraceEntry) string { return e.NamePT })
	sortPT(cat.Classes, func(e ClassEntry) string { return e.NamePT })
	sortPT(cat.Subclasses, func(e SubclassEntry) string { return e.NamePT })
	sortPT(cat.Backgrounds, func(e BackgroundEntry) string { return e.NamePT })
	sortPT(cat.Skills, func(e SkillEntry) string { return e.NamePT })
	sortPT(cat.Armor, func(e ArmorEntry) string { return e.NamePT })
	sortPT(cat.Weapons, func(e WeaponEntry) string { return e.NamePT })
	sortPT(cat.Spells, func(e SpellEntry) string { return e.NamePT })
	cat.ChallengeRatings = slices.Clone(c.ratings)
	c.catalog = cat
}

func sortPT[T any](s []T, name func(T) string) {
	slices.SortStableFunc(s, func(a, b T) int { return comparePT(name(a), name(b)) })
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// correctionFields are the class table columns effects/corrections.json may
// correct. The set is closed.
var correctionFields = []string{"invocations_known"}

// applyCorrections reads effects/corrections.json and writes its numbers over
// the class table rows of the snapshot (data/ is never edited by hand, so a
// number the snapshot has wrong against the SRD 5.1 is fixed here). It refuses
// an unknown class, field or level, and a row without the column.
func (c *content) applyCorrections(fsys fs.FS) error {
	const name = "effects/corrections.json"
	var f struct {
		Comment     string `json:"_comment"`
		Corrections []struct {
			Class   string         `json:"class"`
			Field   string         `json:"field"`
			Source  string         `json:"source"`
			ByLevel map[string]int `json:"by_level"`
		} `json:"corrections"`
	}
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	for _, corr := range f.Corrections {
		rows, ok := c.classLevels[corr.Class]
		if !ok {
			return fmt.Errorf("%s: unknown class %q", name, corr.Class)
		}
		if !slices.Contains(correctionFields, corr.Field) {
			return fmt.Errorf("%s: %s: field %q cannot be corrected", name, corr.Class, corr.Field)
		}
		for lvl, v := range corr.ByLevel {
			n, err := strconv.Atoi(lvl)
			if err != nil || n < 1 || n > MaxLevel {
				return fmt.Errorf("%s: %s: level %q is not 1 to %d", name, corr.Class, lvl, MaxLevel)
			}
			row := rows[n-1]
			cols := map[string]json.RawMessage{}
			if err := json.Unmarshal(row.ClassSpecific, &cols); err != nil {
				return fmt.Errorf("%s: %s level %d has no class_specific columns: %w", name, corr.Class, n, err)
			}
			if _, has := cols[corr.Field]; !has {
				return fmt.Errorf("%s: %s level %d has no column %q", name, corr.Class, n, corr.Field)
			}
			cols[corr.Field] = json.RawMessage(strconv.Itoa(v))
			raw, err := json.Marshal(cols)
			if err != nil {
				return err
			}
			row.ClassSpecific = raw
		}
	}
	return nil
}
