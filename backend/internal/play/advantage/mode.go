// Package advantage holds what the play module knows about advantage and disadvantage that needs no
// store, no transport and no Service: the modes as the database and the API spell them, the source
// kinds of the API, and the Portuguese sentence of each source (SRD 5.1, "Advantage and
// Disadvantage"). The rules are package combat's; play reads the combat for the facts and calls here.
package advantage

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The values of a roll mode as the database keeps them.
const (
	ModeNormalKey       = "normal"
	ModeAdvantageKey    = "advantage"
	ModeDisadvantageKey = "disadvantage"
)

// ModeKey is a mode as the database keeps it.
func ModeKey(m combat.RollMode) string {
	switch m {
	case combat.ModeAdvantage:
		return ModeAdvantageKey
	case combat.ModeDisadvantage:
		return ModeDisadvantageKey
	case combat.ModeNormal:
	}
	return ModeNormalKey
}

// ModeOfKey reads a stored mode; normal for anything else.
func ModeOfKey(k string) combat.RollMode {
	switch k {
	case ModeAdvantageKey:
		return combat.ModeAdvantage
	case ModeDisadvantageKey:
		return combat.ModeDisadvantage
	}
	return combat.ModeNormal
}

// ModeToProto is a roll mode as the API spells it.
var ModeToProto = map[combat.RollMode]playv1.RollMode{
	combat.ModeNormal:       playv1.RollMode_ROLL_MODE_NORMAL,
	combat.ModeAdvantage:    playv1.RollMode_ROLL_MODE_ADVANTAGE,
	combat.ModeDisadvantage: playv1.RollMode_ROLL_MODE_DISADVANTAGE,
}

// ModeFromProto reads a requested mode; ok is false for UNSPECIFIED.
func ModeFromProto(m playv1.RollMode) (combat.RollMode, bool) {
	switch m {
	case playv1.RollMode_ROLL_MODE_NORMAL:
		return combat.ModeNormal, true
	case playv1.RollMode_ROLL_MODE_ADVANTAGE:
		return combat.ModeAdvantage, true
	case playv1.RollMode_ROLL_MODE_DISADVANTAGE:
		return combat.ModeDisadvantage, true
	case playv1.RollMode_ROLL_MODE_UNSPECIFIED:
	}
	return combat.ModeNormal, false
}

var sourceKindToProto = map[string]playv1.AdvantageSourceKind{
	combat.SourceProneTarget:        playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PRONE_TARGET,
	combat.SourceProneAttacker:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PRONE_ATTACKER,
	combat.SourceRestrainedTarget:   playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RESTRAINED_TARGET,
	combat.SourceRestrainedAttacker: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RESTRAINED_ATTACKER,
	combat.SourceBlindedTarget:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_BLINDED_TARGET,
	combat.SourceBlindedAttacker:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_BLINDED_ATTACKER,
	combat.SourcePoisonedAttacker:   playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_POISONED_ATTACKER,
	combat.SourceFrightenedAttacker: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_FRIGHTENED_ATTACKER,
	combat.SourceInvisibleTarget:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_INVISIBLE_TARGET,
	combat.SourceInvisibleAttacker:  playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_INVISIBLE_ATTACKER,
	combat.SourceStunnedTarget:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_STUNNED_TARGET,
	combat.SourceParalyzedTarget:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PARALYZED_TARGET,
	combat.SourceUnconsciousTarget:  playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_UNCONSCIOUS_TARGET,
	combat.SourcePetrifiedTarget:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PETRIFIED_TARGET,
	combat.SourceRecklessAttacker:   playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RECKLESS_ATTACKER,
	combat.SourceRecklessTarget:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RECKLESS_TARGET,
	combat.SourceDodgingTarget:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_DODGING_TARGET,
	combat.SourcePackTactics:        playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PACK_TACTICS,
	combat.SourceUnseenAttacker:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_UNSEEN_ATTACKER,
	combat.SourceUnseenTarget:       playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_UNSEEN_TARGET,
	combat.SourceLongRange:          playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_LONG_RANGE,
	combat.SourceHostileNearby:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_HOSTILE_NEARBY,
	combat.SourceDodgingSave:        playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_DODGING_SAVE,
	combat.SourceDangerSense:        playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_DANGER_SENSE,
	combat.SourceRageStrength:       playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RAGE_STRENGTH,
	combat.SourceExhaustionAttack:   playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EXHAUSTION_ATTACK,
	combat.SourceExhaustionSave:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EXHAUSTION_SAVE,
	combat.SourceExhaustionCheck:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EXHAUSTION_CHECK,
	combat.SourceOutlinedTarget:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OUTLINED_TARGET,
	combat.SourceEffectSave:         playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_SAVE,
	combat.SourceTaggedSave:         playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_TAGGED_SAVE,
	combat.SourceEffectCheck:        playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_CHECK,
	combat.SourceEffectAttack:       playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_ATTACK,
	combat.SourceRestrainedSave:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_RESTRAINED_SAVE,
	combat.SourcePoisonedCheck:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_POISONED_CHECK,
	combat.SourceFrightenedCheck:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_FRIGHTENED_CHECK,
	combat.SourcePerceptionDim:      playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PERCEPTION_DIM,
	combat.SourceHelp:               playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_HELP,
	combat.SourceArmorUntrained:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_ARMOR_NOT_PROFICIENT,
	combat.SourceProtection:         playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_PROTECTION,
	combat.SourceMagicResistance:    playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_MAGIC_RESISTANCE,
	combat.SourceFavor:              playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_FAVOR,
	combat.SourceSunlightAttack:     playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_SUNLIGHT_ATTACK,
	combat.SourceSunlightPerception: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_SUNLIGHT_PERCEPTION,
	combat.SourceSunlightSave:       playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_SUNLIGHT_SAVE,
}
