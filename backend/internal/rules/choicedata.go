package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// This file reads effects/choices.json, the hand-written data behind the
// choices a character makes at creation, on a locked sheet and at level-up
// (choicegroups.go): the Draconic Ancestry table, the prerequisites of the
// Eldritch Invocations, the Ranger's Favored Enemy types and Natural Explorer
// terrains, and the damage resistances some traits give. Every row names its
// SRD 5.1 source, and the loader refuses a row without one or a key the content
// does not have.

// The shapes of a breath weapon.
const (
	BreathLine = "line"
	BreathCone = "cone"
)

// The kinds of a prerequisite an option can have.
const (
	PrerequisiteLevel   = "level"
	PrerequisiteSpell   = "spell"
	PrerequisiteFeature = "feature"
)

// choiceData is the content of effects/choices.json, indexed.
type choiceData struct {
	// ancestry is the Draconic Ancestry table by the dragonborn's trait key and by
	// the sorcerer's Dragon Ancestor option key.
	ancestryByTrait    map[string]*draconicAncestry
	ancestryByFeature  map[string]*draconicAncestry
	breathTrait        string
	breathDice         []breathDice
	breathSource       string
	prerequisites      map[string]invocationPrerequisite
	enemyTypes         []string
	enemyHumanoid      string
	enemyHumanoidRaces int
	terrains           []string
	resistances        map[string]racialResistance
	// summaries are the one-line rules of the options (OptionSummaries).
	summaries map[string]string
}

type draconicAncestry struct {
	Color      string `json:"color"`
	Trait      string `json:"trait"`
	Feature    string `json:"dragon_ancestor"`
	DamageType string `json:"damage_type"`
	Shape      string `json:"shape"`
	SizeFt     int    `json:"size_ft"`
	WidthFt    int    `json:"width_ft"`
	Save       string `json:"save"`
	Source     string `json:"source"`
}

type breathDice struct {
	Level int    `json:"level"`
	Dice  string `json:"dice"`
}

// invocationPrerequisite is what an Eldritch Invocation asks of the warlock:
// any of a warlock level, a known cantrip and a Pact Boon.
type invocationPrerequisite struct {
	Invocation string `json:"invocation"`
	Level      int    `json:"level,omitempty"`
	Spell      string `json:"spell,omitempty"`
	Feature    string `json:"feature,omitempty"`
	Source     string `json:"source"`
}

type racialResistance struct {
	Trait      string `json:"trait"`
	DamageType string `json:"damage_type"`
	Source     string `json:"source"`
}

// choiceDataFile is the file effects/choices.json.
type choiceDataFile struct {
	Comment          string             `json:"_comment"`
	DraconicAncestry []draconicAncestry `json:"draconic_ancestry"`
	BreathWeapon     struct {
		Trait  string       `json:"trait"`
		Dice   []breathDice `json:"dice"`
		Source string       `json:"source"`
	} `json:"breath_weapon"`
	Invocations  []invocationPrerequisite `json:"invocation_prerequisites"`
	FavoredEnemy struct {
		Types         []string `json:"types"`
		Humanoid      string   `json:"humanoid"`
		HumanoidRaces int      `json:"humanoid_races"`
		Source        string   `json:"source"`
	} `json:"favored_enemy"`
	NaturalExplorer struct {
		Terrains []string `json:"terrains"`
		Source   string   `json:"source"`
	} `json:"natural_explorer"`
	Resistances []racialResistance `json:"racial_resistances"`
	Summaries   map[string]string  `json:"option_summaries"`
}

// loadChoiceData reads effects/choices.json and checks every key against the
// content. It must run after the effects (the invocations' keys are features).
//
//nolint:gocognit,gocyclo // one flat list of closed checks, one block per table of the file; splitting it would only scatter them
func (c *content) loadChoiceData(fsys fs.FS) error {
	const name = "effects/choices.json"
	var f choiceDataFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	d := &choiceData{
		ancestryByTrait:   map[string]*draconicAncestry{},
		ancestryByFeature: map[string]*draconicAncestry{},
		prerequisites:     map[string]invocationPrerequisite{},
		resistances:       map[string]racialResistance{},
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", name, fmt.Sprintf(format, args...))
	}
	isDamageType := func(k string) bool {
		n, ok := c.named[k]
		return ok && n != nil && strings.HasPrefix(k, "damage-type:")
	}

	for i := range f.DraconicAncestry {
		a := &f.DraconicAncestry[i]
		if a.Source == "" {
			return fail("draconic ancestry %q needs a source", a.Color)
		}
		if _, ok := c.traits[a.Trait]; !ok {
			return fail("draconic ancestry %q: unknown trait %q", a.Color, a.Trait)
		}
		if _, ok := c.features[a.Feature]; !ok {
			return fail("draconic ancestry %q: unknown feature %q", a.Color, a.Feature)
		}
		if !isDamageType(a.DamageType) {
			return fail("draconic ancestry %q: %q is not a damage type", a.Color, a.DamageType)
		}
		if a.Shape != BreathLine && a.Shape != BreathCone {
			return fail("draconic ancestry %q: shape %q is not line or cone", a.Color, a.Shape)
		}
		if a.SizeFt <= 0 || (a.Shape == BreathLine) != (a.WidthFt > 0) {
			return fail("draconic ancestry %q: a line has a size and a width, a cone only a size", a.Color)
		}
		if _, ok := ability(a.Save); !ok {
			return fail("draconic ancestry %q: %q is not an ability", a.Color, a.Save)
		}
		if d.ancestryByTrait[a.Trait] != nil || d.ancestryByFeature[a.Feature] != nil {
			return fail("draconic ancestry %q is listed twice", a.Color)
		}
		d.ancestryByTrait[a.Trait], d.ancestryByFeature[a.Feature] = a, a
	}

	if _, ok := c.traits[f.BreathWeapon.Trait]; !ok || f.BreathWeapon.Source == "" || len(f.BreathWeapon.Dice) == 0 {
		return fail("breath_weapon needs a known trait, dice and a source")
	}
	d.breathTrait, d.breathSource = f.BreathWeapon.Trait, f.BreathWeapon.Source
	for i, row := range f.BreathWeapon.Dice {
		if _, ok := ParseDice(row.Dice); !ok || row.Level < 1 || row.Level > MaxLevel || (i > 0 && row.Level <= f.BreathWeapon.Dice[i-1].Level) {
			return fail("breath_weapon dice %q at level %d: dice and ascending levels expected", row.Dice, row.Level)
		}
	}
	d.breathDice = f.BreathWeapon.Dice

	for _, p := range f.Invocations {
		if p.Source == "" {
			return fail("invocation %q needs a source", p.Invocation)
		}
		inv, ok := c.features[p.Invocation]
		if !ok || inv.Parent != invocationsFeature {
			return fail("%q is not an Eldritch Invocation", p.Invocation)
		}
		if _, dup := d.prerequisites[p.Invocation]; dup {
			return fail("invocation %q is listed twice", p.Invocation)
		}
		if p.Spell != "" {
			if s, ok := c.spells[p.Spell]; !ok || s.Level != 0 {
				return fail("invocation %q: %q is not a known cantrip", p.Invocation, p.Spell)
			}
		}
		if p.Feature != "" {
			if pf, ok := c.features[p.Feature]; !ok || pf.Parent != "feature:pact-boon" {
				return fail("invocation %q: %q is not a Pact Boon", p.Invocation, p.Feature)
			}
		}
		if p.Level < 0 || p.Level > MaxLevel {
			return fail("invocation %q: level %d is not 0 to %d", p.Invocation, p.Level, MaxLevel)
		}
		d.prerequisites[p.Invocation] = p
	}
	for _, key := range c.features[invocationsFeature].Options {
		if _, ok := d.prerequisites[key]; !ok {
			return fail("invocation %q has no row: list it even when it asks for nothing", key)
		}
	}

	fe := f.FavoredEnemy
	if fe.Source == "" || len(fe.Types) == 0 || fe.Humanoid == "" || fe.HumanoidRaces < 1 || hasDuplicate(fe.Types) {
		return fail("favored_enemy needs types, the humanoid key, a race count and a source")
	}
	d.enemyTypes, d.enemyHumanoid, d.enemyHumanoidRaces = fe.Types, fe.Humanoid, fe.HumanoidRaces
	for _, k := range append(slices.Clone(fe.Types), fe.Humanoid) {
		if !strings.HasPrefix(k, "creature-type:") {
			return fail("favored enemy %q is not a creature-type key", k)
		}
	}
	ne := f.NaturalExplorer
	if ne.Source == "" || len(ne.Terrains) == 0 || hasDuplicate(ne.Terrains) {
		return fail("natural_explorer needs terrains and a source")
	}
	for _, k := range ne.Terrains {
		if !strings.HasPrefix(k, "terrain:") {
			return fail("terrain %q is not a terrain key", k)
		}
	}
	d.terrains = ne.Terrains

	for _, r := range f.Resistances {
		if _, ok := c.traits[r.Trait]; !ok || !isDamageType(r.DamageType) || r.Source == "" {
			return fail("racial resistance of %q needs a known trait, a damage type and a source", r.Trait)
		}
		d.resistances[r.Trait] = r
	}

	// Every option a feature offers says its rule in a line, except the ones whose
	// line the engine writes (the dragon ancestors and the terrains of the Land).
	d.summaries = f.Summaries
	for k, text := range f.Summaries {
		if _, ok := c.features[k]; !ok || strings.TrimSpace(text) == "" {
			return fail("option summary %q: a known feature and a text expected", k)
		}
	}
	for _, parent := range c.features {
		for _, key := range parent.Options {
			if _, isAncestor := d.ancestryByFeature[key]; isAncestor || strings.HasPrefix(key, "feature:circle-of-the-land-") {
				continue
			}
			if _, ok := f.Summaries[key]; !ok {
				return fail("option %q has no summary", key)
			}
		}
	}

	// The names of the keys this file introduces.
	for _, k := range append(slices.Clone(d.enemyTypes), append([]string{d.enemyHumanoid}, d.terrains...)...) {
		if _, ok := c.namesPT[k]; !ok {
			return fail("%s has no Portuguese name in effects/names_pt.json", k)
		}
	}
	c.choices = d
	return nil
}

func hasDuplicate(keys []string) bool {
	for i, k := range keys {
		if slices.Contains(keys[:i], k) {
			return true
		}
	}
	return false
}

// subclassLevelCorrection adds features to a subclass's row of the class table:
// a feature the snapshot lists in no row (the Land druid's terrain).
type subclassLevelCorrection struct {
	Subclass    string   `json:"subclass"`
	Level       int      `json:"level"`
	AddFeatures []string `json:"add_features"`
	Source      string   `json:"source"`
}

// applySubclassLevelCorrections writes the features of effects/corrections.json's
// subclass_level_corrections over the subclasses' rows. It refuses an unknown
// subclass, level (the subclass has no row there) or feature, a feature the row
// already has, and a correction without a source.
func (c *content) applySubclassLevelCorrections(name string, list []subclassLevelCorrection) error {
	for _, corr := range list {
		sub, ok := c.subclasses[corr.Subclass]
		if !ok {
			return fmt.Errorf("%s: unknown subclass %q", name, corr.Subclass)
		}
		if corr.Source == "" || len(corr.AddFeatures) == 0 {
			return fmt.Errorf("%s: %s level %d needs features and a source", name, sub.Key, corr.Level)
		}
		row := c.subclassLevels[sub.Key][corr.Level]
		if row == nil {
			return fmt.Errorf("%s: %s has no row at level %d", name, sub.Key, corr.Level)
		}
		for _, fk := range corr.AddFeatures {
			if _, ok := c.features[fk]; !ok {
				return fmt.Errorf("%s: %s: unknown feature %q", name, sub.Key, fk)
			}
			if slices.Contains(row.Features, fk) {
				return fmt.Errorf("%s: %s level %d already has %s", name, sub.Key, corr.Level, fk)
			}
		}
		row.Features = append(slices.Clone(corr.AddFeatures), row.Features...)
	}
	return nil
}
