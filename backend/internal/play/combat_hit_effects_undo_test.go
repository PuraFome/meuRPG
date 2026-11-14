package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's undo of a damage that used a hit effect gives the effect back with the same mark and the same target. A
// marked_target is never spent, so its rows stay as they were; the part is offered again when the damage is rolled again.
func TestTheUndoOfAMarkedTargetHitKeepsTheMarkAndTheTarget(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	rowsBefore, _ := a.rows(t)
	hit := a.strike(t, e, battleaxe, "Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1)
	a.undoLast(t, e)
	if all, spent := a.rows(t); all != rowsBefore || spent != 0 || all != 2 {
		t.Errorf("after the undo: %d rows, %d spent; want the caster's row and the mark (%d), none spent", all, spent, rowsBefore)
	}
	if got := a.get(t, a.master); byLabel(t, got, "Goblin").GetId() == "" {
		t.Fatalf("the marked target is gone")
	}
	again := a.rollDamage(t, hit, 2, 2)
	if part := hitRoll(again); part == nil || part.GetSum() != 2 {
		t.Errorf("the damage rolled again reads the effect part %v, want it back with the same mark", part)
	}
}

// An effect that ended after the hit is never lost by undoing the hit. Ending the concentration is itself an action the
// master undoes first (it puts the row back with its id), and then the hit's undo gives it back unspent. The master's
// dismissal of the row closes the undo chain: the log offers no undo and a request for one is refused.
func TestTheUndoOfAHitThatUsedAnEffectNeverLosesTheEffectWhenItWasEnded(t *testing.T) {
	t.Parallel()
	newHit := func(t *testing.T) (*armed, *playv1.Encounter) {
		a := newHitCasters(t, nil, hitMod(trigNext))
		e := a.hitFight(t)
		a.castMark(t, e, 1)
		a.rollDamage(t, a.strike(t, e, battleaxe, "Goblin", d20(19)), 1, 1)
		if all, spent := a.rows(t); all != 1 || spent != 1 {
			t.Fatalf("after the hit: %d rows, %d spent, want the spent row", all, spent)
		}
		return a, e
	}
	t.Run("the concentration ended", func(t *testing.T) {
		t.Parallel()
		a, e := newHit(t)
		a.dropConcentration(t, e)
		a.undoLast(t, e) // the end of the concentration
		if all, spent := a.rows(t); all != 1 || spent != 1 {
			t.Fatalf("after undoing the end: %d rows, %d spent, want the spent row back", all, spent)
		}
		a.undoLast(t, a.get(t, a.master)) // the damage
		if all, spent := a.rows(t); all != 1 || spent != 0 {
			t.Errorf("after undoing the hit: %d rows, %d spent, want the effect back, waiting", all, spent)
		}
	})
	t.Run("the master dismissed the row", func(t *testing.T) {
		t.Parallel()
		a := newHitCasters(t, nil, hitMod(trigEach))
		e := a.hitFight(t)
		a.castMark(t, e, 1)
		a.rollDamage(t, a.strike(t, e, battleaxe, "Goblin", d20(19)), 1, 1)
		if err := a.coreEndEffect(t, e, "Pensantus", tableMark, playv1.EffectEndScope_EFFECT_END_SCOPE_THIS); err != nil {
			t.Fatalf("EndLastingEffect() error = %v", err)
		}
		e = a.get(t, a.master)
		if id := a.log(t, a.master, e).GetUndoableEventId(); id != "" {
			t.Errorf("the log offers to undo %s after the dismissal: the damage's undo would not bring the effect back", id)
		}
		if err := a.undo(t, a.master, e, "00000000-0000-4000-8000-000000000000"); connect.CodeOf(err) != connect.CodeAborted && connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("an undo after the dismissal: error = %v, want a refusal", err)
		}
		if all, _ := a.rows(t); all != 0 {
			t.Errorf("%d rows after the dismissal, want none", all)
		}
	})
}
