package rules

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// The summon of a table feature. A feature with a Summon brings the table's creatures to its holder through the
// paths of a summoning spell: the content keeps a spell of its own for it (a hidden, leveled, self-targeted spell
// that no list, search or entry shows) and a granted_spell effect that gives it to the holder, so the options, the
// checks, the concentration, the replacement, the dismissal and the undo are the spell's. What the casting spends
// is a use of a resource of the feature's entry, where a spell spends a slot.

// featureSummonSpellPrefix starts the key of the hidden spell of a feature's summon.
const featureSummonSpellPrefix = "spell:"

// featureSummonLevel is the circle of the hidden spell: a feature's summon has no circle of its own (slot() is 0 in
// its count), the first one keeps the spell leveled, as a summoning spell is.
const featureSummonLevel = 1

// featureSummonAbility is the casting ability of the granted spell. A summon rolls no attack and asks for no saving
// throw, so the choice shows nowhere.
const featureSummonAbility = "int"

// FeatureSummonSpellKey is the key of the hidden spell of a feature's summon: "spell:" and the feature's key
// without its colon ("spell:feature-class-guardiao-3f9a1c--vigor@mesa"). A feature key whose spell key would pass the
// longest slug is cut, with a digest of the whole key in its end, so two long keys never meet.
func FeatureSummonSpellKey(featureKey string) string {
	key := featureSummonSpellPrefix + strings.Replace(featureKey, ":", "-", 1)
	slug, ok := strings.CutSuffix(strings.TrimPrefix(key, featureSummonSpellPrefix), tableSuffix)
	if !ok || len(slug) <= MaxSlugLength {
		return key
	}
	sum := sha256.Sum256([]byte(featureKey))
	const digest = 8
	return featureSummonSpellPrefix + slug[:MaxSlugLength-digest-1] + "-" + hex.EncodeToString(sum[:])[:digest] + tableSuffix
}

// pendingSummon is the resource a lowered summon spends and where its granted_spell effect is among the pending ones.
type pendingSummon struct {
	resource string
	at       int
}

// lowerFeatureSummon checks a feature's summon, registers its hidden spell and returns the effects of the feature
// with the granted_spell effect that gives the spell, and what to check once every entry exists. A feature with no
// summon goes through as it is.
func (b *overlayBuilder) lowerFeatureSummon(f *TableFeature, owner, path string, effects []Effect) ([]Effect, *pendingSummon, error) {
	s := f.Summon
	if s == nil {
		return effects, nil, nil
	}
	var c entryErrors
	spellKey := FeatureSummonSpellKey(f.Key)
	if err := checkTableKey(featureSummonSpellPrefix, spellKey); err != nil {
		c.at(f.Key, path, ".key", ReasonValue, "the feature's summon needs a spell key of its own made from the feature's key, and it is not a valid key")
	} else if err := b.claim(spellKey); err != nil {
		c.at(f.Key, path, ".key", ReasonValue, "the key of the feature's summon (%s) is taken by another entry: rename the feature or that entry", spellKey)
	}
	ts := &TableSpell{
		TableEntry: TableEntry{Key: spellKey, NamePT: f.NamePT},
		Level:      featureSummonLevel, School: "school:conjuration",
		CastingTime: TableCastingTime{Unit: cmp.Or(s.CastingTime.Unit, CastAction), Amount: s.CastingTime.Amount, TriggerPT: s.CastingTime.TriggerPT},
		Range:       TableRange{Kind: RangeSelf}, Duration: TableDuration{Kind: cmp.Or(s.Duration.Kind, DurationInstantaneous), Amount: s.Duration.Amount, Unit: s.Duration.Unit, UpTo: s.Duration.UpTo},
		Concentration: s.Concentration, DescPT: f.DescPT, Target: SpellTarget{Kind: TargetSelf}, Summon: s,
	}
	sp := b.spellHead(&c, ts, path+".summon")
	b.addSummon(&c, spellKey, path, ts, f.Key)
	if err := c.err(); err != nil {
		return nil, nil, err
	}
	b.register(ts.TableEntry)
	b.n.spells[spellKey] = sp
	b.n.spellTargets[spellKey] = SpellTarget{Kind: TargetSelf}
	b.n.featureSummons[spellKey] = owner
	grant := Effect{Type: "granted_spell", Spell: spellKey, Uses: s.ResourceKey, Ability: featureSummonAbility}
	return append(effects[:len(effects):len(effects)], grant), &pendingSummon{resource: s.ResourceKey, at: len(effects)}, nil
}
