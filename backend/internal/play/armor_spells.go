package play

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Armor or a shield without proficiency stops the casting of every spell, in a combat and out
// of one (SRD 5.1, "Armor"): the turn options disable the spells with the reason, and these
// are the refusals of the calls that still arrive.

// errArmorNotProficient is the refusal of a cast in a combat.
func errArmorNotProficient() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_NOT_PROFICIENT, "a character in armor or with a shield it is not proficient with cannot cast spells")
}

// errCastingArmorNotProficient is the refusal of a cast out of a combat.
func errCastingArmorNotProficient() error {
	return errCasting(playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_ARMOR_NOT_PROFICIENT, "a character in armor or with a shield it is not proficient with cannot cast spells")
}
