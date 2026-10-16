package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The branches of the reaction answers (PM-04) that the first reaction tests did not reach:
// each refusal with its code and message, each way a reaction can end. Frozen as they are; a
// finding is commented where the behaviour looks odd.

// wantReactionRefusal fails unless err is a connect error with this code and message.
func wantReactionRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s error = %v, want a connect error %v %q", call, err, code, msg)
	}
	if ce.Code() != code || ce.Message() != msg {
		t.Errorf("%s error = %v %q, want %v %q", call, ce.Code(), ce.Message(), code, msg)
	}
}

// useWith is an answer that uses the reaction with the given edits.
func useWith(edit func(*playv1.AnswerReactionRequest)) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		edit(r)
	}
}

// typedAnswer uses the reaction with a die typed from a physical one.
func typedAnswer(face int32) func(*playv1.AnswerReactionRequest) {
	return useWith(func(r *playv1.AnswerReactionRequest) { r.Roll = &playv1.AnswerReactionRequest_Typed{Typed: face} })
}

// pactWarlock is the Reator of reactorFight as a warlock of the 3rd level who knows Hellish
// Rebuke: its slots are pact slots of the 2nd level.
func pactWarlock(t *testing.T, a *armed) *charactersv1.Character {
	t.Helper()
	return a.ana.castingHero(t, a.campaignID, "Reator", classLevel("class:warlock", 3, ""), abilities(10, 14, 12, 10, 16), nil, []string{spellHellishRebuke}, nil, "")
}

// hurtReator has the Goblin hit the Reator and the master land the damage, which opens the
// Reator's Hellish Rebuke window, and returns the window.
func hurtReator(t *testing.T, a *armed, e *playv1.Encounter) *playv1.ReactionWindow {
	t.Helper()
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	if _, err := a.settle(t, a.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if w == nil {
		t.Fatal("the Reator has no Hellish Rebuke window after being hurt")
	}
	return w
}

// TestHellishRebukeThroughAPactSlotAsksTheMastersSave: the warlock pays a pact slot of the 2nd
// level (3d10), the Goblin's saving throw is the master's, a success halves the fire; each
// wrong answer is refused with its words and spends nothing.
func TestHellishRebukeThroughAPactSlotAsksTheMastersSave(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, pactWarlock)
	w := hurtReator(t, a, e)
	opts := w.GetHellishRebuke().GetOptions()
	if len(opts) != 1 || opts[0].GetRacial() || opts[0].GetLevel() != 2 {
		t.Fatalf("the options = %v, want one pact slot of the 2nd level", opts)
	}

	// Refusals: no slot named, a slot the window does not offer, the Infernal Legacy of a
	// warlock who has none.
	_, err := a.answerReaction(t, a.ana, e, w.GetId(), useWith(func(*playv1.AnswerReactionRequest) {}))
	wantReactionRefusal(t, "Hellish Rebuke with no slot", err, connect.CodeInvalidArgument, "slot is not one of the free slots the window offers")
	_, err = a.answerReaction(t, a.ana, e, w.GetId(), useWith(func(r *playv1.AnswerReactionRequest) { r.Slot = slotOfLevel(5) }))
	wantReactionRefusal(t, "Hellish Rebuke with a slot of the 5th level", err, connect.CodeInvalidArgument, "slot is not one of the free slots the window offers")
	_, err = a.answerReaction(t, a.ana, e, w.GetId(), useWith(func(r *playv1.AnswerReactionRequest) { r.UseRacial = true }))
	wantBlockedBy(t, "Hellish Rebuke through a legacy the warlock lacks", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	wantReactionRefusal(t, "Hellish Rebuke through a legacy the warlock lacks", err, connect.CodeFailedPrecondition, "the Infernal Legacy has no use left")

	res := a.mustAnswer(t, a.ana, e, w.GetId(), useWith(func(r *playv1.AnswerReactionRequest) {
		r.Slot = &playv1.SpellSlot{Level: 2, Pact: true}
	}))
	if res.GetResult().GetNextWindowId() != w.GetId() {
		t.Fatalf("the result = %v, want the window to go on to the Goblin's save", res.GetResult())
	}
	if used := a.vitals(t, a.pens).GetPactSlots().GetUsed(); used != 1 {
		t.Errorf("pact slots used = %d, want 1", used)
	}

	// The second step: only the master answers, and with a roll.
	save := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if save == nil || !save.GetSecondStep() {
		t.Fatalf("the master's window = %v, want the second step", save)
	}
	_, err = a.answerReaction(t, a.master, e, save.GetId(), passAnswer)
	wantReactionRefusal(t, "passing the aggressor's save", err, connect.CodeInvalidArgument, "the saving throw is rolled: use roll_in_app or typed")
	a.h.roller.queue(8, 9, 7) // the fire: 3d10
	done := a.mustAnswer(t, a.master, e, save.GetId(), typedAnswer(20))
	r := done.GetResult().GetHellishRebuke()
	if !r.GetSaved() || r.GetDamage() != 12 || r.GetRacial() || r.GetLevel() != 2 {
		t.Errorf("the result = %v, want a saved throw and half of 24 fire at the 2nd level", r)
	}
}

// TestPassingAHellishRebukeAnswersItAndSpendsNothing: "Deixar passar" closes the window with
// no slot and no reaction spent.
func TestPassingAHellishRebukeAnswersItAndSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, pactWarlock)
	w := hurtReator(t, a, e)
	res := a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	if res.GetResult().GetUsed() || res.GetResult().GetKind() != playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE {
		t.Errorf("the result = %v, want Hellish Rebuke passed", res.GetResult())
	}
	if used := a.vitals(t, a.pens).GetPactSlots().GetUsed(); used != 0 {
		t.Errorf("pact slots used = %d, want 0", used)
	}
	if byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
		t.Error("the Reator's reaction is spent by a pass")
	}
	if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
		t.Errorf("windows left: %d", n)
	}
}

// setResourceUsed has the master set how many uses of a resource a character has spent.
func (a *armed) setResourceUsed(t *testing.T, character *charactersv1.Character, key string, used int32) {
	t.Helper()
	if _, err := a.master.adjust(t, a.campaignID, character.GetId(), func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: key, Used: used}}
	}); err != nil {
		t.Fatalf("AdjustCharacterVitals(%s) error = %v", key, err)
	}
}

// bardWindow has the Goblin attack Toren with this d20 face (+4 to hit) and returns the bard's window.
func bardWindow(t *testing.T, a *armed, e *playv1.Encounter, face int32) *playv1.ReactionWindow {
	t.Helper()
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(face))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatal("the bard has no Cutting Words window")
	}
	return w
}

// TestCuttingWordsRefusalsAndAnEffectlessCut: a typed die out of range is refused, a bard
// whose uses ran out meanwhile is refused with NO_SLOT, and a roller that went deaf after the
// window opened spends the use and the reaction for nothing ("sem efeito").
func TestCuttingWordsRefusalsAndAnEffectlessCut(t *testing.T) {
	t.Parallel()
	t.Run("a die out of range, then no use left", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, loreBard)
		w := bardWindow(t, a, e, 8)
		_, err := a.answerReaction(t, a.ana, e, w.GetId(), typedAnswer(9))
		wantReactionRefusal(t, "Cutting Words with a typed 9 on a d6", err, connect.CodeInvalidArgument, "typed must be 1 to 6")
		a.setResourceUsed(t, a.pens, "bardic_inspiration", 3)
		_, err = a.answerReaction(t, a.ana, e, w.GetId(), typedAnswer(4))
		wantBlockedBy(t, "Cutting Words with no use left", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT)
		wantReactionRefusal(t, "Cutting Words with no use left", err, connect.CodeFailedPrecondition, "no Bardic Inspiration use is left")
	})
	t.Run("a roller that cannot hear", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, loreBard)
		w := bardWindow(t, a, e, 8)
		a.setConditions(t, e, "Goblin", "condition:deafened")
		res := a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(5))
		cw := res.GetResult().GetCuttingWords()
		if cw.GetEffective() || !res.GetResult().GetUsed() {
			t.Errorf("the result = %v, want the use spent and no effect", res.GetResult())
		}
		if left := 3 - func() int32 { used, _ := resourceUsed(a.vitals(t, a.pens), "bardic_inspiration"); return used }(); left != 2 {
			t.Errorf("Bardic Inspiration uses left = %d, want 2", left)
		}
		if !byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
			t.Error("the bard's reaction is not spent")
		}
	})
	t.Run("a cut that leaves the hit", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, loreBard)
		w := bardWindow(t, a, e, 14)
		res := a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(1)) // 18 - 1 = 17 still reaches the armor class
		cw := res.GetResult().GetCuttingWords()
		if !cw.GetEffective() || cw.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || cw.GetDie().GetTotal() != 1 {
			t.Errorf("the result = %v, want the die 1 to leave a hit", res.GetResult())
		}
	})
}

// TestCuttingWordsOnADamageRollCutsTheHeldDamage: the "todos" setting holds the damage roll; the
// die comes off the damage that lands.
func TestCuttingWordsOnADamageRollCutsTheHeldDamage(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, bardAsking(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL))
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(19))
	a.mustAnswer(t, a.ana, e, a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS).GetId(), passAnswer)
	pend := a.firstPending(t)
	a.mustDamage(t, a.master, e, pend, typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil || w.GetCuttingWords().GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_DAMAGE {
		t.Fatalf("the bard's window = %v, want a Cutting Words on the damage", w)
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(2))
	cw := res.GetResult().GetCuttingWords()
	if !cw.GetEffective() || cw.GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_DAMAGE || cw.GetDie().GetTotal() != 2 {
		t.Errorf("the result = %v, want a die 2 taken off the damage", res.GetResult())
	}
	if status, amount := a.pendingOf(t, pend); status != "rolled" || amount != 6 {
		t.Errorf("the pending damage = %s for %d, want rolled for 6 (8 less the die 2)", status, amount)
	}
}

// TestDeflectMissilesEveryWayItEnds: a reduction that leaves damage ends the window; a caught
// arrow asks the throw back, which "use" refuses, which a monk with no ki cannot make, and which
// "Guardar a flecha" answers.
func TestDeflectMissilesEveryWayItEnds(t *testing.T) {
	t.Parallel()
	t.Run("a reduction that leaves damage", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, monkThree)
		hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
		a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
		w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
		res := a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(1)) // 1 + 3 + 3 = 7 of 8
		d := res.GetResult().GetDeflectMissiles()
		if d.GetCaught() || d.GetDamageAfter() != 1 || d.GetThrowBackAvailable() || res.GetResult().GetNextWindowId() != "" {
			t.Errorf("the result = %v, want 1 damage left and no throw back", res.GetResult())
		}
		if status, amount := a.pendingOf(t, hit.GetPendingDamage().GetId()); status != "rolled" || amount != 1 {
			t.Errorf("the pending damage = %s for %d, want rolled for 1", status, amount)
		}
	})
	t.Run("the second step", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, monkThree)
		hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
		a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
		w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
		a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(8))
		_, err := a.answerReaction(t, a.ana, e, w.GetId(), useWith(func(*playv1.AnswerReactionRequest) {}))
		wantReactionRefusal(t, "using the second step", err, connect.CodeInvalidArgument, "the missile is thrown back with RollAttack and catch_window_id")
		// A throw back through a window that is not the monk's second step.
		_, err = a.throwBack(t, a.ana, e, "00000000-0000-4000-8000-000000000001", "Reator", "Goblin", disadvantage(15))
		wantBlockedBy(t, "the throw back of a window that does not exist", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN_TO_ANSWER)
		// The ki is gone meanwhile.
		a.setResourceUsed(t, a.pens, "ki", 3)
		_, err = a.throwBack(t, a.ana, e, w.GetId(), "Reator", "Goblin", disadvantage(15))
		wantReactionRefusal(t, "the throw back with no ki", err, connect.CodeFailedPrecondition, "no ki left to throw the missile back")
		res := a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
		if !res.GetResult().GetUsed() || res.GetResult().GetKind() != playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES {
			t.Errorf("the result = %v, want the arrow kept", res.GetResult())
		}
		if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
			t.Errorf("windows left: %d", n)
		}
	})
}

// TestPassingUncannyDodgeAnswersItAndSpendsNothing.
func TestPassingUncannyDodgeAnswersItAndSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, rogueFive)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	res := a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	if res.GetResult().GetUsed() || byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
		t.Errorf("the result = %v, want a pass that spends nothing", res.GetResult())
	}
	if status, amount := a.pendingOf(t, hit.GetPendingDamage().GetId()); status != "rolled" || amount != 8 {
		t.Errorf("the pending damage = %s for %d, want rolled for 8", status, amount)
	}
}

// TestFeatherFallRefusalsKeepTheirWords: the creatures a wizard saves are checked before the slot,
// and the same creature named twice is saved once.
func TestFeatherFallRefusalsKeepTheirWords(t *testing.T) {
	t.Parallel()
	r, e := fallingRig(t)
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL)
	if w == nil {
		t.Fatal("no Feather Fall window")
	}
	toren := r.id(t, "Toren")
	use := func(slot *playv1.SpellSlot, ids ...string) func(*playv1.AnswerReactionRequest) {
		return useWith(func(q *playv1.AnswerReactionRequest) { q.Slot, q.CreatureIds = slot, ids })
	}
	const notOfTheFall = "creature_ids must be creatures of the fall"
	_, err := r.answerReaction(t, r.ana, e, w.GetId(), use(slotOfLevel(1)))
	wantReactionRefusal(t, "Feather Fall on nobody", err, connect.CodeInvalidArgument, "creature_ids must name 1 to 5 falling creatures")
	_, err = r.answerReaction(t, r.ana, e, w.GetId(), use(slotOfLevel(1), r.id(t, "Pensantus")))
	wantReactionRefusal(t, "Feather Fall on a creature that does not fall", err, connect.CodeInvalidArgument, notOfTheFall)
	_, err = r.answerReaction(t, r.ana, e, w.GetId(), use(slotOfLevel(1), "not-a-creature"))
	wantReactionRefusal(t, "Feather Fall on a creature that does not exist", err, connect.CodeInvalidArgument, notOfTheFall)
	_, err = r.answerReaction(t, r.ana, e, w.GetId(), use(nil, toren))
	wantReactionRefusal(t, "Feather Fall with no slot", err, connect.CodeInvalidArgument, "slot is not one of the free slots the window offers")
	_, err = r.answerReaction(t, r.ana, e, w.GetId(), use(slotOfLevel(5), toren))
	wantReactionRefusal(t, "Feather Fall with a slot of the 5th level", err, connect.CodeInvalidArgument, "slot is not one of the free slots the window offers")
	res := r.mustAnswer(t, r.ana, e, w.GetId(), use(slotOfLevel(1), toren, toren))
	if got := res.GetResult().GetFeatherFall().GetSavedIds(); len(got) != 1 || got[0] != toren {
		t.Errorf("the result = %v, want Toren saved once", res.GetResult())
	}
}

// openPitRig is fallingRig before anyone fell: the cave, the pit on (9, 7) and the combat begun.
func openPitRig(t *testing.T) (*trapRig, *playv1.Encounter, *mapsv1.MapPoint) {
	t.Helper()
	r := newTrapRig(t)
	pt := r.trap(t, "Fosso", 9, 7, pit("1d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	return r, r.fight(t), pt
}

// threeFall has the master put two Goblins next to the pit (no creature of the NPCs' fires a trap
// by walking, so the master fires it by hand) and fire it on Toren and both: it returns the
// wizard's Feather Fall window.
func threeFall(t *testing.T, r *trapRig, pt *mapsv1.MapPoint) *playv1.ReactionWindow {
	t.Helper()
	for label, sq := range map[string][2]int32{"Goblin 1": {10, 7}, "Goblin 2": {11, 7}} {
		if _, err := r.moveResult(t, r.master, label, sq[0], sq[1]); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}
	r.h.roller.queue(2, 3, 4)
	if _, err := r.fireByHand(t, pt, r.id(t, "Toren"), r.id(t, "Goblin 1"), r.id(t, "Goblin 2")); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL)
	if w == nil || len(w.GetFeatherFall().GetFalling()) != 3 {
		t.Fatalf("the wizard's window = %v, want three falling creatures", w)
	}
	return w
}

// TestTheWizardCanSlowItsOwnFall: the master puts the wizard in the pit; it is one of the
// falling creatures and needs no sight of itself to save itself.
func TestTheWizardCanSlowItsOwnFall(t *testing.T) {
	t.Parallel()
	r, e, _ := openPitRig(t)
	r.h.roller.queue(3)
	if _, err := r.moveResult(t, r.master, "Pensantus", 9, 7); err != nil {
		t.Fatalf("MoveCombatant(Pensantus) error = %v", err)
	}
	var own *playv1.ReactionWindow
	for _, w := range r.windows(t, r.ana) {
		if w.GetKind() == playv1.ReactionKind_REACTION_KIND_FEATHER_FALL {
			for _, f := range w.GetFeatherFall().GetFalling() {
				if f.GetLabel() == "Pensantus" {
					own = w
				}
			}
		}
	}
	if own == nil {
		t.Fatalf("the wizard has no window for its own fall: %v", r.windows(t, r.ana))
	}
	res := r.mustAnswer(t, r.ana, e, own.GetId(), useWith(func(q *playv1.AnswerReactionRequest) {
		q.Slot, q.CreatureIds = slotOfLevel(1), []string{r.id(t, "Pensantus")}
	}))
	if got := res.GetResult().GetFeatherFall().GetSavedIds(); len(got) != 1 || got[0] != r.id(t, "Pensantus") {
		t.Errorf("the result = %v, want Pensantus saved", res.GetResult())
	}
	if status, _ := r.fallOf(t, "Pensantus"); status != "discarded" {
		t.Errorf("Pensantus's fall damage is %s, want it discarded", status)
	}
}

// TestWhereAFallLandsWhenTheWindowIsPassed: an NPC's fall damage is applied to its hit points as soon
// as no window is left, a player's character's waits for the master.
func TestWhereAFallLandsWhenTheWindowIsPassed(t *testing.T) {
	t.Parallel()
	r, e, pt := openPitRig(t)
	w := threeFall(t, r, pt)
	for _, label := range []string{"Toren", "Goblin 1", "Goblin 2"} {
		if status, _ := r.fallOf(t, label); status != "awaiting_reaction" {
			t.Errorf("%s's fall = %s, want it waiting for the reaction", label, status)
		}
	}
	r.mustAnswer(t, r.ana, e, w.GetId(), passAnswer)
	for label, want := range map[string]struct {
		status string
		amount int32
	}{"Toren": {"rolled", 2}, "Goblin 1": {"applied", 3}, "Goblin 2": {"applied", 4}} {
		if status, amount := r.fallOf(t, label); status != want.status || amount != want.amount {
			t.Errorf("%s's fall = %s for %d, want %s for %d", label, status, amount, want.status, want.amount)
		}
	}
	if hp := byLabel(t, r.get(t, r.master), "Goblin 1").GetHitPointsCurrent(); hp != 4 {
		t.Errorf("Goblin 1's hit points = %d, want 7 less 3", hp)
	}
}

// TestFeatherFallSavesOnlyWhomItNames: the creatures left out of the answer take their damage
// when the window closes.
func TestFeatherFallSavesOnlyWhomItNames(t *testing.T) {
	t.Parallel()
	r, e, pt := openPitRig(t)
	w := threeFall(t, r, pt)
	r.mustAnswer(t, r.ana, e, w.GetId(), useWith(func(q *playv1.AnswerReactionRequest) {
		q.Slot, q.CreatureIds = slotOfLevel(1), []string{r.id(t, "Goblin 1"), r.id(t, "Toren")}
	}))
	for label, want := range map[string]struct {
		status string
		amount int32
	}{"Toren": {"discarded", 2}, "Goblin 1": {"discarded", 3}, "Goblin 2": {"applied", 4}} {
		if status, amount := r.fallOf(t, label); status != want.status || amount != want.amount {
			t.Errorf("%s's fall = %s for %d, want %s for %d", label, status, amount, want.status, want.amount)
		}
	}
}

// removeCombatant has the master take a combatant out of the combat.
func (a *armed) removeReactionCombatant(t *testing.T, e *playv1.Encounter, label string) {
	t.Helper()
	if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant(%s) error = %v", label, err)
	}
}

// TestAWindowClosesByItselfWhenWhatOpenedItIsGone: each way the trigger of an open window goes away
// closes it at the next change of the combat, and nobody is asked anything.
func TestAWindowClosesByItselfWhenWhatOpenedItIsGone(t *testing.T) {
	t.Parallel()
	t.Run("the attacker left the fight", func(t *testing.T) {
		t.Parallel()
		a, e := reactorFight(t, loreBard)
		bardWindow(t, a, e, 8)
		a.removeReactionCombatant(t, e, "Goblin")
		if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS); w != nil {
			t.Errorf("the bard still has a window: %v", w)
		}
		if got := a.get(t, a.caio).GetReactionWait(); got != nil {
			t.Errorf("Toren's player still waits: %v", got)
		}
	})
	t.Run("the reactor left the fight", func(t *testing.T) {
		t.Parallel()
		a, e := mageFight(t)
		a.pensantusTurn(t, e)
		if _, err := a.castMissile(t, e); err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
		a.removeReactionCombatant(t, e, "Mago")
		if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL); w != nil {
			t.Errorf("the master still has a Counterspell window: %v", w)
		}
	})
	t.Run("the caster of a held spell is down: the window stays", func(t *testing.T) {
		t.Parallel()
		a, e := mageFight(t)
		a.pensantusTurn(t, e)
		if _, err := a.castMissile(t, e); err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
		if _, err := a.master.adjust(t, a.campaignID, a.pens.GetId(), func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(0) }); err != nil {
			t.Fatalf("AdjustCharacterVitals() error = %v", err)
		}
		// Frozen: only a defeated combatant (an NPC's flag) closes the window of its held cast; a player's
		// character at 0 hit points is down, not defeated, and the master still answers for the Mago.
		if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL); w == nil {
			t.Error("the Counterspell window closed when its caster went down")
		}
	})
	t.Run("the hit was settled meanwhile", func(t *testing.T) {
		t.Parallel()
		a, e, _ := shieldedHit(t)
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE pending_damages SET status = 'discarded'`); err != nil {
			t.Fatalf("discard the hit: %v", err)
		}
		a.setConditions(t, e, "Goblin", "condition:frightened") // any change of the combat settles the windows
		if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD); w != nil {
			t.Errorf("the master still has a Shield window: %v", w)
		}
	})
	t.Run("the held action was dropped", func(t *testing.T) {
		t.Parallel()
		a, e := mageFight(t)
		a.pensantusTurn(t, e)
		if _, err := a.castMissile(t, e); err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE reaction_holds SET state = 'dropped'`); err != nil {
			t.Fatalf("drop the hold: %v", err)
		}
		a.setConditions(t, e, "Goblin", "condition:frightened")
		if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL); w != nil {
			t.Errorf("the master still has a Counterspell window: %v", w)
		}
	})
	t.Run("the concentration ended", func(t *testing.T) {
		t.Parallel()
		a, e := concentratingHit(t, 6)
		if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
			t.Fatal("the concentration save did not open")
		}
		if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
		})); err != nil {
			t.Fatalf("EndConcentration() error = %v", err)
		}
		if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE); w != nil {
			t.Errorf("the master still has a concentration save: %v", w)
		}
	})
}

// TestTheWindowsOfOneTriggerAreAnsweredInOrder: two bards may cut the same roll; the one that is
// not first in the initiative order is told it is not its turn to answer.
func TestTheWindowsOfOneTriggerAreAnsweredInOrder(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = a.ana.heroWith(t, a.campaignID, "Reator", classLevel("class:bard", 3, "subclass:lore"), abilities(10, 14, 12, 10, 16), []string{rapier}, nil, nil)
		a.bri = a.bia.heroWith(t, a.campaignID, "Brisa", classLevel("class:bard", 3, "subclass:lore"), abilities(10, 14, 12, 10, 16), []string{rapier}, nil, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {5, 4}},
	})
	e = a.passTo(t, e, "Goblin")
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(8))
	first := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	second := a.windowOf(t, a.bia, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if first == nil || second == nil {
		t.Fatalf("the windows = %v / %v, want one for each bard", first, second)
	}
	_, err := a.answerReaction(t, a.bia, e, second.GetId(), passAnswer)
	wantBlockedBy(t, "the second bard answering first", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN_TO_ANSWER)
	a.mustAnswer(t, a.ana, e, first.GetId(), passAnswer)
	a.mustAnswer(t, a.bia, e, second.GetId(), passAnswer)
}

// shieldedPensantus is the Capitão hitting Pensantus, whose hit waits for her Escudo; it returns
// the pending damage.
func shieldedPensantus(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a := newCasters(t)
	e := a.castersFightNPCFirst(t)
	hit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9)) // 9 + 4 = 13 against 12
	return a, e, hit.GetPendingDamage().GetId()
}

// TestUseReactionRefusalsAndTheMastersAnswer: a pending damage that does not exist, a caster whose
// slots ran out meanwhile, and the master answering for the player (who gets the pending damage
// back, which the player's own answer never carries).
func TestUseReactionRefusalsAndTheMastersAnswer(t *testing.T) {
	t.Parallel()
	t.Run("a pending damage that does not exist", func(t *testing.T) {
		t.Parallel()
		a, e, _ := shieldedPensantus(t)
		_, err := a.useReaction(t, a.ana, e, "00000000-0000-4000-8000-000000000001", slotOfLevel(1))
		wantReactionRefusal(t, "UseReaction on a missing pending damage", err, connect.CodeNotFound, "pending damage not found")
	})
	t.Run("no slot left", func(t *testing.T) {
		t.Parallel()
		a, e, pending := shieldedPensantus(t)
		if _, err := a.master.adjust(t, a.campaignID, a.pens.GetId(), func(r *playv1.AdjustCharacterVitalsRequest) {
			r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 1, Used: 4}, {Level: 2, Used: 2}}
		}); err != nil {
			t.Fatalf("AdjustCharacterVitals() error = %v", err)
		}
		_, err := a.useReaction(t, a.ana, e, pending, slotOfLevel(1))
		b := wantBlockedBy(t, "UseReaction with no slot left", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT)
		if b.GetMinLevel() != 1 {
			t.Errorf("the refusal's minimum level = %d, want 1", b.GetMinLevel())
		}
		wantReactionRefusal(t, "UseReaction with no slot left", err, connect.CodeFailedPrecondition, "there is no free spell slot for Escudo")
	})
	t.Run("the master answers for the player", func(t *testing.T) {
		t.Parallel()
		a, e, pending := shieldedPensantus(t)
		res, err := a.useReaction(t, a.master, e, pending, slotOfLevel(1))
		if err != nil {
			t.Fatalf("UseReaction() error = %v", err)
		}
		if res.GetOutcome() != playv1.ReactionOutcome_REACTION_OUTCOME_STOPPED || res.GetPendingDamage() == nil {
			t.Errorf("UseReaction() = %v, want the attack stopped and the pending damage for the master", res)
		}
	})
}

// damageFight is a fight where Toren (made by build) is the target of the Goblin and the Reator is
// a Lore bard who asks on every roll.
func damageFight(t *testing.T, toren func(t *testing.T, a *armed) *charactersv1.Character) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = toren(t, a)
		a.pens = bardAsking(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)(t, a)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	return a, a.passTo(t, e, "Goblin")
}

// TestTheWindowsOfADamageAreAnsweredCuttingWordsFirst: the bard's window comes before the target's own
// reaction (Uncanny Dodge, Deflect Missiles), whatever the initiative.
func TestTheWindowsOfADamageAreAnsweredCuttingWordsFirst(t *testing.T) {
	t.Parallel()
	rogue := func(t *testing.T, a *armed) *charactersv1.Character {
		t.Helper()
		return a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 5, abilities(10, 14, 12, 10, 8), []string{rapier}, nil)
	}
	monk := func(t *testing.T, a *armed) *charactersv1.Character {
		t.Helper()
		return a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 3, "subclass:open-hand"), abilities(10, 16, 12, 10, 14), nil, nil, nil)
	}
	for _, tc := range []struct {
		name   string
		toren  func(t *testing.T, a *armed) *charactersv1.Character
		attack string
		roll   func(*playv1.RollAttackRequest)
		want   playv1.ReactionKind
	}{
		{"Uncanny Dodge", rogue, sword, d20(19), playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE},
		{"Deflect Missiles", monk, shortBow, disadvantage(19), playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := damageFight(t, tc.toren)
			a.mustAttack(t, a.master, e, "Goblin", tc.attack, "Toren", tc.roll)
			a.mustAnswer(t, a.ana, e, a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS).GetId(), passAnswer)
			a.mustDamage(t, a.master, e, a.firstPending(t), typedDamage(4))
			var kinds []playv1.ReactionKind
			for _, w := range a.windows(t, a.master) {
				kinds = append(kinds, w.GetKind())
			}
			if len(kinds) != 2 || kinds[0] != playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS || kinds[1] != tc.want {
				t.Errorf("the windows = %v, want Cutting Words and then %v", kinds, tc.want)
			}
		})
	}
}

// TestPassingDeflectMissilesAnswersItAndSpendsNothing.
func TestPassingDeflectMissilesAnswersItAndSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, monkThree)
	hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
	res := a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	if res.GetResult().GetUsed() || byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
		t.Errorf("the result = %v, want a pass that spends nothing", res.GetResult())
	}
	if status, amount := a.pendingOf(t, hit.GetPendingDamage().GetId()); status != "rolled" || amount != 8 {
		t.Errorf("the pending damage = %s for %d, want rolled for 8", status, amount)
	}
}

// TestAFallNobodyIsNearEnoughToSlowLandsAtOnce: a wizard farther than 60 feet from the faller gets no
// window, so the damage waits only for the master (a player's character's).
func TestAFallNobodyIsNearEnoughToSlowLandsAtOnce(t *testing.T) {
	t.Parallel()
	r, _, pt := openPitRig(t)
	moved := false
	for _, sq := range [][2]int32{{22, 7}, {22, 8}, {23, 7}, {23, 8}, {22, 6}, {21, 8}} { // more than 60 feet from the pit, on the first free square
		if _, err := r.moveResult(t, r.master, "Pensantus", sq[0], sq[1]); err == nil {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatal("no free square far from the pit")
	}
	r.h.roller.queue(5)
	if _, err := r.fireByHand(t, pt, r.id(t, "Toren")); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	if w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL); w != nil {
		t.Errorf("the faraway wizard has a window: %v", w)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 5 {
		t.Errorf("Toren's fall = %s for %d, want rolled for 5", status, amount)
	}
}

// TestHellishRebukeOpensNoWindowForDamageTheTemporaryHitPointsTook: only hit points lost count.
func TestHellishRebukeOpensNoWindowForDamageTheTemporaryHitPointsTook(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, pactWarlock)
	if _, err := a.master.adjust(t, a.campaignID, a.pens.GetId(), func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsTemporary = proto.Int32(20) }); err != nil {
		t.Fatalf("AdjustCharacterVitals() error = %v", err)
	}
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	if _, err := a.settle(t, a.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE); w != nil {
		t.Errorf("the Reator has a Hellish Rebuke window for damage the temporary hit points took: %v", w)
	}
}

// TestTheShieldPromptTellsEachViewerOnlyWhatTheyMayRead: the master reads the attacker's id, its
// label and the attack's name; the target's player reads the label and the attack's name only while
// the attacker is in sight; nobody else reads the prompt (RN-20).
func TestTheShieldPromptTellsEachViewerOnlyWhatTheyMayRead(t *testing.T) {
	t.Parallel()
	a, e, _ := shieldedPensantus(t)
	want := func(who string, p *playv1.ReactionPrompt, id, label, attack string) {
		t.Helper()
		if p.GetAttackerId() != id || p.GetAttackerLabel() != label || p.GetAttackNamePt() != attack || p.GetSpellKey() != shieldSpell || p.GetSpellNamePt() == "" || p.GetTargetId() != a.id(t, "Pensantus") {
			t.Errorf("%s reads the prompt %v, want attacker %q / %q with %q", who, p, id, label, attack)
		}
	}
	capitao := a.id(t, "Capitão Goblin")
	if ps := a.get(t, a.master).GetReactionPrompts(); len(ps) != 1 {
		t.Fatalf("the master reads %d prompts, want 1", len(ps))
	} else {
		want("the master", ps[0], capitao, "Capitão Goblin", "Cimitarra")
	}
	if ps := a.get(t, a.ana).GetReactionPrompts(); len(ps) != 1 {
		t.Fatalf("Pensantus's player reads %d prompts, want 1", len(ps))
	} else {
		want("Pensantus's player", ps[0], "", "Capitão Goblin", "Cimitarra")
	}
	// Out of sight, the attacker is not named to the player (the master still reads it).
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: capitao, IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	if ps := a.get(t, a.ana).GetReactionPrompts(); len(ps) != 1 {
		t.Fatalf("Pensantus's player reads %d prompts, want 1", len(ps))
	} else {
		want("Pensantus's player, attacker hidden", ps[0], "", "", "")
	}
	if ps := a.get(t, a.master).GetReactionPrompts(); len(ps) != 1 {
		t.Fatalf("the master reads %d prompts, want 1", len(ps))
	} else {
		want("the master, attacker hidden", ps[0], capitao, "Capitão Goblin", "Cimitarra")
	}
	for who, u := range map[string]*user{"Toren's player": a.caio, "Brisa's player": a.bia} {
		if ps := a.get(t, u).GetReactionPrompts(); len(ps) != 0 {
			t.Errorf("%s reads the prompts %v, want none", who, ps)
		}
	}
}
