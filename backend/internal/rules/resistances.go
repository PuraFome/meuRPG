package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// ResistanceWhile says when a resistance holds. "" is always (a race's); "rage" is
// while the character is raging (the barbarian's).
const (
	ResistanceAlways    = ""
	ResistanceWhileRage = "rage"
)

// Resistance is a damage resistance that a feature or a trait gives (SRD 5.1,
// "Damage Resistance and Vulnerability"): the damage types it halves, and when.
type Resistance struct {
	// Source is the feature or trait key, such as "trait:hellish-resistance", and
	// NamePT its Portuguese name.
	Source, NamePT string
	// DamageTypes are the damage type keys it covers, "damage-type:fire".
	DamageTypes []string
	// While is ResistanceAlways or ResistanceWhileRage.
	While string
}

// damageResistanceFile is effects/damage_resistances.json.
type damageResistanceFile struct {
	Source      string `json:"source"`
	Resistances map[string]struct {
		DamageTypes []string `json:"damage_types"`
		While       string   `json:"while"`
	} `json:"resistances"`
}

// loadDamageResistances reads and checks effects/damage_resistances.json: every
// key is a feature or a trait of the content, every damage type exists, "while"
// is closed, and the file says where its rules come from.
func (c *content) loadDamageResistances(fsys fs.FS) error {
	var f damageResistanceFile
	if err := readJSON(fsys, "effects/damage_resistances.json", &f); err != nil {
		return err
	}
	if strings.TrimSpace(f.Source) == "" {
		return fmt.Errorf("effects/damage_resistances.json: source must say where the rules come from")
	}
	c.resistances = map[string]Resistance{}
	for _, key := range sortedKeys(f.Resistances) {
		in := f.Resistances[key]
		fail := func(format string, a ...any) error {
			return fmt.Errorf("effects/damage_resistances.json: %s: %s", key, fmt.Sprintf(format, a...))
		}
		if !strings.HasPrefix(key, "feature:") && !strings.HasPrefix(key, "trait:") || !c.exists(key) {
			return fail("not a feature or a trait of the content")
		}
		if len(in.DamageTypes) == 0 {
			return fail("no damage type")
		}
		for _, t := range in.DamageTypes {
			if !strings.HasPrefix(t, "damage-type:") || !c.exists(t) {
				return fail("%q is not a damage type", t)
			}
		}
		if in.While != ResistanceAlways && in.While != ResistanceWhileRage {
			return fail("while must be empty or %q", ResistanceWhileRage)
		}
		c.resistances[key] = Resistance{Source: key, NamePT: c.namePT(key), DamageTypes: slices.Clone(in.DamageTypes), While: in.While}
	}
	return nil
}

// Resistances lists the damage resistances the character's features and traits
// give, in the order of its features. A resistance that holds only while raging is
// listed too: the play module applies it while the rage lasts.
func (c *Content) Resistances(d Derived) []Resistance {
	var out []Resistance
	for _, f := range d.Features {
		if r, ok := c.c.resistances[f.Key]; ok {
			out = append(out, r)
		}
	}
	return out
}

// HasFeature says whether the character has a feature or a trait.
func HasFeature(d Derived, key string) bool {
	return slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == key })
}

// LevelIn is the character's level in a class ("class:barbarian"), 0 without it.
func LevelIn(d Derived, classKey string) int {
	for _, cl := range d.Classes {
		if cl.ClassKey == classKey {
			return cl.Level
		}
	}
	return 0
}
