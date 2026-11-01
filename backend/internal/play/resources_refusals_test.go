package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of the request checks and the refusals of the rest, hit dice, Arcane
// Recovery and rage handlers, frozen before they were split into phases: the exact code
// and message of each refusal, and which check comes first.

func TestSpendHitDiceRequestRefusalsFrozen(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	const facesMsg = "faces must be the size of one of the character's hit dice"
	typed := func(n int32) func(*playv1.SpendHitDiceRequest) {
		return func(q *playv1.SpendHitDiceRequest) { q.Roll = &playv1.SpendHitDiceRequest_TypedFace{TypedFace: n} }
	}
	for _, tc := range []struct {
		name  string
		faces int32
		edit  func(*playv1.SpendHitDiceRequest)
		msg   string
	}{
		{"zero faces", 0, nil, facesMsg},
		{"faces over the largest die", 13, nil, facesMsg},
		{"roll_in_app false", 10, func(q *playv1.SpendHitDiceRequest) { q.Roll = &playv1.SpendHitDiceRequest_RollInApp{RollInApp: false} }, "roll_in_app must be true"},
		{"no roll", 10, func(q *playv1.SpendHitDiceRequest) { q.Roll = nil }, "set roll_in_app or typed_face"},
		{"typed zero", 10, typed(0), "typed_face must be a face of the die"},
		{"typed over the die", 10, typed(11), "typed_face must be a face of the die"},
		{"bad key", 10, func(q *playv1.SpendHitDiceRequest) { q.IdempotencyKey = "no" }, "idempotency_key must be a UUID"},
		{"bad character", 10, func(q *playv1.SpendHitDiceRequest) { q.CharacterId = "no" }, "character not found"},
	} {
		edit := tc.edit
		if edit == nil {
			edit = func(*playv1.SpendHitDiceRequest) {}
		}
		_, err := r.spend(t, r.ana, r.fighter, tc.faces, edit)
		code := connect.CodeInvalidArgument
		if tc.name == "bad character" {
			code = connect.CodeNotFound
		}
		wantRefusal(t, tc.name, err, code, tc.msg)
	}
	// The check of the key comes before the check of the die, the character's before both.
	_, err := r.spend(t, r.ana, r.fighter, 0, func(q *playv1.SpendHitDiceRequest) { q.IdempotencyKey = "no" })
	wantRefusal(t, "key before die", err, connect.CodeInvalidArgument, "idempotency_key must be a UUID")
	// Another player's character is not theirs.
	_, err = r.spend(t, r.bia, r.fighter, 10)
	wantRefusal(t, "stranger", err, connect.CodePermissionDenied, "only the character's player or the master may do this")
	// A die size the character does not have is the keeper's refusal.
	_, err = r.spend(t, r.ana, r.fighter, 6)
	wantCode(t, "SpendHitDice(d6 of a fighter)", err, connect.CodeInvalidArgument)
}

func TestArcaneRecoveryRequestRefusalsFrozen(t *testing.T) {
	t.Parallel()
	r := newRecoveryTable(t)
	wiz := r.wizard(t, r.ana, "Pensantus", 4)
	call := func(edit func(*playv1.UseArcaneRecoveryRequest)) error {
		req := &playv1.UseArcaneRecoveryRequest{CampaignId: r.campaign, CharacterId: wiz.GetId(), IdempotencyKey: newKey()}
		edit(req)
		_, err := r.ana.resource.UseArcaneRecovery(t.Context(), connect.NewRequest(req))
		return err
	}
	slot := func(level, count int32) *playv1.SpellSlotBack {
		return &playv1.SpellSlotBack{Level: level, Count: count}
	}
	const levelsMsg = "slots must name one to nine spell levels"
	wantRefusal(t, "no slots", call(func(*playv1.UseArcaneRecoveryRequest) {}), connect.CodeInvalidArgument, levelsMsg)
	tooMany := func(q *playv1.UseArcaneRecoveryRequest) {
		for i := range 10 {
			q.Slots = append(q.Slots, slot(int32(i%9)+1, 1))
		}
	}
	wantRefusal(t, "ten entries", call(tooMany), connect.CodeInvalidArgument, levelsMsg)
	wantRefusal(t, "level 0", call(func(q *playv1.UseArcaneRecoveryRequest) { q.Slots = []*playv1.SpellSlotBack{slot(0, 1)} }), connect.CodeInvalidArgument, "a slot level must be 1 to 9")
	wantRefusal(t, "level 10", call(func(q *playv1.UseArcaneRecoveryRequest) { q.Slots = []*playv1.SpellSlotBack{slot(10, 1)} }), connect.CodeInvalidArgument, "a slot level must be 1 to 9")
	const countMsg = "the count of slots of a level must be at least 1"
	wantRefusal(t, "count 0", call(func(q *playv1.UseArcaneRecoveryRequest) { q.Slots = []*playv1.SpellSlotBack{slot(1, 0)} }), connect.CodeInvalidArgument, countMsg)
	wantRefusal(t, "count 10", call(func(q *playv1.UseArcaneRecoveryRequest) { q.Slots = []*playv1.SpellSlotBack{slot(1, 10)} }), connect.CodeInvalidArgument, countMsg)
	wantRefusal(t, "repeated level", call(func(q *playv1.UseArcaneRecoveryRequest) { q.Slots = []*playv1.SpellSlotBack{slot(1, 1), slot(1, 1)} }), connect.CodeInvalidArgument, "each slot level may appear once")
	wantRefusal(t, "bad key", call(func(q *playv1.UseArcaneRecoveryRequest) { q.IdempotencyKey = "no" }), connect.CodeInvalidArgument, "idempotency_key must be a UUID")
	wantRefusal(t, "bad character", call(func(q *playv1.UseArcaneRecoveryRequest) { q.CharacterId = "no" }), connect.CodeNotFound, "character not found")
}

func TestRageEndRefusalsFrozen(t *testing.T) {
	t.Parallel()
	a, e := berserkerTable(t, 3, "subclass:berserker")
	brisa := a.id(t, "Brisa")
	end := func(u *user) error {
		_, err := u.combat.EndRage(t.Context(), connect.NewRequest(&playv1.EndRageRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: brisa, IdempotencyKey: newKey()}))
		return err
	}
	answer := func(u *user, endIt bool) error {
		_, err := u.combat.AnswerRageEnd(t.Context(), connect.NewRequest(&playv1.AnswerRageEndRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: brisa, EndRage: endIt, IdempotencyKey: newKey()}))
		return err
	}
	wantEncounterBlocked(t, end(a.bia), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_RAGING)
	wantEncounterBlocked(t, answer(a.bia, true), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_RAGE_END_NOT_PENDING)
	wantCode(t, "another player ends the rage", end(a.caio), connect.CodePermissionDenied)
	wantCode(t, "another player answers", answer(a.caio, true), connect.CodePermissionDenied)
	if _, err := a.rage(t, e, false); err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	// Raging takes the bonus action of the turn; ending the rage by choice takes it too.
	wantEncounterBlocked(t, end(a.bia), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED)
	// The master is held to nothing of the economy, and the rage ends.
	if err := end(a.master); err != nil {
		t.Fatalf("EndRage(master) error = %v", err)
	}
	wantEncounterBlocked(t, end(a.master), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_RAGING)
	for _, bad := range []struct {
		name string
		call func() error
	}{
		{"EndRage bad key", func() error {
			_, err := a.bia.combat.EndRage(t.Context(), connect.NewRequest(&playv1.EndRageRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: brisa, IdempotencyKey: "no"}))
			return err
		}},
		{"AnswerRageEnd bad encounter", func() error {
			_, err := a.bia.combat.AnswerRageEnd(t.Context(), connect.NewRequest(&playv1.AnswerRageEndRequest{CampaignId: a.campaignID, EncounterId: "no", CombatantId: brisa, IdempotencyKey: newKey()}))
			return err
		}},
	} {
		if err := bad.call(); connect.CodeOf(err) == connect.CodeUnknown || err == nil {
			t.Errorf("%s: err = %v, want a typed refusal", bad.name, err)
		}
	}
}
