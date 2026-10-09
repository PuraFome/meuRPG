package combat

import "github.com/PuraFome/meuRPG/backend/internal/rules"

// What the Life Domain adds to a healing spell (SRD 5.1, Cleric: Life Domain).
// The keys are the features' own.
const (
	// DiscipleOfLifeKey: when a spell of 1st level or higher restores hit points
	// to a creature, the creature regains 2 + the spell's level more.
	DiscipleOfLifeKey = "feature:disciple-of-life"
	// BlessedHealerKey: when the caster casts a spell of 1st level or higher that
	// restores hit points to a creature other than the caster, the caster regains
	// 2 + the spell's level.
	BlessedHealerKey = "feature:blessed-healer"
	// SupremeHealingKey: where the spell would roll dice to restore hit points,
	// the highest number of each die is used instead.
	SupremeHealingKey = "feature:supreme-healing"
)

// HealFeatures says which of the healer's features change a healing spell.
type HealFeatures struct {
	Disciple, Blessed, Supreme bool
}

// HealFeaturesOf reads the features a sheet has.
func HealFeaturesOf(features []rules.Feature) HealFeatures {
	var h HealFeatures
	for _, f := range features {
		switch f.Key {
		case DiscipleOfLifeKey:
			h.Disciple = true
		case BlessedHealerKey:
			h.Blessed = true
		case SupremeHealingKey:
			h.Supreme = true
		}
	}
	return h
}

// levelBonus is Disciple of Life's and Blessed Healer's 2 + the spell's level,
// for a spell of 1st level or higher cast at slotLevel (the spell assumes the
// slot's level for that casting, SRD 5.1, "Casting a Spell at a Higher Level").
// lifeDomainBonus is the 2 of Disciple of Life's and Blessed Healer's "2 + the spell's level".
const lifeDomainBonus = 2

func levelBonus(slotLevel int) int {
	if slotLevel < 1 {
		return 0
	}
	return lifeDomainBonus + slotLevel
}

// TargetExtra is what each creature the spell heals regains besides the spell's
// own healing: Disciple of Life.
func (h HealFeatures) TargetExtra(slotLevel int) int {
	if !h.Disciple {
		return 0
	}
	return levelBonus(slotLevel)
}

// SelfExtra is what the caster regains when the spell healed someone else:
// Blessed Healer, once for the casting, however many creatures it healed.
func (h HealFeatures) SelfExtra(slotLevel int, healedOther bool) int {
	if !h.Blessed || !healedOther {
		return 0
	}
	return levelBonus(slotLevel)
}

// HealTotal is the healing a creature receives from a healing spell's dice
// (count d sides + bonus) whose roll came out as rolled: the roll itself, or the
// highest roll possible with Supreme Healing. The Disciple of Life amount is not
// in it (TargetExtra).
func (h HealFeatures) HealTotal(count, sides, bonus, rolled int) int {
	if h.Supreme {
		return count*sides + bonus
	}
	return rolled + bonus
}
