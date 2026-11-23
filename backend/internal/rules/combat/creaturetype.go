package combat

import "github.com/PuraFome/meuRPG/backend/internal/rules"

// What the creature type of a target changes in a spell (SRD 5.1). The type is the stat block's ("undead", "plant"); a
// creature with no type entry (a character, a creature the master wrote by hand) has none of these rules.

// SpellBlight is Blight, the spell whose save and damage read the target's type.
const SpellBlight = "spell:blight"

// spellTypeEntry is what the creature type of the target changes in one spell.
type spellTypeEntry struct {
	// noUndeadConstruct says the spell has no effect on an undead or a construct (SRD 5.1: "This spell has no effect on
	// undead or constructs").
	noUndeadConstruct bool
	// plant says a plant saves with disadvantage and takes the most the damage dice can give (Blight).
	plant bool
}

// spellTypeRules is the one table of the spells that read the creature type of the target, by spell key.
var spellTypeRules = map[string]spellTypeEntry{
	"spell:cure-wounds":       {noUndeadConstruct: true},
	"spell:healing-word":      {noUndeadConstruct: true},
	"spell:mass-cure-wounds":  {noUndeadConstruct: true},
	"spell:mass-healing-word": {noUndeadConstruct: true},
	"spell:prayer-of-healing": {noUndeadConstruct: true},
	"spell:heal":              {noUndeadConstruct: true},
	"spell:mass-heal":         {noUndeadConstruct: true},
	"spell:spare-the-dying":   {noUndeadConstruct: true},
	SpellBlight:               {noUndeadConstruct: true, plant: true},
}

// LifeMagicNoEffect says the spell does nothing to a creature of the type: it is a spell of the table that has no effect on
// undead or constructs, and the type is one of them.
func LifeMagicNoEffect(spellKey, creatureType string) bool {
	return spellTypeRules[spellKey].noUndeadConstruct && rules.LayOnHandsNoEffect(creatureType)
}

// SpellReadsType says the spell has a plant rule for the target's creature type (SpellTypeRule): Blight.
func SpellReadsType(spellKey string) bool { return spellTypeRules[spellKey].plant }

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

// SpellTypeRule is the rule of the spell for a target of the type; a spell outside the table has none.
func SpellTypeRule(spellKey, creatureType string) TypeRule {
	entry, ok := spellTypeRules[spellKey]
	if !ok {
		return TypeRule{}
	}
	rule := TypeRule{NoEffect: LifeMagicNoEffect(spellKey, creatureType), CreatureType: creatureType}
	if entry.plant {
		rule.Mode, rule.Plant = BlightSaveMode(creatureType), IsPlant(creatureType)
	}
	return rule
}
