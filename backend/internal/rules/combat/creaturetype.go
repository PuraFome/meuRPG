package combat

import "github.com/PuraFome/meuRPG/backend/internal/rules"

// What the creature type of a target changes in a spell (SRD 5.1). The type is the stat block's ("undead", "plant"); a
// creature with no type entry (a character, a creature the master wrote by hand) has none of these rules.

// SpellBlight is Blight, the spell whose save and damage read the target's type.
const SpellBlight = "spell:blight"

// LifeMagicNoEffect says a spell that heals (Cure Wounds, Healing Word, Heal, Mass Cure Wounds), stabilizes (Spare the Dying)
// or withers (Blight) does nothing to a creature of the type: "This spell has no effect on undead or constructs".
func LifeMagicNoEffect(creatureType string) bool {
	return rules.LayOnHandsNoEffect(creatureType)
}

// SpellReadsType says the spell has a rule for the target's creature type (SpellTypeRule).
func SpellReadsType(spellKey string) bool { return spellKey == SpellBlight }

// IsPlant says the creature type is plant.
func IsPlant(creatureType string) bool { return creatureType == "plant" }

// BlightSaveMode is the mode of the saving throw against Blight: a plant makes it with disadvantage, any other creature as it
// would.
func BlightSaveMode(creatureType string) RollMode {
	if IsPlant(creatureType) {
		return ModeDisadvantage
	}
	return ModeNormal
}

// MaxRoll is the most dice of the sides given can give, with the bonus: what a damage that "deals maximum damage" is.
func MaxRoll(count, sides, bonus int) int {
	return count*sides + bonus
}

// TypeRule is what the target's creature type changes in a spell that saves.
type TypeRule struct {
	// NoEffect says the spell does nothing: no save, no damage.
	NoEffect bool
	// Mode is the mode of the save for the type (normal when none) and CreatureType the type it rests on.
	Mode         RollMode
	CreatureType string
	// Plant says the damage is the most its dice can give (MaxRoll).
	Plant bool
}

// SpellTypeRule is the rule of the spell for a target of the type; a spell that does not read the type has none.
func SpellTypeRule(spellKey, creatureType string) TypeRule {
	if !SpellReadsType(spellKey) {
		return TypeRule{}
	}
	return TypeRule{NoEffect: LifeMagicNoEffect(creatureType), Mode: BlightSaveMode(creatureType), CreatureType: creatureType, Plant: IsPlant(creatureType)}
}
