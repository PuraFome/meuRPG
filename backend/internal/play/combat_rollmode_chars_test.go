package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of the roll mode requests (combat_rollmode.go): every
// refusal of RequestRollMode, AnswerRollModeRequest and CancelRollModeRequest
// and of the request an attack uses (chooseMode), with its code and message.

func (a *armed) rmRequestRaw(t *testing.T, u *user, e *playv1.Encounter, edit func(*playv1.RequestRollModeRequest)) (*playv1.RequestRollModeResponse, error) {
	t.Helper()
	r := &playv1.RequestRollModeRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, "Toren"), AttackKey: rapier, TargetId: a.id(t, "Goblin"),
		RollMode: playv1.RollMode_ROLL_MODE_ADVANTAGE, Reason: "ele está distraído", IdempotencyKey: newKey(),
	}
	if edit != nil {
		edit(r)
	}
	res, err := u.combat.RequestRollMode(t.Context(), connect.NewRequest(r))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) rmCancel(t *testing.T, u *user, e *playv1.Encounter, id string) error {
	t.Helper()
	_, err := u.combat.CancelRollModeRequest(t.Context(), connect.NewRequest(&playv1.CancelRollModeRequestRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), RequestId: id, IdempotencyKey: newKey(),
	}))
	return err
}

func rmWant(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	if connect.CodeOf(err) != code {
		t.Errorf("%s: code = %v (%v), want %v", call, connect.CodeOf(err), err, code)
		return
	}
	if ce := new(connect.Error); err != nil && msg != "" {
		if !asConnect(err, ce) || ce.Message() != msg {
			t.Errorf("%s: message = %q, want %q", call, err.Error(), msg)
		}
	}
}

func rmOK(t *testing.T, call string, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: error = %v, want none", call, err)
	}
}

func asConnect(err error, into *connect.Error) bool {
	if ce, ok := err.(*connect.Error); ok { //nolint:errorlint // a handler returns it bare
		*into = *ce
		return true
	}
	return false
}

func TestRollModeRequest_RefusalsOfRequestRollMode(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")

	_, err := a.rmRequestRaw(t, a.master, e, nil)
	rmWant(t, "master", err, connect.CodePermissionDenied, "the master sets the mode on the roll")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.IdempotencyKey = "" })
	rmWant(t, "no key", err, connect.CodeInvalidArgument, "")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.EncounterId = "nope" })
	rmWant(t, "bad encounter", err, connect.CodeNotFound, "encounter not found")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.AttackerId = "nope" })
	rmWant(t, "bad attacker", err, connect.CodeNotFound, "combatant not found")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.TargetId = "nope" })
	rmWant(t, "bad target", err, connect.CodeNotFound, "combatant not found")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.AttackKey = "" })
	rmWant(t, "no attack key", err, connect.CodeInvalidArgument, "attack_key must name one of the attacker's attacks")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) {
		r.AttackKey = string(make([]byte, 101))
	})
	rmWant(t, "long attack key", err, connect.CodeInvalidArgument, "attack_key must name one of the attacker's attacks")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.RollMode = playv1.RollMode_ROLL_MODE_UNSPECIFIED })
	rmWant(t, "no mode", err, connect.CodeInvalidArgument, "roll_mode must be set")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.Reason = "" })
	rmWant(t, "no reason", err, connect.CodeInvalidArgument, "the reason must be 1 to 120 characters on one line")

	// In the transaction: the attacker and the target must exist, be the
	// player's own attacker, and the attack one of its attacks.
	unknown := newKey()
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.AttackerId = unknown })
	rmWant(t, "unknown attacker", err, connect.CodeNotFound, "")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.TargetId = unknown })
	rmWant(t, "unknown target", err, connect.CodeNotFound, "")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.AttackerId = a.id(t, "Brisa") })
	rmWant(t, "another player's attacker", err, connect.CodePermissionDenied, "")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.AttackKey = "weapon:nothing" })
	rmWant(t, "unknown attack", err, connect.CodeInvalidArgument, "attack_key is not one of the attacker's attacks")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.RollMode = playv1.RollMode_ROLL_MODE_NORMAL })
	rmWant(t, "normal is not better", err, connect.CodeInvalidArgument, "roll_mode must be better than the suggestion: roll the suggested mode, or disadvantage, without asking")
	_, err = a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.RollMode = playv1.RollMode_ROLL_MODE_DISADVANTAGE })
	rmWant(t, "disadvantage is not better", err, connect.CodeInvalidArgument, "roll_mode must be better than the suggestion: roll the suggested mode, or disadvantage, without asking")
}

func TestRollModeRequest_NotYourTurn(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t) // Brisa plays first
	_, err := a.rmRequestRaw(t, a.caio, e, nil)
	wantEncounterBlocked(t, err, reasonNotTurn)
}

func TestRollModeRequest_RequestReplacesTheEarlierOneAndCancelIsIdempotent(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	first, err := a.rmRequestRaw(t, a.caio, e, nil)
	if err != nil {
		t.Fatalf("RequestRollMode() error = %v", err)
	}
	if first.GetRequest().GetStatus() != playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_PENDING ||
		first.GetRequest().GetSuggestedMode() != playv1.RollMode_ROLL_MODE_NORMAL ||
		first.GetRequest().GetRequestedMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE ||
		first.GetRequest().GetReason() != "ele está distraído" || first.GetRequest().GetAttackKey() != rapier {
		t.Errorf("request = %v, want pending, normal suggested, advantage asked, with its reason and attack", first.GetRequest())
	}
	second, err := a.rmRequestRaw(t, a.caio, e, func(r *playv1.RequestRollModeRequest) { r.Reason = "outra razão" })
	if err != nil {
		t.Fatalf("second RequestRollMode() error = %v", err)
	}
	var open []string
	for _, r := range second.GetEncounter().GetRollModeRequests() {
		if r.GetStatus() == playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_PENDING {
			open = append(open, r.GetId())
		}
	}
	if len(open) != 1 || open[0] != second.GetRequest().GetId() {
		t.Errorf("pending = %v, want only the second request %s: the first one closed", open, second.GetRequest().GetId())
	}
	if err := a.answerMode(t, a.master, e, first.GetRequest().GetId(), playv1.RollMode_ROLL_MODE_ADVANTAGE); err == nil {
		t.Error("answering the replaced request succeeded, want it blocked as closed")
	} else {
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED)
	}

	id := second.GetRequest().GetId()
	rmWant(t, "unknown request", a.rmCancel(t, a.caio, e, newKey()), connect.CodeNotFound, "roll mode request not found")
	rmWant(t, "bad request id", a.rmCancel(t, a.caio, e, "nope"), connect.CodeNotFound, "roll mode request not found")
	rmWant(t, "another player canceling", a.rmCancel(t, a.ana, e, id), connect.CodePermissionDenied, "")
	rmOK(t, "cancel", a.rmCancel(t, a.caio, e, id))
	rmOK(t, "cancel again", a.rmCancel(t, a.caio, e, id)) // taken back already: nothing differs
	if got := a.get(t, a.master).GetRollModeRequests(); len(got) != 0 {
		t.Errorf("queue after the cancel = %v, want it empty", got)
	}
	err = a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_ADVANTAGE)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED)

	// The master may take a request back too.
	third, err := a.rmRequestRaw(t, a.caio, e, nil)
	if err != nil {
		t.Fatalf("third RequestRollMode() error = %v", err)
	}
	rmOK(t, "master canceling", a.rmCancel(t, a.master, e, third.GetRequest().GetId()))
}

func TestRollModeRequest_AnswerRefusals(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	req, err := a.rmRequestRaw(t, a.caio, e, nil)
	if err != nil {
		t.Fatalf("RequestRollMode() error = %v", err)
	}
	id := req.GetRequest().GetId()
	rmWant(t, "unknown request", a.answerMode(t, a.master, e, newKey(), playv1.RollMode_ROLL_MODE_ADVANTAGE), connect.CodeNotFound, "roll mode request not found")
	rmWant(t, "no mode", a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_UNSPECIFIED), connect.CodeInvalidArgument, "decided_mode must be set")
	rmWant(t, "bad id", a.answerMode(t, a.master, e, "nope", playv1.RollMode_ROLL_MODE_ADVANTAGE), connect.CodeNotFound, "roll mode request not found")
	rmOK(t, "answer", a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_ADVANTAGE))
	rmOK(t, "the same answer again", a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_ADVANTAGE))
	err = a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_NORMAL)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED)
	rmWant(t, "answer differently", err, connect.CodeFailedPrecondition, "the request is answered already")
	// The requester may take an answered request back.
	rmOK(t, "cancel an answered request", a.rmCancel(t, a.caio, e, id))
}

// An attack that names a request: chooseMode's refusals.
func TestRollModeRequest_AttackUsingARequest(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	use := func(id string) func(*playv1.RollAttackRequest) {
		return func(r *playv1.RollAttackRequest) { r.RollModeRequestId, r.D20Faces = id, []int32{4, 17} }
	}
	_, err := a.attack(t, a.caio, e, "Toren", rapier, "Goblin", use(newKey()))
	rmWant(t, "unknown request", err, connect.CodeNotFound, "roll mode request not found")

	req, err := a.rmRequestRaw(t, a.caio, e, nil)
	if err != nil {
		t.Fatalf("RequestRollMode() error = %v", err)
	}
	id := req.GetRequest().GetId()
	// Another target: the request is for another attack.
	_, err = a.attack(t, a.caio, e, "Toren", rapier, "Brisa", use(id))
	rmWant(t, "another target", err, connect.CodeInvalidArgument, "roll_mode_request_id is for another attack")
	_, err = a.attack(t, a.caio, e, "Toren", rapier, "Goblin", use(id))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_PENDING)
	if err := a.answerMode(t, a.master, e, id, playv1.RollMode_ROLL_MODE_DISADVANTAGE); err != nil {
		t.Fatalf("AnswerRollModeRequest() error = %v", err)
	}
	// The master may answer with a mode that is not the one asked: the player
	// rolls it (two d20, the lower counts).
	res := a.mustAttack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollModeRequestId, r.D20Faces = id, []int32{15, 3}
	})
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || res.GetRoll().GetD20().GetCountedIndex() != 1 {
		t.Errorf("roll = %v, want the master's disadvantage with the lower d20 counted", res.GetRoll())
	}
}

// chooseMode with no request: a mode asked on the roll itself.
func TestRollModeRequest_ModeAskedOnTheRoll(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	_, err := a.attack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.D20Faces = playv1.RollMode_ROLL_MODE_DISADVANTAGE, []int32{9, 2}
	})
	rmWant(t, "disadvantage without a reason", err, connect.CodeInvalidArgument, "the reason must be 1 to 120 characters on one line")
	_, err = a.attack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.ModeReason, r.D20Faces = playv1.RollMode_ROLL_MODE_ADVANTAGE, "x", []int32{9, 2}
	})
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_NEEDS_APPROVAL)
	// The suggestion asked for again needs no reason.
	a.mustAttack(t, a.caio, e, "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) {
		r.RollMode, r.D20Faces = playv1.RollMode_ROLL_MODE_NORMAL, []int32{9}
	})
}
