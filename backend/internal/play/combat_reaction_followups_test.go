package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The follow-ups of the reaction window (decisions-batch2-A item 4, "Sempre" as a mask, the held
// action that cannot happen).

// TestCuttingWordsOpensNoWindowForARollerThatCannotBeCut: a creature that cannot hear the bard
// gets no prompt at all, so the bard learns nothing about it from the question; one that can,
// gets it.
func TestCuttingWordsOpensNoWindowForARollerThatCannotBeCut(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, loreBard)
	a.setConditions(t, e, "Goblin", "condition:deafened")
	held := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(8))
	if held.GetRoll().GetHeldForReaction() || a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS) != nil {
		t.Fatal("a deaf Goblin's attack was held for the bard: the prompt tells what the Goblin is")
	}
	if held.GetRoll().GetD20() == nil {
		t.Error("the attack was not rolled")
	}

	b, be := reactorFight(t, loreBard)
	heard := b.mustAttack(t, b.master, be, "Goblin", sword, "Toren", d20(8))
	if !heard.GetRoll().GetHeldForReaction() || b.windowOf(t, b.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS) == nil {
		t.Error("a Goblin that hears the bard was not held for it")
	}
}

// TestSempreAsksTheMasterBeforeAMissedAttackToo: with "Reações dos inimigos: Sempre" a player's attack
// on an enemy waits for the master's check before the roll even when no bard could cut it, so a
// miss waits like a hit and the pause says nothing; a hit is asked once more on the hit (where a real
// Shield would ask), so the number of pauses is the same with and without a reaction.
func TestSempreAsksTheMasterBeforeAMissedAttackToo(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.setRules(t, enemyReactionsAlways)
	e := a.threeAndAGoblin(t)
	miss := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if !miss.GetRoll().GetHeldForReaction() {
		t.Fatalf("the missing attack = %v, want it held for the master's check", miss.GetRoll())
	}
	if got := a.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("Toren's player reads %q, want \"Esperando o mestre\"", got)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatal("the master has no check")
	}
	res := a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
	if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
		t.Errorf("windows after the answer: %d, want none (a miss is asked once)", n)
	}

	// A hit: asked before the roll and again on the hit.
	b := newCasters(t)
	b.setRules(t, enemyReactionsAlways)
	f := b.threeAndAGoblin(t)
	b.mustAttack(t, b.caio, f, "Toren", battleaxe, "Goblin", d20(15))
	b.mustAnswer(t, b.master, f, b.windowOf(t, b.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK).GetId(), passAnswer)
	w2 := b.windowOf(t, b.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w2 == nil {
		t.Fatal("the hit was not asked on the hit")
	}
	b.mustAnswer(t, b.master, f, w2.GetId(), passAnswer)
	if w := b.windowOf(t, b.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK); w != nil {
		t.Errorf("a third question: %v", w)
	}
}

// TestAHeldCastThatCannotHappenAnyMoreIsToldToTheMaster: the answers spent what they spent, the cast
// is refused on its replay, and the master reads in the log that it was dropped (nobody else does).
func TestAHeldCastThatCannotHappenAnyMoreIsToldToTheMaster(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	a.pensantusTurn(t, e)
	if _, err := a.castMissile(t, e); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	// The Goblin falls while the cast waits for the Mago's answer.
	if _, err := a.adjustHP(t, e, "Goblin", func(r *playv1.AdjustCombatantHitPointsRequest) {
		r.Change = &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: 7}
	}); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if w == nil {
		t.Fatal("no Counterspell window")
	}
	a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
	dropped := func(u *user) bool {
		for _, r := range a.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if strings.Contains(en.GetReactionTextPt(), "não pôde acontecer") {
					return true
				}
			}
		}
		return false
	}
	if !dropped(a.master) {
		t.Error("the master's log has no line for the dropped cast")
	}
	if dropped(a.ana) || dropped(a.caio) {
		t.Error("a player reads that a held cast was dropped")
	}
	if a.get(t, a.ana).GetReactionWait() != nil {
		t.Error("the caster still waits")
	}
}
