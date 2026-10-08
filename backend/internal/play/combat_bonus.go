package play

import (
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// flurryStrikes is how many unarmed strikes Flurry of Blows makes.
const flurryStrikes = 2

// flurryOfBlows is the monk's bonus action that spends 1 ki point for two
// unarmed strikes.
const flurryOfBlows = "feature:flurry-of-blows"

// traitsOf is what the bonus action rules read of a sheet attack.
func traitsOf(a link.Attack) combat.AttackTraits {
	return combat.AttackTraits{Spell: a.Spell, Melee: a.Melee, Light: a.Light, Unarmed: a.Unarmed, MartialArts: a.MartialArts}
}

// lastAttack is the sheet attack the combatant made last with its action this
// turn.
func lastAttack(sheet link.Sheet, c playdb.Combatant) (link.Attack, bool) {
	if c.ActionAttackKey == nil {
		return link.Attack{}, false
	}
	i := slices.IndexFunc(sheet.Attacks, func(a link.Attack) bool { return a.Key == *c.ActionAttackKey })
	if i < 0 {
		return link.Attack{}, false
	}
	return sheet.Attacks[i], true
}

// attackEconomy is what a player's attack (not a reaction) costs. It is the
// action when the action is free; the next attack of the Attack action when
// Extra Attack leaves one; the next beam of the cantrip the action cast; and
// otherwise the bonus action, when a bonus action attack rule lets it (Flurry
// of Blows, Martial Arts, Two-Weapon Fighting). It returns the rule that made
// it a bonus action attack, or the error that says why it cannot be made.
func attackEconomy(attacker playdb.Combatant, sheet link.Sheet, attack link.Attack) (combat.BonusKind, error) {
	actionUsed := errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
	last, hasLast := lastAttack(sheet, attacker)
	if attack.Spell {
		// A cantrip takes the whole action, whatever the Attack action did; the
		// beams of one cast (Eldritch Blast) are the exception.
		switch {
		case !attacker.ActionUsed:
			return combat.BonusNone, nil
		case attack.Beams > 1 && hasLast && last.Key == attack.Key && int(attacker.AttacksMade) < attack.Beams:
			return combat.BonusNone, nil
		}
		return combat.BonusNone, actionUsed
	}
	perAction := max(sheet.AttacksPerAction, 1)
	castCantrip := hasLast && last.Spell // the action went to a cantrip: no Attack action attacks left
	turn := combat.TurnState{ActionUsed: attacker.ActionUsed, AttacksMade: int(attacker.AttacksMade)}
	if !castCantrip && combat.AttacksLeft(perAction, turn) > 0 {
		return combat.BonusNone, nil
	}
	kind := combat.BonusAttack(combat.BonusAttackTurn{
		AttackAction: attacker.ActionUsed && attacker.AttacksMade > 0 && hasLast,
		FlurryLeft:   int(attacker.BonusAttacksLeft), Last: traitsOf(last),
	}, traitsOf(attack))
	switch {
	case kind == combat.BonusFlurry:
		return kind, nil
	case kind != combat.BonusNone && attacker.BonusActionUsed:
		return combat.BonusNone, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED, "the bonus action of this turn is used")
	case kind != combat.BonusNone:
		return kind, nil
	case !castCantrip && attacker.AttacksMade > 0 && perAction > 1:
		return combat.BonusNone, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ATTACKS_USED, "the Attack action made all its attacks")
	}
	return combat.BonusNone, actionUsed
}
