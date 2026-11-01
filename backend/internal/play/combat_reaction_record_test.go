package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A reaction records the kind and the round it was spent in (#643). The record is the owner's and the
// master's; a joint-turn partner reads reaction_used. These tests need the database
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

// TestAnActionKeepsTheRecordOfASpentReactionAndItsUndoToo: Pensantus spends her Shield on her own turn (a
// joint turn), takes the Dodge action and takes it back: the record stays through both, because the economy
// only clears it when it gives the reaction back.
func TestAnActionKeepsTheRecordOfASpentReactionAndItsUndoToo(t *testing.T) {
	t.Parallel()
	a, e := jointCasters(t)
	hit, err := a.attackAs(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9), true) // off turn: a reaction
	if err != nil {
		t.Fatalf("RollAttack(the Capitão as a reaction) error = %v", err)
	}
	if _, err := a.useReaction(t, a.ana, e, hit.GetPendingDamage().GetId(), slotOfLevel(1)); err != nil {
		t.Fatalf("UseReaction() error = %v", err)
	}
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Pensantus", "shield", round)
	if _, err := a.takeActionRaw(t, a.ana, e, "Pensantus", "standard:dodge", nil); err != nil {
		t.Fatalf("TakeAction(dodge) error = %v", err)
	}
	a.wantRecord(t, "Pensantus", "shield", round)
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	a.wantRecord(t, "Pensantus", "shield", round)
}

// TestEachReactionWindowRecordsItsKindAndRound: every site that spends a reaction through a window names
// its kind; dropping the record from any of them fails its row.
func TestEachReactionWindowRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name  string
		kind  string
		label string
		view  playv1.ReactionKind
		run   func(t *testing.T) (*armed, *user)
	}{
		{"Hellish Rebuke", "hellish_rebuke", "Reator", playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE, func(t *testing.T) (*armed, *user) {
			t.Helper()
			a, e := reactorFight(t, tieflingFighter)
			w := hurtReator(t, a, e)
			a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
				r.Answer, r.UseRacial = playv1.ReactionChoice_REACTION_CHOICE_USE, true
			})
			return a, a.ana
		}},
		{"Cutting Words", "cutting_words", "Reator", playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS, func(t *testing.T) (*armed, *user) {
			t.Helper()
			a, e := cutAndShield(t)
			a.mustAttack(t, a.master, e, "Goblin", sword, "Pensantus", d20(12))
			a.cut(t, e, 3)
			return a, a.bia
		}},
		{"Protection", "protection", "Reator", playv1.ReactionKind_REACTION_KIND_PROTECTION, func(t *testing.T) (*armed, *user) {
			t.Helper()
			a, e := protectionFight(t, protector())
			a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
			a.h.roller.queue(4)
			a.mustAnswer(t, a.ana, e, a.protectionWindow(t).GetId(), useAnswer(0))
			return a, a.ana
		}},
		{"Feather Fall", "feather_fall", "Pensantus", playv1.ReactionKind_REACTION_KIND_FEATHER_FALL, func(t *testing.T) (*armed, *user) {
			t.Helper()
			r, e := fallingRig(t)
			w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL)
			r.mustAnswer(t, r.ana, e, w.GetId(), func(q *playv1.AnswerReactionRequest) {
				q.Answer, q.Slot, q.CreatureIds = playv1.ReactionChoice_REACTION_CHOICE_USE, slotOfLevel(1), []string{r.id(t, "Toren")}
			})
			return r.armed, r.ana
		}},
		{"a reduction maneuver", "maneuver_reduce", "Pensantus", playv1.ReactionKind_REACTION_KIND_MANEUVER_REDUCE, func(t *testing.T) (*armed, *user) {
			t.Helper()
			m := newManeuverTable(t, "4")
			e := m.passTo(t, m.castersFight(t, 1), "Capitão Goblin")
			m.h.roller.queue(4)
			hit := m.mustAttack(t, m.master, e, "Capitão Goblin", sword, "Pensantus", d20(19))
			m.mustDamage(t, m.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
			w := m.windowOf(t, m.ana, playv1.ReactionKind_REACTION_KIND_MANEUVER_REDUCE)
			m.h.roller.queue(6)
			m.mustAnswer(t, m.ana, m.get(t, m.ana), w.GetId(), func(r *playv1.AnswerReactionRequest) {
				r.Answer, r.ManeuverKey = playv1.ReactionChoice_REACTION_CHOICE_USE, m.guard
				r.Roll = &playv1.AnswerReactionRequest_RollInApp{RollInApp: true}
			})
			return m.armed, m.ana
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a, owner := row.run(t)
			round := a.get(t, a.master).GetRound()
			a.wantRecord(t, row.label, row.kind, round)
			a.wantViewRecord(t, owner, row.label, row.view, round)
		})
	}
}

// TestDeflectMissilesAndItsThrowBackKeepOneRecord: answering Deflect Missiles records the reaction; the
// throw back is the same reaction and writes nothing, and the undo of the throw back leaves the record.
func TestDeflectMissilesAndItsThrowBackKeepOneRecord(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, monkThree)
	hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
	a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 8}
	})
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, "Reator", "deflect_missiles", round)
	if _, err := a.throwBack(t, a.ana, e, w.GetId(), "Reator", "Goblin", disadvantage(15)); err != nil {
		t.Fatalf("RollAttack(throw back) error = %v", err)
	}
	a.wantRecord(t, "Reator", "deflect_missiles", round)
	a.wantViewRecord(t, a.ana, "Reator", playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES, round)
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction(the throw back) error = %v", err)
	}
	a.wantRecord(t, "Reator", "deflect_missiles", round)
}

// TestAnOpportunityAttackByHandRecordsItsKindAndRound: an attack with as_reaction and no offer is the
// same reaction as the offered one.
func TestAnOpportunityAttackByHandRecordsItsKindAndRound(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.oppFight(t)
	e := c.get(t, c.master)
	if _, err := c.attackAs(t, c.master, e, "Goblin 1", sword, "Toren", d20(15), true); err != nil {
		t.Fatalf("RollAttack(Goblin 1 as a reaction) error = %v", err)
	}
	c.wantRecord(t, "Goblin 1", "opportunity", e.GetRound())
}

// TestTheUndoOfAReactionGivesTheRecordBack: the undo of an offered opportunity attack and of a feature
// reaction taken with TakeAction leaves the reaction free with no record.
func TestTheUndoOfAReactionGivesTheRecordBack(t *testing.T) {
	t.Parallel()
	t.Run("an opportunity attack", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.oppFight(t)
		c.leaveGoblin(t)
		offer := c.offersOf(t, c.master)[0]
		if _, err := c.offerAttack(t, c.master, "Goblin 1", sword, "Toren", offer.GetId(), d20(15)); err != nil {
			t.Fatalf("the offer's attack error = %v", err)
		}
		c.wantRecord(t, "Goblin 1", "opportunity", c.get(t, c.master).GetRound())
		c.undoLast(t)
		c.wantNoRecord(t, "Goblin 1", "after the undo")
	})
	t.Run("a feature reaction", func(t *testing.T) {
		t.Parallel()
		r, e := monkFall(t)
		if _, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall"); err != nil {
			t.Fatalf("TakeAction(Slow Fall) error = %v", err)
		}
		r.wantRecord(t, "Toren", "feature:slow-fall", r.get(t, r.master).GetRound())
		r.undoLast(t)
		r.wantNoRecord(t, "Toren", "after the undo")
	})
}

// jointCasters is the casters' table with Brisa, Toren and Pensantus tied in one joint turn, on turn, and the
// Capitão Goblin and the Goblin after them.
func jointCasters(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCasters(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId()}, {CharacterId: a.goblin.GetId()}},
		npcRolls: []int{1, 1},
		players:  map[string]int32{"Pensantus": 17, "Toren": 17, "Brisa": 16},
		reveal:   []string{"Capitão Goblin", "Goblin"},
		at:       map[string][2]int32{"Pensantus": {5, 5}, "Toren": {6, 5}, "Brisa": {5, 6}, "Capitão Goblin": {4, 4}, "Goblin": {9, 5}},
	})
	if len(e.GetTurnGroupIds()) < 2 {
		t.Fatalf("group = %q, want a joint turn", group(e))
	}
	return a, e
}

// TestAJointTurnPartnerReadsReactionUsedNotTheRecord: Pensantus, in a joint turn with Toren and Brisa, spends
// her Shield; her player and the master read its kind and round, the partners read that the reaction is used
// and nothing more; the players read nothing of an NPC's reaction.
func TestAJointTurnPartnerReadsReactionUsedNotTheRecord(t *testing.T) {
	t.Parallel()
	t.Run("a player in the group", func(t *testing.T) {
		t.Parallel()
		a, e := jointCasters(t)
		hit, err := a.attackAs(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9), true) // off turn: a reaction
		if err != nil {
			t.Fatalf("RollAttack(the Capitão as a reaction) error = %v", err)
		}
		if _, err := a.useReaction(t, a.ana, e, hit.GetPendingDamage().GetId(), slotOfLevel(1)); err != nil {
			t.Fatalf("UseReaction() error = %v", err)
		}
		round := a.get(t, a.master).GetRound()
		for _, u := range []*user{a.master, a.ana} {
			a.wantViewRecord(t, u, "Pensantus", playv1.ReactionKind_REACTION_KIND_SHIELD, round)
		}
		for _, u := range []*user{a.caio, a.bia} {
			if !byLabel(t, a.get(t, u), "Pensantus").GetReactionUsed() {
				t.Errorf("a partner does not read that Pensantus's reaction is used")
			}
			a.wantNoViewRecord(t, u, "Pensantus")
		}
	})
	t.Run("an NPC out of the group", func(t *testing.T) {
		t.Parallel()
		a, e := jointCasters(t)
		if _, err := a.attackAs(t, a.master, e, "Capitão Goblin", sword, "Toren", d20(15), true); err != nil {
			t.Fatalf("RollAttack(the Capitão as a reaction) error = %v", err)
		}
		a.wantViewRecord(t, a.master, "Capitão Goblin", playv1.ReactionKind_REACTION_KIND_OPPORTUNITY, a.get(t, a.master).GetRound())
		for _, u := range []*user{a.ana, a.bia, a.caio} {
			a.wantNoViewRecord(t, u, "Capitão Goblin")
		}
	})
}

// TestParryRecordsItsKindAndRoundAndTheUndoClearsIt: the master answers the Knight's Parry. The master reads
// the kind and the round, no player reads an NPC's, and the undo gives the reaction back with no record.
func TestParryRecordsItsKindAndRoundAndTheUndoClearsIt(t *testing.T) {
	t.Parallel()
	a, e, _, w := parriedHit(t, knightHit)
	a.wantNoRecord(t, knight, "before Parry")
	a.mustAnswer(t, a.master, e, w.GetId(), useAnswer(0))
	round := a.get(t, a.master).GetRound()
	a.wantRecord(t, knight, "parry", round)
	a.wantViewRecord(t, a.master, knight, playv1.ReactionKind_REACTION_KIND_PARRY, round)
	for _, u := range []*user{a.ana, a.bia, a.caio} {
		a.wantNoViewRecord(t, u, knight)
	}
	a.undoLast(t, e)
	a.wantNoRecord(t, knight, "after the undo")
	a.wantNoViewRecord(t, a.master, knight)
}
