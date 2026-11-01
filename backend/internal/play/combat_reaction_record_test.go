package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A reaction records the kind and the round it was spent in (#643). The record is the master's, the
// owner's and a joint turn's, like reaction_used. These tests need the database
// (MEURPG_TEST_DATABASE_URL).

// reactionRecord is what the database holds of a combatant's reaction: whether it is spent, the kind and
// the round ("" and 0 for NULL).
func (a *armed) reactionRecord(t *testing.T, label string) (used bool, kind string, round int32) {
	t.Helper()
	var k *string
	var r *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT reaction_used, reaction_kind, reaction_round FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&used, &k, &r); err != nil {
		t.Fatalf("read the reaction of %s: %v", label, err)
	}
	if k != nil {
		kind = *k
	}
	if r != nil {
		round = *r
	}
	return used, kind, round
}

// wantRecord fails unless the combatant's reaction is spent as kind in round.
func (a *armed) wantRecord(t *testing.T, label, kind string, round int32) {
	t.Helper()
	if used, k, r := a.reactionRecord(t, label); !used || k != kind || r != round {
		t.Errorf("%s's reaction = (used %v, %q, round %d), want (used, %q, round %d)", label, used, k, r, kind, round)
	}
}

// wantNoRecord fails unless the combatant's reaction is free with no record.
func (a *armed) wantNoRecord(t *testing.T, label, when string) {
	t.Helper()
	if used, k, r := a.reactionRecord(t, label); used || k != "" || r != 0 {
		t.Errorf("%s %s: %s's reaction = (used %v, %q, round %d), want it free with no record", when, label, label, used, k, r)
	}
}

// wantViewRecord fails unless u reads the combatant's reaction as spent as kind in round.
func (a *armed) wantViewRecord(t *testing.T, u *user, label string, kind playv1.ReactionKind, round int32) {
	t.Helper()
	c := byLabel(t, a.get(t, u), label)
	if !c.GetReactionUsed() || c.GetReactionKind() != kind || c.GetReactionRound() != round {
		t.Errorf("a read of %s = (used %v, %v, round %d), want (used, %v, round %d)", label, c.GetReactionUsed(), c.GetReactionKind(), c.GetReactionRound(), kind, round)
	}
}

// wantNoViewRecord fails when u reads a kind or a round on the combatant.
func (a *armed) wantNoViewRecord(t *testing.T, u *user, label string) {
	t.Helper()
	c := byLabel(t, a.get(t, u), label)
	if c.GetReactionKind() != playv1.ReactionKind_REACTION_KIND_UNSPECIFIED || c.GetReactionRound() != 0 {
		t.Errorf("a read of %s holds (%v, round %d), want no record", label, c.GetReactionKind(), c.GetReactionRound())
	}
}

// TestShieldRecordsItsKindAndRound: the master uses the Mago's Shield; the master reads the kind and
// the round, the players read neither (an NPC's, RN-20).
func TestShieldRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	a, e, w := shieldedHit(t)
	a.wantNoRecord(t, "Mago", "before Shield")
	a.mustAnswer(t, a.master, e, w.GetId(), useAnswer(1))
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Mago", "shield", round)
	a.wantViewRecord(t, a.master, "Mago", playv1.ReactionKind_REACTION_KIND_SHIELD, round)
	for _, u := range []*user{a.ana, a.bia, a.caio} {
		a.wantNoViewRecord(t, u, "Mago")
	}
}

// TestAPlayersShieldIsReadByItsOwnerAndTheMasterAloneAndIsGivenBackByTheUndo: Pensantus's Shield is
// recorded and read by her player and the master, not by another player; the undo gives the reaction
// back with no record.
func TestAPlayersShieldIsReadByItsOwnerAndTheMasterAloneAndIsGivenBackByTheUndo(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFightNPCFirst(t)
	hit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9))
	if _, err := a.useReaction(t, a.ana, e, hit.GetPendingDamage().GetId(), slotOfLevel(1)); err != nil {
		t.Fatalf("UseReaction() error = %v", err)
	}
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Pensantus", "shield", round)
	for _, u := range []*user{a.master, a.ana} {
		a.wantViewRecord(t, u, "Pensantus", playv1.ReactionKind_REACTION_KIND_SHIELD, round)
	}
	for _, u := range []*user{a.caio, a.bia} {
		a.wantNoViewRecord(t, u, "Pensantus")
	}

	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	a.wantNoRecord(t, "Pensantus", "after the undo")
	for _, u := range []*user{a.master, a.ana} {
		a.wantNoViewRecord(t, u, "Pensantus")
	}
}

// TestTheTurnStartGivesTheReactionBackWithNoRecord: the reaction comes back at the start of the
// combatant's turn, and so does the empty record.
func TestTheTurnStartGivesTheReactionBackWithNoRecord(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFightNPCFirst(t)
	hit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9))
	if _, err := a.useReaction(t, a.ana, e, hit.GetPendingDamage().GetId(), slotOfLevel(1)); err != nil {
		t.Fatalf("UseReaction() error = %v", err)
	}
	a.wantRecord(t, "Pensantus", "shield", a.get(t, a.master).GetRound())
	a.passTo(t, e, "Pensantus")
	a.wantNoRecord(t, "Pensantus", "at the start of her turn")
}

// TestUncannyDodgeRecordsItsKindAndRound: the rogue's player reads it, the other players do not.
func TestUncannyDodgeRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, rogueFive)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil {
		t.Fatal("the rogue has no Uncanny Dodge window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Reator", "uncanny_dodge", round)
	for _, u := range []*user{a.master, a.ana} {
		a.wantViewRecord(t, u, "Reator", playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE, round)
	}
	for _, u := range []*user{a.caio, a.bia} {
		a.wantNoViewRecord(t, u, "Reator")
	}
}

// TestCounterspellRecordsItsKindAndRound: Pensantus counters the Mago Sombrio's cast.
func TestCounterspellRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	a, e := counterFight(t)
	if _, err := a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll); err != nil {
		t.Fatalf("CastSpell(the Mago Sombrio) error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if w == nil {
		t.Fatal("Pensantus has no Counterspell window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(3))
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Pensantus", "counterspell", round)
	a.wantViewRecord(t, a.ana, "Pensantus", playv1.ReactionKind_REACTION_KIND_COUNTERSPELL, round)
	a.wantNoViewRecord(t, a.caio, "Pensantus")
}

// TestAnOpportunityAttackRecordsItsKindAndRound: Goblin 1 attacks Toren as he leaves its reach.
func TestAnOpportunityAttackRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.oppFight(t)
	c.leaveGoblin(t)
	offer := c.offersOf(t, c.master)[0]
	c.wantNoRecord(t, "Goblin 1", "before the attack")
	if _, err := c.offerAttack(t, c.master, "Goblin 1", sword, "Toren", offer.GetId(), d20(15)); err != nil {
		t.Fatalf("the offer's attack error = %v", err)
	}
	round := c.get(t, c.master).GetRound()
	c.wantRecord(t, "Goblin 1", "opportunity", round)
	c.wantViewRecord(t, c.master, "Goblin 1", playv1.ReactionKind_REACTION_KIND_OPPORTUNITY, round)
	for _, u := range []*user{c.ana, c.bia, c.caio} {
		c.wantNoViewRecord(t, u, "Goblin 1")
	}
}

// TestAFeatureReactionRecordsItsKeyAndRound: Slow Fall, taken with TakeAction, records the feature's
// key. The view names no kind for it (it is no reaction window's), and the round says when.
func TestAFeatureReactionRecordsItsKeyAndRound(t *testing.T) {
	t.Parallel()
	r, e := monkFall(t)
	r.wantNoRecord(t, "Toren", "before Slow Fall")
	if _, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall"); err != nil {
		t.Fatalf("TakeAction(Slow Fall) error = %v", err)
	}
	round := r.get(t, r.master).GetRound()
	r.wantRecord(t, "Toren", "feature:slow-fall", round)
	r.wantViewRecord(t, r.caio, "Toren", playv1.ReactionKind_REACTION_KIND_UNSPECIFIED, round)
	r.wantViewRecord(t, r.master, "Toren", playv1.ReactionKind_REACTION_KIND_UNSPECIFIED, round)
	r.wantNoViewRecord(t, r.ana, "Toren")
}

// TestAnActionKeepsTheRecordOfASpentReactionAndItsUndoToo: an action of the turn that leaves the reaction
// spent leaves its record, and the undo of that action does too; the economy only clears the record when
// it gives the reaction back.
func TestAnActionKeepsTheRecordOfASpentReactionAndItsUndoToo(t *testing.T) {
	t.Parallel()
	a, e := takeActionFighters(t)
	toren := a.id(t, "Toren")
	a.execSQL(t, `UPDATE combatants SET reaction_used = true, reaction_kind = 'uncanny_dodge', reaction_round = 1 WHERE id = $1`, toren)
	if _, err := a.takeActionRaw(t, a.caio, e, "Toren", "standard:dodge", nil); err != nil {
		t.Fatalf("TakeAction(dodge) error = %v", err)
	}
	a.wantRecord(t, "Toren", "uncanny_dodge", 1)
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	a.wantRecord(t, "Toren", "uncanny_dodge", 1)
}
