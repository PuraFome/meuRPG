package play

import (
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Brutal Critical (SRD 5.1, Barbarian, levels 9, 13 and 17): on a critical hit with
// a melee attack the barbarian rolls one, two or three more weapon damage dice
// when working out the extra damage. They are the weapon's die, always rolled
// (whatever the table's rule for the critical's own dice is), and come after the
// critical's own dice in the roll.

// Savage Attacks (SRD 5.1, Half-Orc): when the half-orc scores a critical hit with
// a melee weapon attack, it rolls one of the weapon's damage dice one additional
// time and adds it to the extra damage of the critical hit. It is one more weapon
// die, rolled like Brutal Critical's, and the two stack. An unarmed strike is a
// melee attack but has no weapon die, so it gets none.

// maxExtraDice is the most extra dice the features add to a critical hit (Brutal
// Critical at level 17 and Savage Attacks).
const maxExtraDice = 4

// savageAttacksNamePT is the trait's name as the damage line writes it.
const savageAttacksNamePT = "Ataques Selvagens"

// brutalCriticalNamePT is the feature's name as the damage line writes it.
const brutalCriticalNamePT = "Crítico Brutal"

// brutalCriticalDice says how many extra weapon dice the hit rolls: the attacker
// must be a player's character with the feature, the hit a critical one and the attack
// a melee weapon attack with dice of its own. A ranged attack, a spell, a creature
// and an NPC never have them.
func brutalCriticalDice(attacker playdb.Combatant, attack link.Attack, sheet link.Sheet, critical bool) int {
	if !critical || attacker.Kind != kindPlayer || !attack.Melee || attack.Spell || attack.DiceCount < 1 {
		return 0
	}
	return min(max(sheet.BrutalCriticalDice, 0), maxExtraDice)
}

// savageAttacksDice says whether the hit rolls the half-orc's extra die (1) or not (0):
// the attacker must be a player's character with Savage Attacks, the hit a critical
// one and the attack a melee weapon attack with a die of its own (not a spell, not
// an unarmed strike, not a ranged attack).
func savageAttacksDice(attacker playdb.Combatant, attack link.Attack, sheet link.Sheet, critical bool) int {
	if !critical || attacker.Kind != kindPlayer || !sheet.SavageAttacks || !attack.Melee || attack.Spell || !attack.Weapon || attack.Unarmed || attack.DiceCount < 1 {
		return 0
	}
	return 1
}

// extraDiceName is the Portuguese name of the feature that adds the extra dice,
// empty when there are none. savage is how many of the extra dice are the
// Savage Attacks'; the rest are Brutal Critical's.
func extraDiceName(extra, savage int32) string {
	switch {
	case extra < 1:
		return ""
	case savage < 1:
		return brutalCriticalNamePT
	case savage >= extra:
		return savageAttacksNamePT
	}
	return brutalCriticalNamePT + " e " + savageAttacksNamePT
}
