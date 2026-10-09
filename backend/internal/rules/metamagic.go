package rules

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Metamagic (SRD 5.1, Sorcerer, Metamagic). A sorcerer knows two options from
// level 3 and one more at levels 10 and 17; each costs sorcery points and shapes a
// casting. "You can use only one Metamagic option on a spell when you cast it,
// unless otherwise noted": Empowered Spell is the one that may join another.

// The Metamagic options, as the features that give them are keyed.
const (
	MetamagicCareful    = "feature:metamagic-careful-spell"
	MetamagicDistant    = "feature:metamagic-distant-spell"
	MetamagicEmpowered  = "feature:metamagic-empowered-spell"
	MetamagicExtended   = "feature:metamagic-extended-spell"
	MetamagicHeightened = "feature:metamagic-heightened-spell"
	MetamagicQuickened  = "feature:metamagic-quickened-spell"
	MetamagicSubtle     = "feature:metamagic-subtle-spell"
	MetamagicTwinned    = "feature:metamagic-twinned-spell"
)

// metamagicOrder is the order the options are listed in: the SRD's, which is
// alphabetical in English.
var metamagicOrder = []string{
	MetamagicCareful, MetamagicDistant, MetamagicEmpowered, MetamagicExtended,
	MetamagicHeightened, MetamagicQuickened, MetamagicSubtle, MetamagicTwinned,
}

// metamagicFlatCost is the sorcery points of an option that costs the same for
// every spell (the SRD's text of each option); Twinned Spell is not here: it costs
// the spell's level.
// The costs the SRD names: Heightened Spell 3 points, Quickened Spell 2, the rest 1.
const (
	heightenedCost = 3
	quickenedCost  = 2
)

var metamagicFlatCost = map[string]int{
	MetamagicCareful:    1,
	MetamagicDistant:    1,
	MetamagicEmpowered:  1,
	MetamagicExtended:   1,
	MetamagicHeightened: heightenedCost,
	MetamagicQuickened:  quickenedCost,
	MetamagicSubtle:     1,
}

// metamagicSummaryPT says in one phrase what each option does, in our words.
var metamagicSummaryPT = map[string]string{
	MetamagicCareful:    "Criaturas à sua escolha passam sozinhas no teste de resistência.",
	MetamagicDistant:    "Dobra o alcance; uma magia de toque passa a alcançar 9 m.",
	MetamagicEmpowered:  "Rerrola alguns dados de dano; pode se somar a outra opção.",
	MetamagicExtended:   "Dobra a duração, até 24 horas.",
	MetamagicHeightened: "Um alvo tem desvantagem no primeiro teste de resistência contra a magia.",
	MetamagicQuickened:  "Uma magia de 1 ação passa a ser conjurada com uma ação bônus.",
	MetamagicSubtle:     "Conjura sem palavras nem gestos.",
	MetamagicTwinned:    "Um segundo alvo no alcance. Custa o nível da magia: 1 para um truque.",
}

// ErrMetamagic is the base of the Metamagic refusals.
var ErrMetamagic = errors.New("metamagic")

// MetamagicOption is one option a sorcerer knows, for a spell cast at a slot level.
type MetamagicOption struct {
	Key string
	// NamePT is the option's name without the "Metamágica:" the feature's name
	// carries ("Magia Cuidadosa").
	NamePT string
	// Cost is the sorcery points it takes for this spell.
	Cost int
	// SummaryPT is what it does.
	SummaryPT string
	// Allowed says the spell takes the option; otherwise ReasonPT says why not.
	Allowed  bool
	ReasonPT string
}

// MetamagicKnown lists the Metamagic options the character has, in the order the
// options are listed in.
func MetamagicKnown(features []Feature) []string {
	var out []string
	for _, key := range metamagicOrder {
		if slices.ContainsFunc(features, func(f Feature) bool { return f.Key == key }) {
			out = append(out, key)
		}
	}
	return out
}

// MetamagicCost is the sorcery points the option takes on a spell of the level:
// Twinned Spell costs "a number of sorcery points equal to the spell's level (1
// sorcery point if the spell is a cantrip)". An unknown option costs 0.
func MetamagicCost(key string, spellLevel int) int {
	if key == MetamagicTwinned {
		return max(spellLevel, 1)
	}
	return metamagicFlatCost[key]
}

// MetamagicOptions says, for each option known, whether the spell takes it when
// cast with the slot level (the spell's own for a cantrip), and why not.
func (c *Content) MetamagicOptions(known []string, spellKey string, slotLevel int) []MetamagicOption {
	d, ok := c.SpellDetails(spellKey)
	if !ok {
		return nil
	}
	var out []MetamagicOption
	for _, key := range metamagicOrder {
		if !slices.Contains(known, key) {
			continue
		}
		o := MetamagicOption{
			Key: key, NamePT: MetamagicNamePT(c.NamePT(key)), Cost: MetamagicCost(key, d.Spell.Level), SummaryPT: metamagicSummaryPT[key],
		}
		o.ReasonPT = metamagicReason(key, d, slotLevel)
		o.Allowed = o.ReasonPT == ""
		out = append(out, o)
	}
	return out
}

// MetamagicNamePT is the option's name as the cast sheet writes it: the feature's
// name without the "Metamágica: " it starts with.
func MetamagicNamePT(featureName string) string {
	return strings.TrimPrefix(featureName, "Metamágica: ")
}

// minuteUnits are the duration units that last a minute or more.
var minuteUnits = []string{DurationMinute, DurationHour, DurationDay}

// metamagicReason is why the spell does not take the option, in Portuguese, or ""
// when it does. It reads the spell as the SRD gives it.
func metamagicReason(key string, d *SpellDetails, slotLevel int) string {
	name := d.Spell.NamePT
	switch key {
	case MetamagicCareful, MetamagicHeightened:
		if d.Save == nil {
			return fmt.Sprintf("%s não pede teste de resistência.", name)
		}
	case MetamagicDistant:
		if d.Range.Kind != RangeTouch && (d.Range.Kind != RangeRanged || d.Range.DistanceFt < distantMinRangeFt) {
			return fmt.Sprintf("O alcance de %s não pode ser dobrado.", name)
		}
	case MetamagicEmpowered:
		if len(d.Damage) == 0 {
			return fmt.Sprintf("%s não rola dados de dano.", name)
		}
	case MetamagicExtended:
		if d.Duration.Kind != DurationTimed || !slices.Contains(minuteUnits, d.Duration.Unit) {
			return "Só vale para magias de 1 minuto ou mais."
		}
	case MetamagicQuickened:
		if d.CastingTime.Unit != CastAction || d.CastingTime.Amount != 1 {
			return fmt.Sprintf("%s não é conjurada com 1 ação.", name)
		}
	case MetamagicTwinned:
		switch {
		case d.Range.Kind == RangeSelf:
			return fmt.Sprintf("%s tem alcance Pessoal.", name)
		case d.Target.IsArea():
			return fmt.Sprintf("%s atinge uma área: não tem “um só alvo”.", name)
		case d.Target.Kind != TargetCreature || d.Target.MaxTargets(max(slotLevel, d.Spell.Level), d.Spell.Level) != 1:
			return fmt.Sprintf("%s pode atingir mais de uma criatura.", name)
		}
	}
	return ""
}

// distantMinRangeFt is the shortest range Distant Spell doubles: "a range of 5 feet
// or greater".
const distantMinRangeFt = 5

// CheckMetamagicChoice checks the options a casting uses: all known, none repeated,
// one only, or Empowered Spell with one other.
func CheckMetamagicChoice(chosen, known []string) error {
	seen := map[string]bool{}
	for _, key := range chosen {
		switch {
		case !slices.Contains(known, key):
			return fmt.Errorf("%w: %s is not an option the character knows", ErrMetamagic, key)
		case seen[key]:
			return fmt.Errorf("%w: %s is chosen twice", ErrMetamagic, key)
		}
		seen[key] = true
	}
	switch {
	case len(chosen) <= 1:
		return nil
	case len(chosen) == maxMetamagicOptions && seen[MetamagicEmpowered]:
		return nil
	}
	return fmt.Errorf("%w: only one option on a spell, unless Empowered Spell joins another", ErrMetamagic)
}

// maxMetamagicOptions is the most options on one casting: Empowered Spell and one other.
const maxMetamagicOptions = 2

// CarefulCreatures is how many creatures Careful Spell protects: up to the
// Charisma modifier, and at least one.
func CarefulCreatures(charismaModifier int) int { return max(charismaModifier, 1) }

// EmpoweredDice is how many damage dice Empowered Spell rerolls: up to the
// Charisma modifier, and at least one.
func EmpoweredDice(charismaModifier int) int { return max(charismaModifier, 1) }

// DistantRangeFt is the range of a spell made Distant: double, or 30 feet for a
// spell of touch (SRD 5.1, Distant Spell).
func DistantRangeFt(kind string, distanceFt int) int {
	if kind == RangeTouch {
		return distantTouchFt
	}
	return distanceFt * 2 //nolint:mnd // doubling the range
}

// distantTouchFt is what Distant Spell makes of a range of touch.
const distantTouchFt = 30
