package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The initiative of a character in SETUP is a Dexterity check (SRD 5.1), so it asks the halfling's Lucky about a 1 and
// the Bardic Inspiration die the character holds: the d20 is kept, the initiative stays unset, and the answer sets it
// with the die added.

// setupWithToren starts a combat with an NPC and leaves every player without an initiative.
func setupWithToren(t *testing.T, race string) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCastersOf(t, 3, race)
	a.h.roller.queue(1)
	res, err := a.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), Name: "Emboscada na estrada",
		Participants: []*playv1.Participant{{CharacterId: a.capitao.GetId()}},
	}))
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	return a, res.Msg.GetEncounter()
}

// submitInitiative calls SubmitInitiative as u for the combatant, with the physical d20 typed.
func (a *armed) submitInitiative(t *testing.T, u *user, e *playv1.Encounter, label string, face int32) (*playv1.SubmitInitiativeResponse, error) {
	t.Helper()
	res, err := u.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, label).GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: face},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestAnInitiativeWithABardicDieIsHeldAndTheDieAddsToIt(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
	res, err := a.submitInitiative(t, a.caio, e, "Toren", 7)
	if err != nil || res.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want the die's question", res, err)
	}
	if offer := res.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_INITIATIVE || offer.GetD20().GetFaces()[0] != 7 {
		t.Errorf("the offer = %v, want the initiative's 7", offer)
	}
	toren := byLabel(t, res.GetEncounter(), "Toren")
	if toren.Initiative != nil {
		t.Errorf("Toren's initiative = %d while the question waits; want it unset", toren.GetInitiative())
	}
	a.h.roller.queue(4)
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	set := byLabel(t, out.GetInitiative().GetEncounter(), "Toren")
	if want := 7 + set.GetInitiativeBonus() + 4; set.GetInitiative() != want || set.GetInitiativeFace() != 7 {
		t.Errorf("Toren's initiative = %d (face %d), want 7 + %d + the die 4 = %d", set.GetInitiative(), set.GetInitiativeFace(), set.GetInitiativeBonus(), want)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM character_inspiration WHERE character_id = $1`, a.toren.GetId()).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows of the die after using it = %d (%v), want none", n, err)
	}
}

func TestAnInitiativeWithTheDieKeptCountsTheBareD20(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
	res, err := a.submitInitiative(t, a.caio, e, "Toren", 7)
	if err != nil {
		t.Fatalf("SubmitInitiative() error = %v", err)
	}
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	set := byLabel(t, out.GetInitiative().GetEncounter(), "Toren")
	if want := 7 + set.GetInitiativeBonus(); set.GetInitiative() != want {
		t.Errorf("Toren's initiative = %d, want the bare %d", set.GetInitiative(), want)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM character_inspiration WHERE character_id = $1`, a.toren.GetId()).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows of the die after keeping it = %d (%v), want 1", n, err)
	}
}

func TestAHalflingsInitiativeAsksLuckyAboutA1(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "race:halfling")
	res, err := a.submitInitiative(t, a.caio, e, "Toren", 1)
	if err != nil || res.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want Lucky's question", res, err)
	}
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), luckyAnswer(14))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	set := byLabel(t, out.GetInitiative().GetEncounter(), "Toren")
	if want := 14 + set.GetInitiativeBonus(); set.GetInitiative() != want || set.GetInitiativeFace() != 14 {
		t.Errorf("Toren's initiative = %d (face %d), want the 1 rolled again to 14: %d", set.GetInitiative(), set.GetInitiativeFace(), want)
	}
}

func TestAnInitiativeWithoutADieOrLuckSettlesAtOnce(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	res, err := a.submitInitiative(t, a.caio, e, "Toren", 12)
	if err != nil || res.GetHeldRollOffer() != nil || byLabel(t, res.GetEncounter(), "Toren").Initiative == nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want the initiative set", res, err)
	}
}

func TestTheMastersInitiativeForAPlayerAsksAndWaitsForThePlayer(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
	res, err := a.submitInitiative(t, a.master, e, "Toren", 9)
	if err != nil || res.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want the question", res, err)
	}
	// The master answers only after the player's first 10 s (E4-10).
	if _, err := a.answerHeldRoll(t, a.master, res.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("the master's early answer error = %v, want failed_precondition", err)
	}
	if _, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); err != nil {
		t.Errorf("the player's answer error = %v", err)
	}
}

func TestAnNPCsInitiativeWaitsForNobody(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	res, err := a.submitInitiative(t, a.master, e, "Capitão Goblin", 5)
	if err != nil || res.GetHeldRollOffer() != nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want it settled", res, err)
	}
}

func TestTheQuestionOfAnInitiativeIsOnTheCombatantOfTheOwnerAndTheMasterOnly(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
	res, err := a.submitInitiative(t, a.caio, e, "Toren", 7)
	if err != nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("SubmitInitiative() = %v, %v; want the question", res, err)
	}
	for name, u := range map[string]*user{"the master": a.master, "the owner": a.caio} {
		offer := byLabel(t, a.get(t, u), "Toren").GetHeldRollOffer()
		if offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_INITIATIVE || offer.GetHoldId() != res.GetHeldRollOffer().GetHoldId() {
			t.Errorf("%s reads Toren's offer = %v, want the initiative's question", name, offer)
		}
	}
	if offer := byLabel(t, a.get(t, a.ana), "Toren").GetHeldRollOffer(); offer != nil {
		t.Errorf("another player reads Toren's offer = %v, want none", offer)
	}
	if _, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if offer := byLabel(t, a.get(t, a.master), "Toren").GetHeldRollOffer(); offer != nil {
		t.Errorf("the master reads Toren's offer = %v after the answer, want none", offer)
	}
}
