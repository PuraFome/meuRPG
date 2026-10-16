package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These tests freeze the answer to a Counterspell window (answerCounterspell, reactionRoll): a
// spell above the slot asks an ability check, the refusals of an answer and their order. They need
// the database (MEURPG_TEST_DATABASE_URL).

// upcastWindow has the Mago Sombrio cast Magic Missile with a slot of the 4th level at Toren and
// returns Pensantus's Counterspell window: her 3rd-level slot is below the spell, so it asks a check
// (DC 10 + 4, with her +4).
func upcastWindow(t *testing.T) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a, e := counterFight(t)
	held, err := a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(4), []*playv1.SpellTarget{darts(a, t, "Toren", 6)}, noCastRoll)
	if err != nil || held.GetCast() != nil {
		t.Fatalf("CastSpell() = %v, %v; want the cast held", held, err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if w == nil {
		t.Fatal("no Counterspell window for Pensantus")
	}
	return a, e, w
}

func typedAnswer(face int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		useAnswer(3)(r)
		r.Roll = &playv1.AnswerReactionRequest_Typed{Typed: face}
	}
}

// A spell above the slot: the master reads the check, the reactor answers it with a roll or fails to,
// and every refusal leaves the window open.
func TestCounterspellAboveTheSlotAsksAnAbilityCheck(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	mw := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if mw == nil || len(mw.GetTrigger().GetSlotEffects()) != 1 {
		t.Fatalf("the master's window = %v, want one slot effect", mw)
	}
	if se := mw.GetTrigger().GetSlotEffects()[0]; se.GetNoCheck() || se.GetCheckDc() != 14 || se.GetCheckBonus() != 4 {
		t.Errorf("slot effect = %v, want a check against DC 14 with +3", se)
	}
	// Refusals, in the order the checks run.
	_, err := a.answerReaction(t, a.ana, e, w.GetId(), useAnswer(3))
	wantBlockedBy(t, "no roll", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_NEEDS_ROLL)
	if err.Error() != "failed_precondition: this answer needs a roll" {
		t.Errorf("no roll message = %q", err)
	}
	_, err = a.answerReaction(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useAnswer(3)(r)
		r.Roll = &playv1.AnswerReactionRequest_RollInApp{RollInApp: false}
	})
	wantCode(t, "roll_in_app false", err, connect.CodeInvalidArgument)
	if err.Error() != "invalid_argument: roll_in_app must be true" {
		t.Errorf("roll_in_app false message = %q", err)
	}
	for _, face := range []int32{0, 21} {
		_, err = a.answerReaction(t, a.ana, e, w.GetId(), typedAnswer(face))
		wantCode(t, "typed out of range", err, connect.CodeInvalidArgument)
		if err.Error() != "invalid_argument: typed must be 1 to 20" {
			t.Errorf("typed %d message = %q", face, err)
		}
	}
	_, err = a.answerReaction(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Slot = playv1.ReactionChoice_REACTION_CHOICE_USE, slotOfLevel(4)
		r.Roll = &playv1.AnswerReactionRequest_Typed{Typed: 10}
	})
	wantCode(t, "a slot not offered", err, connect.CodeInvalidArgument)
	if err.Error() != "invalid_argument: slot is not one of the free slots the window offers" {
		t.Errorf("slot message = %q", err)
	}
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	_, err = a.answerReaction(t, a.ana, e, w.GetId(), typedAnswer(10))
	wantBlockedBy(t, "typed on app dice", err, blockedWrongDice)
	if err.Error() != "failed_precondition: this is not how the campaign has you roll your dice" {
		t.Errorf("wrong dice message = %q", err)
	}
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)

	// A 9 and the +4 are a 13 against 14: the spell goes through, the slot and the reaction are spent.
	res := a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(9))
	c := res.GetResult().GetCounterspell()
	if c.GetCountered() || c.GetCheckDc() != 14 || c.GetCheck().GetTotal() != 13 || c.GetCheck().GetModifier() != 4 || c.GetCheck().GetFaces()[0] != 9 || !c.GetCheck().GetPhysical() {
		t.Fatalf("result = %v, want the spell through on a 13 against 14 with a typed die", c)
	}
	if pens := byLabel(t, res.GetEncounter(), "Pensantus"); !pens.GetReactionUsed() {
		t.Error("Pensantus's reaction is not spent")
	}
}

// The same check passed, with the die rolled in the app: the spell is countered and the other
// windows of the cast have nothing left to counter.
func TestCounterspellCheckPassedInTheAppCountersTheSpell(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	a.h.roller.queue(12)
	res := a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useAnswer(3)(r)
		r.Roll = &playv1.AnswerReactionRequest_RollInApp{RollInApp: true}
	})
	c := res.GetResult().GetCounterspell()
	if !c.GetCountered() || c.GetCheck().GetTotal() != 16 || c.GetCheck().GetPhysical() || c.GetCheckDc() != 14 {
		t.Fatalf("result = %v, want the spell countered on a 16 rolled in the app", c)
	}
	if toren := byLabel(t, a.get(t, a.master), "Toren"); toren.GetHitPointsCurrent() != toren.GetHitPointsMax() {
		t.Error("Toren took damage from a countered spell")
	}
}

// What blocks a use: the reaction already spent and no free slot, checked in that order; passing still works.
func TestCounterspellAnswerRefusalsOfTheReaction(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	a.pensantusTurn(t, e)
	if _, err := a.castMissile(t, e); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if w == nil {
		t.Fatal("no Counterspell window")
	}
	mago := a.id(t, "Mago")
	exec := func(q string) {
		t.Helper()
		if _, err := a.h.pool.Exec(t.Context(), q, mago); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// No free slot of the 3rd level.
	exec(`UPDATE combatants SET slots_used = '{"1": 4, "3": 3, "4": 3, "5": 1}'::JSONB WHERE id = $1`)
	_, err := a.answerReaction(t, a.master, e, w.GetId(), useAnswer(3))
	wantBlockedBy(t, "no slot", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT)
	if err.Error() != "failed_precondition: there is no free spell slot for Contramágica" {
		t.Errorf("no slot message = %q", err)
	}
	// The reaction already used comes before the slots.
	exec(`UPDATE combatants SET reaction_used = true WHERE id = $1`)
	_, err = a.answerReaction(t, a.master, e, w.GetId(), useAnswer(3))
	wantBlockedBy(t, "reaction used", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_USED)
	if err.Error() != "failed_precondition: the reaction is already used" {
		t.Errorf("reaction used message = %q", err)
	}
	// Passing is not a use: it asks neither.
	res := a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
	if r := res.GetResult(); r.GetUsed() || r.GetCounterspell() != nil {
		t.Errorf("pass result = %v, want not used and no counterspell result", r)
	}
}
