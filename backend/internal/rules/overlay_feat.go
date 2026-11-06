package rules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// TableFeat is a feat of the table (MR-025): a name, a text, what it asks of the
// character and its effects, from the same closed menu as a feature's, plus the
// ability_increase only a feat has. Its Key is "feat:<slug>@mesa". The effects
// belong to the feat itself (no feature of their own): a character that took the
// feat has them.
type TableFeat struct {
	TableEntry
	DescPT       []string
	Prerequisite FeatPrerequisite
	Effects      []Effect
	// Options is the feat's option list, as a class feature's (TableFeature.Options): up
	// to MaxFeatOptions options, each a feature "feature:...@mesa" whose effects apply
	// only while it is picked. They do not nest. The feat's choice of features lists
	// them in `from` (or, with none, offers all of them), or offers an option set (Effect.Set).
	Options []TableFeature
}

// addFeat registers a feat and its effects, which are compiled later with the
// others. Every field that is wrong is reported at its own path.
func (b *overlayBuilder) addFeat(tf *TableFeat, path string) error {
	n := b.n
	key := tf.Key
	var c entryErrors
	checkTextAt(&c, key, path, ".desc_pt", tf.DescPT)
	feat := &srd51.Feat{
		Key: key, Name: tf.NamePT, Desc: slices.Clone(tf.DescPT),
		Minimums: abilityStrings(tf.Prerequisite.Minimums), AnyOf: abilityStrings(tf.Prerequisite.AnyOf),
		Proficiency: tf.Prerequisite.Proficiency, Spellcasting: tf.Prerequisite.Spellcasting, Race: tf.Prerequisite.Race, Level: tf.Prerequisite.Level,
		Races: slices.Clone(tf.Prerequisite.Races), Size: tf.Prerequisite.Size, LacksFeature: tf.Prerequisite.LacksFeature,
	}
	b.checkFeatPrerequisite(&c, feat, key, path)
	increases := 0
	for i := range tf.Effects {
		if tf.Effects[i].Type == "ability_increase" {
			increases++
			if increases > 1 {
				c.at(key, path, fmt.Sprintf(".effects[%d]", i), ReasonLimit, "a feat has one ability increase")
			}
		}
	}
	// A feat's own resources and states are the ones its effects (a granted spell, an action, a hook)
	// may spend and read.
	k := featureKind{owner: key, prefix: featureKeyPrefix}
	b.know(key, k)
	effects := b.featOptions(&c, tf, k, path)
	b.declareResources(k, effects)
	b.declareStates(key, key, effects)
	b.writeFeatOfferer(&c, feat, effects, path)
	// The effects are queued before the entry is judged: if it is refused, its effects'
	// problems come with the rest (compileEffectsOf).
	if len(effects) > 0 {
		b.pending = append(b.pending, pendingEffects{owner: key, entry: key, effects: effects, path: path, strict: b.strict[key]})
	}
	if err := c.err(); err != nil {
		return err
	}
	b.register(tf.TableEntry)
	n.feats[key] = feat
	return nil
}

// featOptions registers the feat's option list, when it has one, as a class feature's is
// (featureOptions), and returns the feat's effects, with the choice of features that has no
// `from` offering the options. A problem is reported at the option's own path.
func (b *overlayBuilder) featOptions(c *entryErrors, tf *TableFeat, k featureKind, path string) []Effect {
	if len(tf.Options) == 0 {
		return tf.Effects
	}
	shim := &TableFeature{Key: tf.Key, NamePT: tf.NamePT, Effects: tf.Effects, Options: tf.Options}
	effects, err := b.featureOptions(shim, k, path)
	if err != nil {
		c.add(err)
		return tf.Effects
	}
	return effects
}

// writeFeatOfferer makes the feat the offerer of the options its choice of features
// lists, as writeFeature does for a class feature: the feat's Options, OptionsChoose (or
// OptionSet, for a choice that offers a whole set) say what it offers, and offeredBy
// says, for each option, that the feat is who offers it. A feat offers one choice of options.
func (b *overlayBuilder) writeFeatOfferer(c *entryErrors, feat *srd51.Feat, effects []Effect, path string) {
	offers := false
	for i := range effects {
		e := &effects[i]
		if !isFeatureChoice(e) {
			continue
		}
		if offers {
			c.at(feat.Key, path, ".effects", ReasonEffect, "a feat offers one choice of options")
			return
		}
		offers = true
		feat.Options, feat.OptionsChoose, feat.OptionSet = slices.Clone(e.From), e.Count, e.Set
		for _, o := range e.From {
			b.n.offeredBy[o] = append(b.n.offeredBy[o], feat.Key)
		}
	}
}

// checkFeatPrerequisite reports what is wrong with a feat's prerequisite, each
// problem at its own field. The proficiency, the races and the lacked feature it names
// may be the SRD's or the table's.
func (b *overlayBuilder) checkFeatPrerequisite(c *entryErrors, f *srd51.Feat, key, path string) {
	checkFeatScores(c, key, path, ".prerequisite.minimums", f.Minimums)
	checkFeatScores(c, key, path, ".prerequisite.any_of", f.AnyOf)
	if p := f.Proficiency; p != "" {
		if _, ok := b.base.proficiencies[p]; !ok {
			c.at(key, path, ".prerequisite.proficiency_key", ReasonReference, "%q is not a proficiency of the SRD", p)
		}
	}
	if r := f.Race; r != "" && !b.isRace(r) && !b.isSubrace(r) {
		c.at(key, path, ".prerequisite.race_key", ReasonReference, "the race or subrace %q does not exist", r)
	}
	b.checkFeatRaces(c, f, key, path)
	if f.Size != "" && !slices.Contains(raceSizes, f.Size) {
		c.at(key, path, ".prerequisite.size", ReasonValue, "the size is one of %s", strings.Join(raceSizes, ", "))
	}
	if k := f.LacksFeature; k == key {
		c.at(key, path, ".prerequisite.lacks_feature_key", ReasonValue, "a feat cannot ask not to have itself")
	} else if k != "" {
		b.checkLackedFeature(c, key, path, k)
	}
	if f.Level < 0 || f.Level > MaxLevel {
		c.at(key, path, ".prerequisite.level", ReasonValue, "the level is 0 to %d", MaxLevel)
	}
}

// checkFeatRaces checks the list of races a prerequisite asks for: it is not set with the
// single race_key, it is short, each race or subrace exists and none repeats.
func (b *overlayBuilder) checkFeatRaces(c *entryErrors, f *srd51.Feat, key, path string) {
	if len(f.Races) == 0 {
		return
	}
	if f.Race != "" {
		c.at(key, path, ".prerequisite.races", ReasonValue, "set races or race_key, not both")
	}
	if len(f.Races) > maxEffectList {
		c.at(key, path, ".prerequisite.races", ReasonLimit, "%d races; the limit is %d", len(f.Races), maxEffectList)
		return
	}
	for i, r := range f.Races {
		at := fmt.Sprintf(".prerequisite.races[%d]", i)
		switch {
		case !b.isRace(r) && !b.isSubrace(r):
			c.at(key, path, at, ReasonReference, "the race or subrace %q does not exist", r)
		case slices.Contains(f.Races[:i], r):
			c.at(key, path, at, ReasonValue, "the race or subrace %q is listed twice", r)
		}
	}
}

// featLack is the feature a feat asks the character not to have, checked once every entry
// of the overlay exists.
type featLack struct{ owner, path, feature string }

// checkLackedFeature checks the feature a feat asks the character not to have: an SRD key is
// judged now, a table key (its entry may come later in the overlay) once all the entries exist.
func (b *overlayBuilder) checkLackedFeature(c *entryErrors, key, path, feature string) {
	switch {
	case isTableKey(feature):
		b.featLacks = append(b.featLacks, featLack{owner: key, path: path, feature: feature})
	case !b.base.isFeatureKey(feature):
		c.at(key, path, ".prerequisite.lacks_feature_key", ReasonReference, "the feature %q does not exist", feature)
	}
}

// checkFeatLacks reports each lacks_feature_key that names no feature, trait or feat of the
// SRD or of the table.
func (b *overlayBuilder) checkFeatLacks() error {
	var c entryErrors
	for _, l := range b.featLacks {
		if !b.n.isFeatureKey(l.feature) {
			c.at(l.owner, l.path, ".prerequisite.lacks_feature_key", ReasonReference, "the feature %q does not exist", l.feature)
		}
	}
	return c.err()
}

// isFeatureKey says the key is a class or subclass feature, a trait, a background feature, a
// feat or an option the content has: what a sheet can list among its features.
func (c *content) isFeatureKey(key string) bool {
	for _, prefix := range []string{featureKeyPrefix, "trait:", "background-feature:", "feat:", OptionKeyPrefix, ClassFeatureKeyPrefix} {
		if strings.HasPrefix(key, prefix) {
			return c.effectOwnerExists(key)
		}
	}
	return false
}

// checkFeatScores checks the ability scores a feat asks for, in the order of
// the abilities' names; attr is the set's field.
func checkFeatScores(c *entryErrors, key, path, attr string, set map[string]int) {
	for _, a := range sortedKeys(set) {
		v := set[a]
		if _, ok := abilityIndex[Ability(a)]; !ok {
			c.at(key, path, attr, ReasonValue, "unknown ability %q", a)
		} else if v < 1 || v > MaxScore {
			c.at(key, path, attr+"."+abilityField[Ability(a)], ReasonValue, "the minimum score is 1 to %d", MaxScore)
		}
	}
}

func (b *overlayBuilder) isSubrace(key string) bool {
	_, ok := b.base.subraces[key]
	return ok || (b.entries[key] && strings.HasPrefix(key, "subrace:"))
}

// abilityStrings turns a map of abilities into the SRD data's keys, dropping the
// zeros ("not asked").
func abilityStrings(m map[Ability]int) map[string]int {
	var out map[string]int
	for a, v := range m {
		if v == 0 {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[string(a)] = v
	}
	return out
}

// checkAbilityIncrease is the closed shape of an ability_increase: a count of
// different abilities the player picks from a list, and the same amount (1 or 2,
// as an Ability Score Improvement gives) added to each. It returns the field the
// first problem is at, or "".
func checkAbilityIncrease(e *Effect) (attr, msg string) {
	switch {
	case len(e.From) == 0:
		return ".from", "an ability_increase lists the abilities the player picks from"
	case e.Count < 1 || e.Count > len(e.From):
		return ".count", fmt.Sprintf("an ability_increase picks 1 to the %d abilities it lists", len(e.From))
	case e.Value != "1" && e.Value != strconv.Itoa(abilityScoreImprovementPoints):
		return ".value", "an ability_increase adds 1 or 2 to each ability"
	}
	for i, a := range e.From {
		if _, ok := abilityIndex[Ability(a)]; !ok {
			return ".from", fmt.Sprintf("unknown ability %q in an ability_increase", a)
		}
		if slices.Contains(e.From[:i], a) {
			return ".from", fmt.Sprintf("the ability %q is listed twice in an ability_increase", a)
		}
	}
	return "", ""
}
