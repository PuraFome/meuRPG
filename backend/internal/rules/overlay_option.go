package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The table's options for the SRD's option sets (MR-025): an Eldritch Invocation, a
// Metamagic option, a fighting style, a Pact Boon... written by the master as an entry
// "option:<set>:<slug>@mesa". The option joins the Options of every SRD feature that
// offers the set, so the choice engine deals with it like an SRD option: the picks are
// shared in the pool of the set, the option is a feature once picked (its effects
// apply while the character owns an offering feature), and what it asks for is listed
// on it as prerequisites.

// optionSetDef is an option set a table option may join: its short name (the one in the
// key), the Portuguese name and the SRD features that offer it.
type optionSetDef struct {
	Name, NamePT, HintPT string
	Offerers             []string
}

// optionSetDefs are the sets, in the menu's order. A set whose options are data the engine
// reads by name (the Land druid's terrain, the draconic ancestry) is not here.
var optionSetDefs = []optionSetDef{
	{"invocation", "Invocação mística", "As invocações do bruxo.", []string{invocationsFeature}},
	{"pact_boon", "Dádiva do pacto", "A dádiva que o bruxo escolhe no 3º nível.", []string{pactBoonKey}},
	{"metamagic", "Metamagia", "As opções de Metamagia do feiticeiro.", []string{"feature:metamagic-1", "feature:metamagic-2", "feature:metamagic-3"}},
	{"fighting_style", "Estilo de luta", "Os estilos de luta do guerreiro, do paladino e do patrulheiro.", []string{
		"feature:fighter-fighting-style", "feature:additional-fighting-style", "feature:paladin-fighting-style", "feature:ranger-fighting-style",
	}},
	{"hunters_prey", "Presa do Caçador", "As opções de Presa do Caçador do patrulheiro.", []string{"feature:hunters-prey"}},
	{"defensive_tactics", "Táticas defensivas", "As opções de Táticas Defensivas do patrulheiro.", []string{"feature:defensive-tactics"}},
	{"multiattack", "Ataque múltiplo", "As opções de Ataque Múltiplo do patrulheiro.", []string{"feature:multiattack"}},
	{"superior_hunters_defense", "Defesa superior do caçador", "As opções de Defesa Superior do Caçador do patrulheiro.", []string{"feature:superior-hunters-defense"}},
}

// optionSetOf is the set with this name.
func optionSetOf(name string) (optionSetDef, bool) {
	i := slices.IndexFunc(optionSetDefs, func(d optionSetDef) bool { return d.Name == name })
	if i < 0 {
		return optionSetDef{}, false
	}
	return optionSetDefs[i], true
}

// OptionKeyPrefix starts the key of a table option.
const OptionKeyPrefix = "option:"

// featureKeyPrefix starts the key of a class, subclass or racial feature.
const featureKeyPrefix = "feature:"

// pactBoonKey is the warlock's Pact Boon, the parent of its options.
const pactBoonKey = "feature:pact-boon"

// MaxOptionRequires is the most options one option may require.
const MaxOptionRequires = 10

// TableOption is an option of one of the SRD's option sets. Key is
// "option:<set>:<slug>@mesa" and Set is the same set the key names.
type TableOption struct {
	TableEntry
	DescPT       []string
	Set          string
	Prerequisite TableOptionPrerequisite
	Effects      []Effect
}

// TableOptionPrerequisite is what a table option asks of the character; every one set must
// hold. The zero value asks nothing.
type TableOptionPrerequisite struct {
	// ClassKey and ClassLevel: a level in one class (1 to MaxLevel); both or neither.
	ClassKey   string
	ClassLevel int
	// RequiresOptions are options (the SRD's or the table's) the character has picked.
	RequiresOptions []string
	// RequiresPact is the Pact Boon the character has.
	RequiresPact string
	// RequiresSpell is a spell the character knows.
	RequiresSpell string
}

// IsZero says the option asks nothing.
func (p TableOptionPrerequisite) IsZero() bool {
	return p.ClassKey == "" && p.ClassLevel == 0 && len(p.RequiresOptions) == 0 && p.RequiresPact == "" && p.RequiresSpell == ""
}

// optionKeySet is the set a table option key names: "option:fighting_style:x@mesa".
func optionKeySet(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, OptionKeyPrefix)
	if !ok {
		return "", false
	}
	set, _, ok := strings.Cut(rest, ":")
	return set, ok
}

// checkOptionKey checks "option:<set>:<slug>@mesa": a known set and a slug as any key's.
func checkOptionKey(key string) error {
	set, ok := optionKeySet(key)
	if !ok {
		return ovErr(key, "the key must start with %q and name a set", OptionKeyPrefix)
	}
	if _, known := optionSetOf(set); !known {
		return ovErr(key, "the key names an unknown option set %q", set).at("key", ReasonKey)
	}
	return checkTableKey(OptionKeyPrefix+set+":", key)
}

// addOption registers a table option: a feature of the clone that the SRD features of its set
// offer, with its effects queued with the others.
func (b *overlayBuilder) addOption(o *TableOption, path string) error {
	n := b.n
	key := o.Key
	var c entryErrors
	def, known := optionSetOf(o.Set)
	switch keySet, _ := optionKeySet(key); {
	case !known:
		c.at(key, path, ".set", ReasonValue, "unknown option set %q", o.Set)
	case keySet != o.Set:
		c.at(key, path, ".set", ReasonValue, "the set is the one in the key (%q)", keySet)
	}
	checkTextAt(&c, key, path, ".desc_pt", o.DescPT)
	b.checkOptionPrerequisite(&c, o, path)
	for i := range o.Effects {
		if o.Effects[i].Type == "choice" {
			c.at(key, path, fmt.Sprintf(".effects[%d].type", i), ReasonEffect, "an option does not ask for another choice")
		}
	}
	if len(o.Effects) > MaxFeatureEffects {
		c.at(key, path, ".effects", ReasonLimit, "%d effects; the limit is %d per feature", len(o.Effects), MaxFeatureEffects)
	}
	if len(o.Effects) > 0 {
		b.pending = append(b.pending, pendingEffects{owner: key, entry: key, effects: o.Effects, path: path, strict: b.strict[key]})
		b.know(key, featureKind{owner: key})
		b.declareResources(featureKind{owner: key}, o.Effects)
		b.declareStates(key, key, o.Effects)
	}
	if err := c.err(); err != nil {
		return err
	}
	b.register(o.TableEntry)
	b.offerOption(o, def)
	n.optionSet[key] = o.Set
	if !o.Prerequisite.IsZero() {
		n.optionPrereq[key] = o.Prerequisite
	}
	return nil
}

// offerOption writes the option as a feature and adds it to the options of every feature
// that offers its set (the ones the content has). The offering features are copied before
// they change: the SRD's are shared.
func (b *overlayBuilder) offerOption(o *TableOption, def optionSetDef) {
	n := b.n
	var offerers []string
	for _, k := range def.Offerers {
		if n.features[k] != nil {
			offerers = append(offerers, k)
		}
	}
	if len(offerers) == 0 {
		return
	}
	first := n.features[offerers[0]]
	n.features[o.Key] = &srd51.Feature{Key: o.Key, Name: o.NamePT, Class: first.Class, Parent: offerers[0], Desc: slices.Clone(o.DescPT)}
	if b.optionOffers == nil {
		b.optionOffers = map[string]bool{}
	}
	for _, k := range offerers {
		if !b.optionOffers[k] {
			cp := *n.features[k]
			cp.Options = slices.Clone(cp.Options)
			n.features[k] = &cp
			b.optionOffers[k] = true
		}
		n.features[k].Options = append(n.features[k].Options, o.Key)
		n.offeredBy[o.Key] = append(n.offeredBy[o.Key], k)
	}
}

// checkOptionPrerequisite checks what the option asks for, each at its own field.
func (b *overlayBuilder) checkOptionPrerequisite(c *entryErrors, o *TableOption, path string) {
	p, key := o.Prerequisite, o.Key
	base := path + ".prerequisite"
	if (p.ClassKey == "") != (p.ClassLevel == 0) {
		c.at(key, path, ".prerequisite.class_level", ReasonValue, "a class and a level go together")
	}
	if p.ClassKey != "" && !b.isClass(p.ClassKey) {
		c.at(key, path, ".prerequisite.class_level.class_key", ReasonReference, "the class %q does not exist", p.ClassKey)
	}
	if p.ClassLevel < 0 || p.ClassLevel > MaxLevel {
		c.at(key, path, ".prerequisite.class_level.level", ReasonValue, "the level is 1 to %d", MaxLevel)
	}
	if len(p.RequiresOptions) > MaxOptionRequires {
		c.at(key, path, ".prerequisite.requires_option_keys", ReasonLimit, "%d options; the limit is %d", len(p.RequiresOptions), MaxOptionRequires)
	}
	for i, k := range p.RequiresOptions {
		switch {
		case k == key:
			c.at(key, path, fmt.Sprintf(".prerequisite.requires_option_keys[%d]", i), ReasonValue, "an option cannot require itself")
		case !b.isOption(k):
			c.at(key, path, fmt.Sprintf(".prerequisite.requires_option_keys[%d]", i), ReasonReference, "the option %q does not exist", k)
		}
	}
	if p.RequiresPact != "" && !b.isPact(p.RequiresPact) {
		c.at(key, path, ".prerequisite.requires_pact", ReasonReference, "%q is not a Pact Boon", p.RequiresPact)
	}
	if p.RequiresSpell != "" && !b.isSpell(p.RequiresSpell) {
		c.at(key, path, ".prerequisite.requires_spell_key", ReasonReference, "the spell %q does not exist", p.RequiresSpell)
	}
	_ = base
}

// isOption says the key is an option of an SRD option set or a table option of the overlay.
func (b *overlayBuilder) isOption(key string) bool {
	if strings.HasPrefix(key, OptionKeyPrefix) {
		return b.entries[key]
	}
	if f := b.base.features[key]; f != nil && f.Parent != "" {
		return slices.ContainsFunc(optionSetDefs, func(d optionSetDef) bool { return slices.Contains(d.Offerers, f.Parent) })
	}
	return false
}

// isPact says the key is a Pact Boon: the SRD's, or a table option of the set "pact_boon".
func (b *overlayBuilder) isPact(key string) bool {
	if set, ok := optionKeySet(key); ok {
		return set == "pact_boon" && b.entries[key]
	}
	f := b.base.features[key]
	return f != nil && f.Parent == pactBoonKey
}

// OptionEntry is a table option as the content lists it.
type OptionEntry struct {
	Key, NamePT, Set string
	Archived, Off    bool
}

// Options lists the table's options, sorted by set and Portuguese name.
func (c *Content) Options() []OptionEntry {
	x := c.c
	out := make([]OptionEntry, 0, len(x.optionSet))
	for _, k := range sortedKeys(x.optionSet) {
		out = append(out, OptionEntry{Key: k, NamePT: x.namePT(k), Set: x.optionSet[k], Archived: x.archived[k], Off: x.off[k]})
	}
	slices.SortStableFunc(out, func(a, b OptionEntry) int {
		if a.Set != b.Set {
			return strings.Compare(a.Set, b.Set)
		}
		return comparePT(a.NamePT, b.NamePT)
	})
	return out
}
