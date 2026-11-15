package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A character is asked once about a feature for each creature in each turn (docs/architecture/play/reactions.md "The feature
// window"): the answer, a pass included, holds until the turn is over, so a creature that goes in, out and in again of the range
// does not ask again. The turn's start and the move are the same question. These tests need the database.

// The goblin steps within range, Ana passes, the goblin steps out and in again in the same turn: no second window.
func TestADeclinedFeatureWindowHoldsWhenTheCreatureGoesOutAndInAgain(t *testing.T) {
	t.Parallel()
	m := haloTable(t, whenItMoves)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 8, 5)
	e = m.passTo(t, e, "Goblin")
	m.moveTo(t, e, "Goblin", 7, 5)
	w := m.windowOf(t, m.ana, kindFeature)
	if w == nil {
		t.Fatalf("Ana reads no FEATURE window when the goblin steps to 10 ft: %v", m.windows(t, m.ana))
	}
	m.mustAnswer(t, m.ana, e, w.GetId(), passAnswer)
	m.moveTo(t, e, "Goblin", 9, 5)
	m.moveTo(t, e, "Goblin", 7, 5)
	if again := m.windowOf(t, m.master, kindFeature); again != nil {
		t.Errorf("the goblin goes out and in again in the same turn and the window opens again: %v", again)
	}
}

// The turn's start and the move are one question: the answer to the first holds for the move.
func TestTheAnswerToTheTurnStartHoldsForTheMoveOfTheSameTurn(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 7, 5)
	e = m.passTo(t, e, "Goblin")
	w := m.windowOf(t, m.ana, kindFeature)
	if w == nil {
		t.Fatalf("Ana reads no FEATURE window when the goblin starts its turn at 10 ft: %v", m.windows(t, m.ana))
	}
	m.mustAnswer(t, m.ana, e, w.GetId(), passAnswer)
	m.moveTo(t, e, "Goblin", 9, 5)
	m.moveTo(t, e, "Goblin", 7, 5)
	if again := m.windowOf(t, m.master, kindFeature); again != nil {
		t.Errorf("the goblin moves in again after the answer to its turn's start and the window opens again: %v", again)
	}
}

// The creature's next turn asks again.
func TestTheFeatureWindowOpensAgainOnTheCreaturesNextTurn(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 7, 5)
	e = m.passTo(t, e, "Goblin")
	w := m.windowOf(t, m.ana, kindFeature)
	if w == nil {
		t.Fatalf("Ana reads no FEATURE window when the goblin starts its turn at 10 ft: %v", m.windows(t, m.ana))
	}
	m.mustAnswer(t, m.ana, e, w.GetId(), passAnswer)
	m.passTo(t, e, "Pensantus")
	m.passTo(t, e, "Goblin")
	if m.windowOf(t, m.ana, kindFeature) == nil {
		t.Errorf("the goblin starts its next turn at 10 ft and Ana is not asked again: %v", m.windows(t, m.ana))
	}
}

// Another creature is another question: the Capitão takes his turn with the goblin and is within range too, so Ana is asked about
// each of them, and the pass to the goblin does not answer for the Capitão.
func TestADeclinedFeatureWindowDoesNotHoldForAnotherCreature(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 7, 5)
	m.put(t, "Capitão Goblin", 7, 6)
	e = m.passTo(t, e, "Goblin")
	var goblin, captain *playv1.ReactionWindow
	for _, w := range m.windows(t, m.ana) {
		switch w.GetTrigger().GetActorLabel() {
		case "Goblin":
			goblin = w
		case "Capitão Goblin":
			captain = w
		}
	}
	if goblin == nil || captain == nil {
		t.Fatalf("Ana reads %v, want a window for the goblin and one for the Capitão", m.windows(t, m.ana))
	}
	m.mustAnswer(t, m.ana, e, goblin.GetId(), passAnswer)
	got := m.windowOf(t, m.ana, kindFeature)
	if got == nil || got.GetTrigger().GetActorLabel() != "Capitão Goblin" {
		t.Errorf("Ana reads %v after the pass to the goblin, want the Capitão's window still open", got)
	}
}

// The undo of the pass opens the window again and the question is still the one asked: the answer, given again, holds.
func TestTheUndoOfAPassLeavesTheFeatureWindowAskedOnce(t *testing.T) {
	t.Parallel()
	m := haloTable(t, whenItMoves)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 8, 5)
	e = m.passTo(t, e, "Goblin")
	m.moveTo(t, e, "Goblin", 7, 5)
	w := m.windowOf(t, m.ana, kindFeature)
	m.mustAnswer(t, m.ana, e, w.GetId(), passAnswer)
	m.undoLast(t, e)
	if m.windowOf(t, m.ana, kindFeature) == nil {
		t.Fatalf("the window is not open again after the pass was undone")
	}
	var open []*playv1.ReactionWindow
	for _, w := range m.windows(t, m.ana) {
		if w.GetKind() == kindFeature {
			open = append(open, w)
		}
	}
	if len(open) != 1 {
		t.Errorf("Ana reads %d FEATURE windows after the undo, want 1", len(open))
	}
}

// The window the undo took away was never asked: the move done again asks.
func TestTheUndoOfTheMoveThatOpenedTheFeatureWindowLeavesItUnasked(t *testing.T) {
	t.Parallel()
	m := haloTable(t, whenItMoves)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 8, 5)
	e = m.passTo(t, e, "Goblin")
	m.moveTo(t, e, "Goblin", 7, 5)
	m.undoLast(t, e)
	if m.windowOf(t, m.master, kindFeature) != nil {
		t.Fatalf("the window stands after the move was undone")
	}
	m.moveTo(t, e, "Goblin", 7, 5)
	if m.windowOf(t, m.ana, kindFeature) == nil {
		t.Errorf("the move done again after its undo opens no window: %v", m.windows(t, m.ana))
	}
}
