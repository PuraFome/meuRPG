package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A next_hit effect goes to the earlier of two pending attacks of the attacker, so an earlier attack whose damage is never
// rolled keeps it from the later one for as long as it is open. What closes it is the end of the attacker's turn (the
// master passes it over and the damage is discarded), the master's undo of the attack and the end of the combat, and from then on
// the effect goes to the next hit.

func twoPendingHits(t *testing.T) (a *armed, e *playv1.Encounter, first, second *playv1.PendingDamage) {
	t.Helper()
	a = newHitParty(t, hitParty{class: "class:paladin", level: 5, cha: 16, spells: []hitSpellOf{{name: "Marca de Nanquim", key: tableMark, mods: []*tableMod{hitMod(trigNext)}}}})
	e = a.hitFight(t)
	a.castMark(t, e, 1)
	first = a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	second = a.strike(t, e, battleaxe, "Capitão Goblin", d20(18))
	return a, e, first, second
}

func TestAHitSpellTheEndOfTheTurnReleasesAnEarlierHitNeverRolled(t *testing.T) {
	t.Parallel()
	a, e, _, second := twoPendingHits(t)
	if r := hitRoll(a.rollDamage(t, second, 5)); r != nil {
		t.Fatalf("the later hit's roll has the part %v, want the effect kept for the earlier attack that never rolls", r)
	}
	e = a.nextTurn(t, e) // the master passes the turn over the damage still to roll: the earlier attack is discarded
	if n := a.count(t, `SELECT count(*) FROM pending_damages WHERE status = 'awaiting_roll'`); n != 0 {
		t.Fatalf("%d pending damages still wait for their roll after the turn passed, want none", n)
	}
	if _, spent := a.rows(t); spent != 0 {
		t.Fatalf("spent = %d, want the effect still there for the next hit", spent)
	}
	if r := hitRoll(a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 5, 3)); r == nil {
		t.Errorf("the hit of the next turn has no part of the spell, want the earlier attack's hold gone")
	}
	if _, spent := a.rows(t); spent != 1 {
		t.Errorf("spent = %d, want 1", spent)
	}
}

func TestAHitSpellTheUndoOfTheEarlierHitNeverRolledReleasesIt(t *testing.T) {
	t.Parallel()
	a, e, _, _ := twoPendingHits(t)
	a.undoLast(t, e) // the later attack
	a.undoLast(t, e) // the earlier one, whose damage never came
	if n := a.count(t, `SELECT count(*) FROM pending_damages WHERE status = 'awaiting_roll'`); n != 0 {
		t.Fatalf("%d pending damages still wait for their roll after the undo, want none", n)
	}
	if r := hitRoll(a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 5, 3)); r == nil {
		t.Errorf("the hit after the undo has no part of the spell, want the earlier attack's hold gone")
	}
}

// The combat's end leaves the pending damages as they were (nothing can roll them any more) and holds nothing of the next combat:
// the effect of a spell cast there is the first hit's.
func TestAHitSpellTheEndOfTheCombatLeavesNothingHeldForTheNextOne(t *testing.T) {
	t.Parallel()
	a, e, _, _ := twoPendingHits(t)
	a.endEncounter(t, e)
	e = a.hitFight(t)
	a.castMark(t, e, 1)
	if r := hitRoll(a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 5, 3)); r == nil {
		t.Errorf("the first hit of the next combat has no part of the spell, want the old combat's pending hits to hold nothing")
	}
}
