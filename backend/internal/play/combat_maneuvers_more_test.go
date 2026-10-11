package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The table maneuvers that start a grapple after a melee hit and that reduce a melee hit's
// damage as a reaction (superiority_die, applies grapple and reduce_melee_damage). Names are
// invented. These tests need the database (MEURPG_TEST_DATABASE_URL).

func (m *maneuverTable) grapple(t *testing.T, e *playv1.Encounter, target string, set func(*playv1.StartContestRequest)) (*playv1.StartContestResponse, error) {
	t.Helper()
	req := &playv1.StartContestRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), InitiatorId: m.id(t, "Pensantus"), TargetId: m.id(t, target),
		Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE, Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS,
		Roll: &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_RollInApp{RollInApp: true}}, ManeuverKey: m.grip,
	}
	if set != nil {
		set(req)
	}
	res, err := m.ana.contests.StartContest(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (m *maneuverTable) bonusUsed(t *testing.T) bool {
	t.Helper()
	return byLabel(t, m.get(t, m.ana), "Pensantus").GetBonusActionUsed()
}

// After a melee hit the maneuver opens the grapple with its die added to the Athletics check,
// and spends the bonus action and a use, not an attack.
func TestManeuverGrappleAfterAMeleeHitAddsItsDieAndSpendsTheBonusActionAndAUse(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	m.hit(t, e)

	m.h.roller.queue(12, 5) // the d20, then the maneuver die
	res, err := m.grapple(t, e, "Capitão Goblin", nil)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	roll := res.GetContest().GetInitiatorRoll()
	if roll.GetManeuverDieSides() != 8 || roll.GetManeuverDieFace() != 5 || roll.GetManeuverNamePt() != "Golpe de Pegada" {
		t.Fatalf("the initiator's roll = %v, want the d8 maneuver die (5) named in it", roll)
	}
	if roll.GetTotal() != roll.GetFaces()[0]+roll.GetModifier()+5 {
		t.Errorf("total = %d, want the d20 %d + modifier %d + the die 5", roll.GetTotal(), roll.GetFaces()[0], roll.GetModifier())
	}
	if !m.bonusUsed(t) {
		t.Error("the bonus action is not spent")
	}
	if got := diceLeft(t, m); got != 3 {
		t.Errorf("dice left = %d, want 3", got)
	}
}

// Without a melee hit this turn, without the bonus action or without a use it is refused.
func TestManeuverGrappleIsRefusedWithoutAHitABonusActionOrAUse(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "1")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	if _, err := m.grapple(t, e, "Capitão Goblin", nil); err == nil {
		t.Fatal("a maneuver grapple with no melee hit this turn is accepted")
	}
	if m.bonusUsed(t) {
		t.Fatal("the refusal spent the bonus action")
	}
	m.hit(t, e)
	if _, err := m.grapple(t, e, "Capitão Goblin", func(r *playv1.StartContestRequest) { r.Roll = nil }); err == nil {
		t.Error("a grapple with no roll is accepted")
	}
	m.h.roller.queue(12, 5)
	if _, err := m.grapple(t, e, "Capitão Goblin", nil); err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	if got := diceLeft(t, m); got != 0 {
		t.Fatalf("dice left = %d, want 0", got)
	}
}

// Physical dice: the face of the maneuver die is typed and must fit its sides.
func TestManeuverGrappleWithPhysicalDiceNeedsTheDieFace(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	m.hit(t, e)
	typed := func(face int32) func(*playv1.StartContestRequest) {
		return func(r *playv1.StartContestRequest) {
			r.Roll = &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_D20Faces{D20Faces: &playv1.D20Faces{Faces: []int32{12}}}}
			r.ManeuverFace = face
		}
	}
	if _, err := m.grapple(t, e, "Capitão Goblin", typed(0)); err == nil {
		t.Error("physical dice with no maneuver face are accepted")
	}
	if _, err := m.grapple(t, e, "Capitão Goblin", typed(9)); err == nil {
		t.Error("a face above the die's sides is accepted")
	}
	res, err := m.grapple(t, e, "Capitão Goblin", typed(6))
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	if got := res.GetContest().GetInitiatorRoll().GetManeuverDieFace(); got != 6 {
		t.Errorf("the die face = %d, want the 6 typed", got)
	}
}

// A melee hit on the character opens a window with each reduction maneuver; using it takes
// die + Dexterity modifier off the damage and spends the reaction and a use. A ranged hit opens none.
func TestManeuverReduceIsOfferedOnAMeleeHitAndReducesTheDamage(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.castersFight(t, 1)
	e = m.passTo(t, e, "Capitão Goblin")
	m.h.roller.queue(4) // the goblin's damage die
	hit := m.mustAttack(t, m.master, e, "Capitão Goblin", sword, "Pensantus", d20(19))
	if hit.GetPendingDamage() == nil {
		t.Fatalf("the Capitão's attack = %v, want a hit", hit.GetRoll())
	}
	m.mustDamage(t, m.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	w := m.windowOf(t, m.ana, playv1.ReactionKind_REACTION_KIND_MANEUVER_REDUCE)
	prompt := w.GetManeuverReduce()
	if prompt == nil || len(prompt.GetOptions()) != 1 || prompt.GetOptions()[0].GetKey() != m.guard || prompt.GetOptions()[0].GetDieSides() != 8 {
		t.Fatalf("the prompt = %v, want the one reduction maneuver as a d8", w.GetPrompt())
	}
	m.h.roller.queue(6)
	res := m.mustAnswer(t, m.ana, m.get(t, m.ana), w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		r.ManeuverKey = m.guard
		r.Roll = &playv1.AnswerReactionRequest_RollInApp{RollInApp: true}
	})
	got := res.GetResult().GetManeuverReduce()
	if got == nil || got.GetReduction().GetFaces()[0] != 6 || got.GetDamageAfter() != max(got.GetDamageBefore()-(6+got.GetReduction().GetModifier()), 0) {
		t.Fatalf("the result = %v, want the d8 (6) plus the Dexterity modifier off the damage", res.GetResult())
	}
	if !byLabel(t, m.get(t, m.ana), "Pensantus").GetReactionUsed() {
		t.Error("the reaction is not spent")
	}
	if got := diceLeft(t, m); got != 3 {
		t.Errorf("dice left = %d, want 3", got)
	}
}

// An answer that names no maneuver of the prompt is refused and spends nothing.
func TestManeuverReduceRefusesAnUnknownManeuver(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Capitão Goblin")
	m.h.roller.queue(4)
	hit := m.mustAttack(t, m.master, e, "Capitão Goblin", sword, "Pensantus", d20(19))
	m.mustDamage(t, m.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	w := m.windowOf(t, m.ana, playv1.ReactionKind_REACTION_KIND_MANEUVER_REDUCE)
	if _, err := m.answerReaction(t, m.ana, m.get(t, m.ana), w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		r.ManeuverKey = "feature:inventado@mesa"
		r.Roll = &playv1.AnswerReactionRequest_RollInApp{RollInApp: true}
	}); err == nil {
		t.Fatal("an unknown maneuver is accepted")
	}
	if got := diceLeft(t, m); got != 4 {
		t.Errorf("dice left = %d, want 4", got)
	}
}
