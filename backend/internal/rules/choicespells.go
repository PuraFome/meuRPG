package rules

import (
	"slices"
	"strings"
)

// SpellSource is where a casting class's spell picker finds the spells that are not
// on the class list (PM-05): the patron's expanded list and the Bard's Magical
// Secrets.
type SpellSource struct {
	// Class is the class key.
	Class string
	// Patron are the spells the subclass adds to the class list, with the class level
	// they come at (The Fiend's Expanded Spell List, SRD 5.1).
	Patron []PatronSpell
	// Secrets is how many known spells may come from any class's list: 2 for each of
	// the Bard's Magical Secrets (levels 10, 14 and 18) and 2 for the College of Lore's
	// Additional Magical Secrets (level 6). SecretsBeyondKnown is how many of them are
	// beyond the number of spells known (Lore's). SecretsMaxSpellLevel is the highest
	// circle of a secret: the highest the class casts.
	Secrets, SecretsBeyondKnown, SecretsMaxSpellLevel int
}

// PatronSpell is a spell the patron adds to the class list.
type PatronSpell struct {
	Spell      string
	ClassLevel int
}

// secretsPerFeature is how many spells each Magical Secrets feature gives (SRD 5.1,
// Bard, Magical Secrets; Lore, Additional Magical Secrets).
const secretsPerFeature = 2

// SpellSources answers, for each class of the Build that casts, the spells outside its
// list its picker offers.
func (c *Content) SpellSources(b Build) []SpellSource {
	cc := c.c
	x := &deriver{b: b, c: cc, d: &Derived{}, proficient: map[string]bool{}, conditions: map[*Effect]bool{}}
	x.resolve()
	d := derive(b, cc)
	var out []SpellSource
	for _, oc := range x.classes {
		sc := spellcastingOf(d, oc.key)
		if sc == nil {
			continue
		}
		src := SpellSource{Class: oc.key}
		if oc.subclass != nil && oc.subclass.ExpandedList {
			for _, ss := range oc.subclass.Spells {
				if ss.ClassLevel <= oc.level {
					src.Patron = append(src.Patron, PatronSpell{Spell: ss.Spell, ClassLevel: ss.ClassLevel})
				}
			}
		}
		for lvl := 1; lvl <= oc.level; lvl++ {
			classFeatures, subFeatures := cc.levelFeatures(oc.key, oc.subclass, lvl)
			for _, fk := range slices.Concat(classFeatures, subFeatures) {
				switch {
				case strings.HasPrefix(fk, "feature:magical-secrets-") && !isTableKey(fk):
					src.Secrets += secretsPerFeature
				case fk == "feature:additional-magical-secrets":
					src.Secrets += secretsPerFeature
					src.SecretsBeyondKnown += secretsPerFeature
				}
			}
		}
		if src.Secrets > 0 {
			src.SecretsMaxSpellLevel = sc.MaxSpellLevel
		}
		if len(src.Patron) > 0 || src.Secrets > 0 {
			out = append(out, src)
		}
	}
	return out
}
