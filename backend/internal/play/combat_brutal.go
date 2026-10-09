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

// maxExtraDice is the most extra dice a feature adds to a critical hit (Brutal
// Critical at level 17).
const maxExtraDice = 3

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

// extraDiceName is the Portuguese name of the feature that adds the extra dice,
// empty when there are none.
func extraDiceName(extra int32) string {
	if extra < 1 {
		return ""
	}
	return brutalCriticalNamePT
}
