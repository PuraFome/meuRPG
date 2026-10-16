package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of the calls that follow a contest (RespondContest, ResolveShove,
// CloseContest, ReleaseGrapple) and of the holds that end by themselves (pruneHolds): every
// refusal with its exact code, reason and message, who may call, what each call writes and
// what each viewer reads afterwards. The tests that already exist for contests
// (combat_contests_test.go, combat_start_contest_test.go) follow the rules; these freeze the
// answers of each branch so a refactor cannot change one.

const (
	answerSkillAthletics = playv1.ContestSkill_CONTEST_SKILL_ATHLETICS
	answerSkillAcrobats  = playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
)

// answerWith calls RespondContest as u with the request the test edits: the default is the
// Athletics answer rolled in the app.
func (c *cx) answerWith(t *testing.T, u *user, contestID string, edit func(*playv1.RespondContestRequest)) (*playv1.RespondContestResponse, error) {
	t.Helper()
	req := &playv1.RespondContestRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: contestID, Skill: answerSkillAthletics, Roll: rollIn(),
	}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.RespondContest(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// closeWith calls CloseContest as u.
func (c *cx) closeWith(t *testing.T, u *user, edit func(*playv1.CloseContestRequest)) (*playv1.CloseContestResponse, error) {
	t.Helper()
	req := &playv1.CloseContestRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.CloseContest(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// releaseWith calls ReleaseGrapple as u.
func (c *cx) releaseWith(t *testing.T, u *user, edit func(*playv1.ReleaseGrappleRequest)) (*playv1.ReleaseGrappleResponse, error) {
	t.Helper()
	req := &playv1.ReleaseGrappleRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.ReleaseGrapple(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// shoveWith calls ResolveShove as u with the request the test edits.
func (c *cx) shoveWith(t *testing.T, u *user, contestID string, outcome playv1.ShoveOutcome, edit func(*playv1.ResolveShoveRequest)) (*playv1.ResolveShoveResponse, error) {
	t.Helper()
	req := &playv1.ResolveShoveRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: contestID, Outcome: outcome}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.ResolveShove(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// answerLines are the contest lines of the log as u reads them, "line actor>target", in order.
func (c *cx) answerLines(t *testing.T, u *user) []string {
	t.Helper()
	var out []string
	for _, e := range contestLogEntries(c.log(t, u, c.e)) {
		if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONTEST {
			out = append(out, e.GetContest().GetLine().String()+" "+e.GetActorLabel()+">"+e.GetTargetLabel())
		}
	}
	return out
}

// answerContestOf is the one contest the master reads, by id.
func (c *cx) answerContestOf(t *testing.T, contestID string) *playv1.ContestView {
	t.Helper()
	for _, v := range c.state(t, c.master).GetContests() {
		if v.GetId() == contestID {
			return v
		}
	}
	t.Fatalf("the master reads no contest %s", contestID)
	return nil
}

// wantAnswerNotAwaiting checks the refusal of a contest that does not wait for this.
func wantAnswerNotAwaiting(t *testing.T, call string, err error) {
	t.Helper()
	wantRefusal(t, call, err, connect.CodeFailedPrecondition, "the contest does not wait for this")
	wantContestBlocked(t, call, err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AWAITING)
}

// grappleOfToren opens Toren's grapple on the Hobgoblin (d20 10) and gives the contest.
func (c *cx) grappleOfToren(t *testing.T) *playv1.ContestView {
	t.Helper()
	res, err := c.grapple(t, c.caio, "Toren", c.hob, 10)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	return res.GetContest()
}

// TestRespondContestRefusesBadRequestsBeforeTheTransaction freezes every refusal that comes
// from reading the request alone, and that none of them touched the contest.
func TestRespondContestRefusesBadRequestsBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	id := c.grappleOfToren(t).GetId()
	stranger := a.h.newUser("Intruso")
	tests := []struct {
		name string
		as   *user
		edit func(r *playv1.RespondContestRequest)
		code connect.Code
		msg  string
	}{
		{"a stranger to the campaign", stranger, nil, connect.CodeNotFound, "campaign not found"},
		{"an idempotency key that is not a UUID", a.master, func(r *playv1.RespondContestRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", a.master, func(r *playv1.RespondContestRequest) { r.EncounterId = "x" }, connect.CodeNotFound, "encounter not found"},
		{"a contest that is not a UUID", a.master, func(r *playv1.RespondContestRequest) { r.ContestId = "x" }, connect.CodeNotFound, "contest not found"},
		{"a contest that is not a UUID comes before the roll", a.master, func(r *playv1.RespondContestRequest) { r.ContestId, r.Roll = "x", nil }, connect.CodeNotFound, "contest not found"},
		{"no way to roll", a.master, func(r *playv1.RespondContestRequest) { r.Roll = nil }, connect.CodeInvalidArgument, "set roll_in_app or d20_faces"},
		{"roll_in_app false", a.master, func(r *playv1.RespondContestRequest) {
			r.Roll = &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_RollInApp{RollInApp: false}}
		}, connect.CodeInvalidArgument, "roll_in_app must be true"},
		{"no faces", a.master, func(r *playv1.RespondContestRequest) { r.Roll = faces() }, connect.CodeInvalidArgument, "d20_faces must have one or two faces"},
		{"three faces", a.master, func(r *playv1.RespondContestRequest) { r.Roll = faces(1, 2, 3) }, connect.CodeInvalidArgument, "d20_faces must have one or two faces"},
		{"a face of 21", a.master, func(r *playv1.RespondContestRequest) { r.Roll = faces(21) }, connect.CodeInvalidArgument, "each d20 face must be 1 to 20"},
		{"the master leaving the roll to himself", a.master, func(r *playv1.RespondContestRequest) { r.DeferToMaster = true }, connect.CodeInvalidArgument, "the master rolls himself: leaveto_master is the player's"},
	}
	for _, tc := range tests {
		_, err := c.answerWith(t, tc.as, id, tc.edit)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	if v := c.answerContestOf(t, id); v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER || v.GetDefenderRoll() != nil || v.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_UNSPECIFIED {
		t.Errorf("a refused answer changed the contest: %v", v)
	}
}

// TestRespondContestRefusesInsideTheTransaction freezes who may answer and what the contest
// must be: the refusals read from the combat, none of which writes.
func TestRespondContestRefusesInsideTheTransaction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	id := c.grappleOfToren(t).GetId()
	tests := []struct {
		name string
		as   *user
		id   string
		code connect.Code
		msg  string
	}{
		{"a contest that is not in the combat", a.master, newKey(), connect.CodeNotFound, "contest not found"},
		{"a player outside the contest", a.ana, id, connect.CodeNotFound, "contest not found"},
		{"the other player outside the contest", a.bia, id, connect.CodeNotFound, "contest not found"},
		{"the initiator's player answering for the NPC", a.caio, id, connect.CodePermissionDenied, "only the one that answers or the master may do this"},
	}
	for _, tc := range tests {
		_, err := c.answerWith(t, tc.as, tc.id, nil)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	if v := c.answerContestOf(t, id); v.GetDefenderRoll() != nil || v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER {
		t.Errorf("a refused answer changed the contest: %v", v)
	}

	// The master answers; a second answer, by anyone who may, finds nothing waiting.
	c.mustRespond(t, a.master, id, answerSkillAthletics, 5)
	_, err := c.answerWith(t, a.master, id, nil)
	wantAnswerNotAwaiting(t, "an answer to a contest that is resolved", err)
	_, err = c.answerWith(t, a.master, id, func(r *playv1.RespondContestRequest) { r.DeferToMaster = false; r.Roll = faces(3) })
	wantAnswerNotAwaiting(t, "typed dice to a contest that is resolved", err)
}

// TestRespondContestWhenTheCombatIsNotRunning freezes the answer to a contest of a combat that
// ended: nothing is answered and the contest stays as it was.
func TestRespondContestWhenTheCombatIsNotRunning(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	id := c.grappleOfToren(t).GetId()
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	_, err := c.answerWith(t, a.master, id, nil)
	wantRefusal(t, "a combat that ended", err, connect.CodeFailedPrecondition, "the combat has ended")
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
}

// TestRespondContestOfAPlayerDefender freezes the answers of a player who defends: the dice
// mode, the deferral to the master, what is left to the master afterwards, and who reads what.
func TestRespondContestOfAPlayerDefender(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.advance(t, c.hob)
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeGrappleP
	})
	id := started.GetContest().GetId()

	// Someone who is neither side does not know the contest exists; the NPC's side is the master's.
	_, err := c.answerWith(t, a.caio, id, nil)
	wantRefusal(t, "another player", err, connect.CodeNotFound, "contest not found")

	// A player is held to the dice of the campaign; the master is not.
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	_, err = c.answerWith(t, a.bia, id, nil)
	wantRefusal(t, "the app's roll when everybody rolls real dice", err, connect.CodeFailedPrecondition, "this is not how the campaign has you roll your dice")
	wantEncounterBlocked(t, err, blockedWrongDice)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)

	// A face typed for a roll that takes the pair is told to read the options again.
	_, err = c.answerWith(t, a.bia, id, func(r *playv1.RespondContestRequest) { r.Skill, r.Roll = answerSkillAcrobats, faces(10, 11) })
	if err == nil {
		t.Fatal("a pair of typed faces for a plain roll was accepted in app mode")
	}

	// She leaves the roll to the master with Acrobatics as her choice: the skill is kept, she cannot answer again.
	left, err := c.answerWith(t, a.bia, id, func(r *playv1.RespondContestRequest) {
		r.Skill, r.Roll, r.DeferToMaster = answerSkillAcrobats, nil, true
	})
	if err != nil {
		t.Fatalf("RespondContest(leave to the master) error = %v", err)
	}
	if v := left.GetContest(); !v.GetDeferred() || v.GetDeferredSkill() != answerSkillAcrobats || v.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER ||
		v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER || v.GetYouAnswer() || v.GetDefenderRoll() != nil {
		t.Errorf("Brisa reads %v after leaving the roll, want deferred to the master with Acrobatics and nothing to answer", v)
	}
	_, err = c.answerWith(t, a.bia, id, nil)
	wantAnswerNotAwaiting(t, "the player answering after leaving it to the master", err)
	_, err = c.answerWith(t, a.bia, id, func(r *playv1.RespondContestRequest) { r.DeferToMaster, r.Roll = true, nil })
	wantAnswerNotAwaiting(t, "the player leaving it twice", err)
	if v := c.answerContestOf(t, id); !v.GetDeferred() || !v.GetYouAnswer() || len(v.GetAnswerOptions()) != 1 || v.GetAnswerOptions()[0].GetSkill() != answerSkillAcrobats {
		t.Errorf("the master reads %v, want to answer with the one skill she chose", v)
	}

	// The master answers for her: the skill she chose rolls (Acrobatics +3), whatever he asks for. 8 + 3 = 11 against 18.
	done, err := c.answerWith(t, a.master, id, func(r *playv1.RespondContestRequest) { c.h.roller.queue(8); r.Skill = answerSkillAthletics })
	if err != nil {
		t.Fatalf("RespondContest(master) error = %v", err)
	}
	roll := done.GetContest().GetDefenderRoll()
	if roll.GetSkill() != answerSkillAcrobats || roll.GetTotal() != 11 || !roll.GetRolledByMaster() || done.GetContest().GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR {
		t.Errorf("answer = %v, want Acrobatics 11 and the Hobgoblin the winner", done.GetContest())
	}
	if !c.hasCondition(t, "Brisa", condGrappled) {
		t.Error("Brisa is not Grappled after losing")
	}
	want := []string{"CONTEST_LOG_LINE_GRAPPLED " + c.hob + ">Brisa"} // newest first
	for who, u := range map[string]*user{"master": a.master, "Brisa": a.bia, "Toren": a.caio} {
		if got := c.answerLines(t, u); !slices.Equal(got, want) {
			t.Errorf("%s reads the log %v, want %v", who, got, want)
		}
	}
	if v := c.state(t, a.bia).GetContests()[0]; v.GetInitiatorRoll() != nil || v.GetEscapeDc() != 0 || v.GetDefenderRoll().GetTotal() != 11 || v.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR {
		t.Errorf("Brisa reads %v, want her own 11 and who won, not the Hobgoblin's roll", v)
	}
}

// TestRespondContestDefaultsTheSkillToAthletics freezes that a request with no skill is the
// Athletics answer, and that the master's typed d20 is read as the roll of the defender.
func TestRespondContestDefaultsTheSkillToAthletics(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	id := c.grappleOfToren(t).GetId() // Toren 10 + 3 = 13
	// Typed 14 + the Hobgoblin's Athletics +1 = 15: it wins.
	res, err := c.answerWith(t, a.master, id, func(r *playv1.RespondContestRequest) {
		r.Skill, r.Roll = playv1.ContestSkill_CONTEST_SKILL_UNSPECIFIED, faces(14)
	})
	if err != nil {
		t.Fatalf("RespondContest() error = %v", err)
	}
	roll := res.GetContest().GetDefenderRoll()
	if roll.GetSkill() != answerSkillAthletics || roll.GetTotal() != 15 || !roll.GetPhysical() || res.GetContest().GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER {
		t.Errorf("answer = %v, want a typed Athletics 15 won by the defender", res.GetContest())
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("the defender's win grappled it")
	}
	if got := c.answerLines(t, a.caio); !slices.Equal(got, []string{"CONTEST_LOG_LINE_GRAPPLE_FAILED Toren>" + c.hob}) {
		t.Errorf("the log of the failed grapple = %v", got)
	}
}

// TestRespondContestDoesNotAnswerAFixedDCContest: a grapple that came from an attack is
// settled when it starts, so there is nothing to answer.
func TestRespondContestDoesNotAnswerAFixedDCContest(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.grappledByAttack(t, "Brisa", 16)
	grapples := c.state(t, a.master).GetContests()
	if len(grapples) != 1 {
		t.Fatalf("the master reads %d contests, want the one by attack", len(grapples))
	}
	_, err := c.answerWith(t, a.master, grapples[0].GetId(), nil)
	wantAnswerNotAwaiting(t, "a fixed DC contest", err)

	// An escape against that DC is no contest to answer either: it is settled at once.
	c.advance(t, "Brisa")
	c.h.roller.queue(3)
	esc := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose = c.id(t, "Brisa"), purposeEscapeP
	})
	_, err = c.answerWith(t, a.master, esc.GetContest().GetId(), nil)
	wantAnswerNotAwaiting(t, "a fixed DC escape", err)
}

// TestRespondContestOfAnEscapeAndAShove freezes the settle of the other purposes: an escape
// that fails keeps the grapple (the log says so), a shove that fails changes nothing and a
// shove that wins waits for its winner.
func TestRespondContestOfAnEscapeAndAShove(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	c.advance(t, c.hob)
	c.advance(t, "Toren")

	// The Hobgoblin is grappled; it escapes with 4 + 1 = 5 against Toren's Athletics 15 + 3: fails.
	c.advance(t, c.hob)
	c.h.roller.queue(4)
	esc := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose = c.id(t, c.hob), purposeEscapeP
	}).GetContest()
	c.mustRespond(t, a.caio, esc.GetId(), answerSkillAcrobats, 15)
	if !c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("a failed escape freed the Hobgoblin")
	}
	if got := c.answerLines(t, a.caio); got[0] != "CONTEST_LOG_LINE_ESCAPE_FAILED "+c.hob+">Toren" {
		t.Errorf("the log of the failed escape = %v", got)
	}

	// A shove that loses changes nothing.
	c.advance(t, "Toren")
	c.h.roller.queue(2)
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	lost := c.mustRespond(t, a.master, shove.GetId(), answerSkillAthletics, 19).GetContest()
	if lost.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_RESOLVED || lost.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER || lost.GetYouChoose() {
		t.Errorf("a lost shove = %v, want it resolved for the defender", lost)
	}
	if got := c.answerLines(t, a.master); got[0] != "CONTEST_LOG_LINE_SHOVE_FAILED Toren>"+c.hob {
		t.Errorf("the log of the lost shove = %v", got)
	}
}

// TestResolveShoveRefusals freezes every refusal of ResolveShove: the request, the sides, the
// contest and the outcome.
func TestResolveShoveRefusals(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.h.roller.queue(18)
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	id := shove.GetId()
	prone, push, stays := playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE, playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH, playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS
	stranger := a.h.newUser("Intruso")
	tests := []struct {
		name    string
		as      *user
		id      string
		outcome playv1.ShoveOutcome
		edit    func(r *playv1.ResolveShoveRequest)
		code    connect.Code
		msg     string
	}{
		{"a stranger to the campaign", stranger, id, prone, nil, connect.CodeNotFound, "campaign not found"},
		{"an idempotency key that is not a UUID", a.master, id, prone, func(r *playv1.ResolveShoveRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", a.master, id, prone, func(r *playv1.ResolveShoveRequest) { r.EncounterId = "x" }, connect.CodeNotFound, "encounter not found"},
		{"a contest that is not a UUID", a.master, "x", prone, nil, connect.CodeNotFound, "contest not found"},
		{"no outcome", a.master, id, playv1.ShoveOutcome_SHOVE_OUTCOME_UNSPECIFIED, nil, connect.CodeInvalidArgument, "outcome must be PRONE, PUSH or STAYS"},
		{"no outcome comes after the contest id", a.master, "x", playv1.ShoveOutcome_SHOVE_OUTCOME_UNSPECIFIED, nil, connect.CodeNotFound, "contest not found"},
		{"a contest that is not in the combat", a.master, newKey(), prone, nil, connect.CodeNotFound, "contest not found"},
		{"a shove that still waits for the defender", a.master, id, prone, nil, connect.CodeFailedPrecondition, "the contest does not wait for this"},
		{"a player outside the shove", a.ana, id, prone, nil, connect.CodeNotFound, "contest not found"},
	}
	for _, tc := range tests {
		_, err := c.shoveWith(t, tc.as, tc.id, tc.outcome, tc.edit)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	_, err := c.shoveWith(t, a.master, id, prone, nil)
	wantContestBlocked(t, "a shove that still waits for the defender", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AWAITING)

	// Won: the choice is Toren's or the master's, never the defender's, and STAYS is for a blocked push.
	c.mustRespond(t, a.master, id, answerSkillAthletics, 2)
	_, err = c.shoveWith(t, a.bia, id, prone, nil)
	wantRefusal(t, "a player outside the shove, after the answer", err, connect.CodeNotFound, "contest not found")
	_, err = c.shoveWith(t, a.caio, id, stays, nil)
	wantRefusal(t, "STAYS on a push that is free", err, connect.CodeInvalidArgument, "outcome STAYS is for a push that is blocked")
	if v := c.answerContestOf(t, id); v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_OUTCOME || !v.GetYouChoose() {
		t.Fatalf("a refused choice changed the shove: %v", v)
	}
	if c.hasCondition(t, c.hob, condProne) || c.square(t, c.hob) != [2]int32{4, 3} {
		t.Error("a refused choice moved or knocked down the Hobgoblin")
	}

	// The master chooses for the NPC's side too: here he resolves Toren's shove.
	res, err := c.shoveWith(t, a.master, id, push, nil)
	if err != nil {
		t.Fatalf("ResolveShove(master) error = %v", err)
	}
	if res.GetContest().GetShoveOutcome() != push || res.GetContest().GetStatus() != playv1.ContestStatus_CONTEST_STATUS_RESOLVED {
		t.Errorf("shove = %v, want it resolved with the push", res.GetContest())
	}
	if c.square(t, c.hob) != [2]int32{5, 3} {
		t.Errorf("the Hobgoblin stands on %v, want (5,3)", c.square(t, c.hob))
	}
	_, err = c.shoveWith(t, a.master, id, prone, nil)
	wantAnswerNotAwaiting(t, "a shove already resolved", err)

	// A combat that ended.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	_, err = c.shoveWith(t, a.master, id, prone, nil)
	wantRefusal(t, "a combat that ended", err, connect.CodeFailedPrecondition, "the combat has ended")
}

// TestResolveShoveByAPlayerThatDefendsIsRefused: the player who lost the contest does not
// choose what the winner does, but reads the contest.
func TestResolveShoveByAPlayerThatDefendsIsRefused(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.advance(t, c.hob)
	c.h.roller.queue(18)
	shove := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeShoveP
	}).GetContest()
	won := c.mustRespond(t, a.bia, shove.GetId(), answerSkillAcrobats, 1).GetContest()
	if won.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_OUTCOME || won.GetYouChoose() || won.GetShoveChoice() != nil || won.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER {
		t.Fatalf("Brisa reads %v, want a won shove that waits for the master and no choice for her", won)
	}
	_, err := c.shoveWith(t, a.bia, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE, nil)
	wantRefusal(t, "the defender choosing", err, connect.CodePermissionDenied, "only the one that won the shove or the master may do this")
	_, err = c.shoveWith(t, a.caio, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE, nil)
	wantRefusal(t, "a player outside the shove", err, connect.CodeNotFound, "contest not found")
	if c.hasCondition(t, "Brisa", condProne) {
		t.Fatal("a refused choice knocked Brisa down")
	}

	// The master chooses for his NPC; the log says it to everyone.
	c.mustShove(t, a.master, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	if !c.hasCondition(t, "Brisa", condProne) {
		t.Error("Brisa is not Derrubada")
	}
	for _, u := range []*user{a.master, a.bia, a.caio} {
		if got := c.answerLines(t, u); !slices.Equal(got, []string{"CONTEST_LOG_LINE_SHOVE_PRONE " + c.hob + ">Brisa"}) {
			t.Errorf("the log of the shove = %v", got)
		}
	}
}

// TestResolveShovePushOffTheMapIsBlockedByAWall freezes the push that has no square to go to:
// the choice reads the wall, PUSH is refused as PUSH_BLOCKED and STAYS is accepted, and
// nobody moves.
func TestResolveShovePushOffTheMapIsBlockedByAWall(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	// The Hobgoblin at the east edge of the map: the square behind it is off the grid.
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Toren": {18, 3}, "Hobgoblin": {19, 3}}})
	c.h.roller.queue(18)
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	view := c.mustRespond(t, a.master, shove.GetId(), answerSkillAthletics, 2).GetContest()
	if choice := view.GetShoveChoice(); !choice.GetProneAvailable() || choice.GetPushAvailable() || choice.GetPushBlocked() != playv1.ShoveBlockedReason_SHOVE_BLOCKED_REASON_WALL {
		t.Fatalf("shove choice at the map's edge = %v, want prone only and the push blocked by the wall", choice)
	}
	_, err := c.shoveWith(t, a.caio, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH, nil)
	wantRefusal(t, "pushing off the map", err, connect.CodeFailedPrecondition, "the square behind the target is blocked")
	wantContestBlocked(t, "ResolveShove", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_PUSH_BLOCKED)
	res, err := c.shoveWith(t, a.caio, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS, nil)
	if err != nil {
		t.Fatalf("ResolveShove(STAYS) error = %v", err)
	}
	if got := res.GetContest().GetShoveOutcome(); got != playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS {
		t.Errorf("outcome = %v, want STAYS", got)
	}
	if c.square(t, c.hob) != [2]int32{19, 3} || c.hasCondition(t, c.hob, condProne) {
		t.Errorf("a push that stays moved or knocked down the Hobgoblin: %v", c.square(t, c.hob))
	}
}

// TestCloseContestRefusals freezes who may close a contest and which ones.
func TestCloseContestRefusals(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	id := c.grappleOfToren(t).GetId()
	stranger := a.h.newUser("Intruso")
	tests := []struct {
		name string
		as   *user
		edit func(r *playv1.CloseContestRequest)
		code connect.Code
		msg  string
	}{
		{"a stranger to the campaign", stranger, func(r *playv1.CloseContestRequest) { r.ContestId = id }, connect.CodeNotFound, "campaign not found"},
		{"a player", a.caio, func(r *playv1.CloseContestRequest) { r.ContestId = id }, connect.CodePermissionDenied, "only the campaign's master can do this"},
		{"an idempotency key that is not a UUID", a.master, func(r *playv1.CloseContestRequest) { r.ContestId, r.IdempotencyKey = id, "x" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", a.master, func(r *playv1.CloseContestRequest) { r.ContestId, r.EncounterId = id, "x" }, connect.CodeNotFound, "encounter not found"},
		{"a contest that is not a UUID", a.master, func(r *playv1.CloseContestRequest) { r.ContestId = "x" }, connect.CodeNotFound, "contest not found"},
		{"a contest that is not in the combat", a.master, func(r *playv1.CloseContestRequest) { r.ContestId = newKey() }, connect.CodeNotFound, "contest not found"},
	}
	for _, tc := range tests {
		_, err := c.closeWith(t, tc.as, tc.edit)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	if v := c.answerContestOf(t, id); v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER {
		t.Errorf("a refused close changed the contest: %v", v)
	}

	// A contest that is settled cannot be closed.
	c.mustRespond(t, a.master, id, answerSkillAthletics, 20)
	_, err := c.closeWith(t, a.master, func(r *playv1.CloseContestRequest) { r.ContestId = id })
	wantAnswerNotAwaiting(t, "a contest already resolved", err)

	// A combat that ended.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	_, err = c.closeWith(t, a.master, func(r *playv1.CloseContestRequest) { r.ContestId = id })
	wantRefusal(t, "a combat that ended", err, connect.CodeFailedPrecondition, "the combat has ended")
}

// TestCloseContestClosesWhatWaits freezes what closing writes: a contest that waits for its
// defender and a shove that waits for its winner both end CLOSED, the master reads the line,
// a player reads no line and no contest wait, and the initiator is free to start another.
func TestCloseContestClosesWhatWaits(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	id := c.grappleOfToren(t).GetId()
	closed, err := c.closeWith(t, a.master, func(r *playv1.CloseContestRequest) { r.ContestId = id })
	if err != nil {
		t.Fatalf("CloseContest() error = %v", err)
	}
	if closed.GetEncounter() == nil || len(closed.GetEncounter().GetReactionWindows()) != 0 {
		t.Errorf("the encounter after closing = %v, want no window", closed.GetEncounter().GetReactionWindows())
	}
	v := c.answerContestOf(t, id)
	if v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_CLOSED || v.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_NONE || v.GetYouAnswer() || v.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_UNSPECIFIED {
		t.Errorf("closed contest = %v", v)
	}
	if got := c.answerLines(t, a.master); !slices.Equal(got, []string{"CONTEST_LOG_LINE_CLOSED Toren>" + c.hob}) {
		t.Errorf("the master reads the log %v", got)
	}
	for who, u := range map[string]*user{"Toren": a.caio, "Brisa": a.bia} {
		if got := c.answerLines(t, u); len(got) != 0 {
			t.Errorf("%s reads the closing in the log: %v", who, got)
		}
	}
	if got := c.state(t, a.caio).GetContests(); len(got) != 1 || got[0].GetStatus() != playv1.ContestStatus_CONTEST_STATUS_CLOSED {
		t.Errorf("Toren reads %v, want the contest closed", got)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("closing grappled")
	}

	// A shove that waits for its winner: the choice is closed, the outcome (none yet) kept.
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	c.h.roller.queue(18)
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	c.mustRespond(t, a.master, shove.GetId(), answerSkillAthletics, 2)
	if _, err := c.closeWith(t, a.master, func(r *playv1.CloseContestRequest) { r.ContestId = shove.GetId() }); err != nil {
		t.Fatalf("CloseContest(shove) error = %v", err)
	}
	if v := c.answerContestOf(t, shove.GetId()); v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_CLOSED || v.GetYouChoose() || v.GetShoveChoice() != nil {
		t.Errorf("closed shove = %v", v)
	}
	_, err = c.shoveWith(t, a.caio, shove.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE, nil)
	wantAnswerNotAwaiting(t, "choosing on a closed shove", err)
	_, err = c.answerWith(t, a.master, shove.GetId(), nil)
	wantAnswerNotAwaiting(t, "answering a closed shove", err)
}

// TestReleaseGrappleRefusals freezes who may let go and of what.
func TestReleaseGrappleRefusals(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	hob := c.id(t, c.hob)
	stranger := a.h.newUser("Intruso")
	of := func(label string) func(r *playv1.ReleaseGrappleRequest) {
		return func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = c.id(t, label) }
	}
	tests := []struct {
		name string
		as   *user
		edit func(r *playv1.ReleaseGrappleRequest)
		code connect.Code
		msg  string
	}{
		{"a stranger to the campaign", stranger, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = hob }, connect.CodeNotFound, "campaign not found"},
		{"an idempotency key that is not a UUID", a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId, r.IdempotencyKey = hob, "x" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId, r.EncounterId = hob, "x" }, connect.CodeNotFound, "encounter not found"},
		{"a combatant that is not a UUID", a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = "x" }, connect.CodeNotFound, "combatant not found"},
		{"a combatant that is not in the combat", a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = newKey() }, connect.CodeNotFound, "combatant not found"},
		{"a combatant that is not grappled", a.caio, of(c.hob), connect.CodeFailedPrecondition, "the combatant is not grappled"},
		{"the master, for a combatant that is not grappled", a.master, of("Brisa"), connect.CodeFailedPrecondition, "the combatant is not grappled"},
	}
	for _, tc := range tests {
		_, err := c.releaseWith(t, tc.as, tc.edit)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	_, err := c.releaseWith(t, a.caio, of(c.hob))
	wantContestBlocked(t, "ReleaseGrapple", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED)

	// Toren grapples the Hobgoblin: only the grappler's player and the master let go.
	c.grappleWon(t)
	_, err = c.releaseWith(t, a.ana, of(c.hob))
	wantRefusal(t, "a player who is not the grappler's", err, connect.CodePermissionDenied, "only the combatant's player or the master may do this")
	_, err = c.releaseWith(t, a.bia, of(c.hob))
	wantRefusal(t, "the other player who is not the grappler's", err, connect.CodePermissionDenied, "only the combatant's player or the master may do this")
	if !c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("a refused release freed the Hobgoblin")
	}

	// A combat that ended.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	_, err = c.releaseWith(t, a.caio, of(c.hob))
	wantRefusal(t, "a combat that ended", err, connect.CodeFailedPrecondition, "the combat has ended")
}

// TestReleaseGrappleLetsGoWithoutAnAction freezes what a release writes: the condition and the
// hold go, nothing is spent, the grappler's player may do it out of turn, the master may do it
// for an NPC, and everybody reads the line.
func TestReleaseGrappleLetsGoWithoutAnAction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.grappleWon(t)
	if g := c.state(t, a.master).GetGrapples(); len(g) != 1 {
		t.Fatalf("the master reads grapples %v, want one", g)
	}
	// It is not Toren's turn any more: the Hobgoblin's.
	c.advance(t, c.hob)
	res, err := c.releaseWith(t, a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = c.id(t, c.hob) })
	if err != nil {
		t.Fatalf("ReleaseGrapple() error = %v", err)
	}
	if res.GetEncounter() == nil {
		t.Fatal("ReleaseGrapple() answered no encounter")
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("the Hobgoblin is still Grappled after the release")
	}
	if g := c.state(t, a.master).GetGrapples(); len(g) != 0 {
		t.Errorf("the hold stays after the release: %v", g)
	}
	if hob := c.combatant(t, a.master, c.hob); hob.GetActionUsed() || hob.GetBonusActionUsed() || hob.GetReactionUsed() {
		t.Errorf("the release spent the Hobgoblin's economy: %v", hob)
	}
	want := []string{"CONTEST_LOG_LINE_RELEASED Toren>" + c.hob, "CONTEST_LOG_LINE_GRAPPLED Toren>" + c.hob} // newest first
	for who, u := range map[string]*user{"master": a.master, "Toren": a.caio, "Brisa": a.bia} {
		if got := c.answerLines(t, u); !slices.Equal(got, want) {
			t.Errorf("%s reads the log %v, want %v", who, got, want)
		}
	}
	_, err = c.releaseWith(t, a.caio, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = c.id(t, c.hob) })
	wantRefusal(t, "a second release", err, connect.CodeFailedPrecondition, "the combatant is not grappled")

	// An NPC's grapple: the master lets go for it; the player who is held cannot.
	c.grappledByAttack(t, "Brisa", 14)
	_, err = c.releaseWith(t, a.bia, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = c.id(t, "Brisa") })
	wantRefusal(t, "the held player letting go", err, connect.CodePermissionDenied, "only the combatant's player or the master may do this")
	if _, err := c.releaseWith(t, a.master, func(r *playv1.ReleaseGrappleRequest) { r.GrappledId = c.id(t, "Brisa") }); err != nil {
		t.Fatalf("ReleaseGrapple(master) error = %v", err)
	}
	if c.hasCondition(t, "Brisa", condGrappled) {
		t.Error("Brisa is still Grappled after the master let go")
	}
}

// TestAHoldEndsWhenTheMasterTakesTheConditionOff (SRD 5.1, "Grappled"): the hold is a row
// that goes with the condition, and with a combatant that left the fight.
func TestAHoldEndsWhenTheMasterTakesTheConditionOff(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	a.setConditions(t, c.e, c.hob) // none: the master took Grappled off
	if g := c.state(t, a.master).GetGrapples(); len(g) != 0 {
		t.Errorf("the hold stays after the condition went: %v", g)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("the condition came back")
	}

	// A hold whose grappled creature is defeated ends and frees it.
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	c.grappleWon(t)
	a.execSQL(t, `UPDATE combatants SET defeated = true WHERE id = $1`, c.id(t, c.hob))
	a.setConditions(t, c.e, "Goblin", "condition:poisoned") // any change of the combat settles the holds
	if g := c.state(t, a.master).GetGrapples(); len(g) != 0 {
		t.Errorf("the hold on a defeated creature stays: %v", g)
	}
}

// notesOf is the kinds of the notes of a roll, in order.
func notesOf(notes []*playv1.RollNote) []string {
	var out []string
	for _, n := range notes {
		out = append(out, n.GetKind()+":"+n.GetLabelPt())
	}
	return out
}

// TestContestRollNotesNameEveryCircumstanceBehindTheMode freezes checkSources: the conditions
// and the exhaustion that give disadvantage on a Strength check, the effect that gives advantage
// and the Help of an ally, in the order the roll keeps them, in the roll and in the option the
// turn offers before it.
func TestContestRollNotesNameEveryCircumstanceBehindTheMode(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	a.setConditions(t, c.e, "Toren", "condition:poisoned", "condition:frightened")
	a.mustAddEffect(t, c.refresh(t), enhanceAbilityKey, []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) {
		r.CasterId, r.AbilityKey = c.id(t, "Pensantus"), "str"
	})

	// Advantage from the effect, disadvantage from two conditions: they cancel, the check is normal,
	// and the option and the roll say each one.
	wantNotes := []string{"effect:Efeito ativo", "poisoned:Envenenado", "frightened:Amedrontado"}
	opt := grappleOption(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetRollOption()
	if got := notesOf(opt.GetNotes()); !slices.Equal(got, wantNotes) || opt.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL {
		t.Errorf("the option reads notes %v in mode %v, want %v and normal", got, opt.GetMode(), wantNotes)
	}
	c.h.roller.queue(12)
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	}).GetContest()
	roll := started.GetInitiatorRoll()
	if got := notesOf(roll.GetNotes()); !slices.Equal(got, wantNotes) || roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL || roll.GetTotal() != 15 {
		t.Errorf("the roll reads notes %v in mode %v for %d, want %v, normal and 12 + 3", got, roll.GetMode(), roll.GetTotal(), wantNotes)
	}
}

// TestContestRollHelpOfAnAllyGivesAdvantageOnceAndIsUsed freezes the Help that covers a
// contest's check: the note, the advantage, and that the roll uses it up.
func TestContestRollHelpOfAnAllyGivesAdvantageOnceAndIsUsed(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	// Brisa's turn comes after Toren's: Toren helps Brisa? No: Brisa helps Toren on his next Athletics check.
	c.advance(t, "Brisa")
	if _, err := c.helpCheck(t, a.bia, "Brisa", "Toren", skillAthletics); err != nil {
		t.Fatalf("Help() error = %v", err)
	}
	if got := c.state(t, a.caio).GetHelps(); len(got) != 1 || got[0].GetTaskKey() != skillAthletics {
		t.Fatalf("Toren reads helps %v, want the one for Athletics", got)
	}
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	opt := grappleOption(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetRollOption()
	if got := notesOf(opt.GetNotes()); !slices.Equal(got, []string{"help:Ajuda de Brisa"}) || opt.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_ADVANTAGE {
		t.Fatalf("the option reads %v in mode %v, want the help and advantage", got, opt.GetMode())
	}
	c.h.roller.queue(3, 14)
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	}).GetContest()
	if roll := started.GetInitiatorRoll(); roll.GetTotal() != 17 || roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_ADVANTAGE {
		t.Errorf("the roll = %v, want 14 + 3 with advantage", roll)
	}
	if got := c.state(t, a.caio).GetHelps(); len(got) != 0 {
		t.Errorf("the help stays after the check it covered: %v", got)
	}
}
