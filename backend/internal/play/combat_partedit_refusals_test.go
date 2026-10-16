package play

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These tests freeze what the master's Desfazer and his removal of an extra refuse, and
// in which order: the request is read field by field (the first bad one is the error),
// then the combat and the pending damage are checked. They need the database
// (MEURPG_TEST_DATABASE_URL).

// wantPartsRefusal requires a connect error with the code and the message.
func wantPartsRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s succeeded, want %v %q", call, code, msg)
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("%s error = %v, want a connect error", call, err)
	}
	if cerr.Code() != code || cerr.Message() != msg {
		t.Errorf("%s = %v %q, want %v %q", call, cerr.Code(), cerr.Message(), code, msg)
	}
}

// TestUndoLastActionRefusesTheRequestFieldByField: a key, a combat and an expected event
// that are not UUIDs are refused in that order, the first bad one deciding.
func TestUndoLastActionRefusesTheRequestFieldByField(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	good := func() *playv1.UndoLastActionRequest {
		return &playv1.UndoLastActionRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), ExpectedEventId: newKey(), IdempotencyKey: newKey()}
	}
	call := func(edit func(*playv1.UndoLastActionRequest)) error {
		req := good()
		edit(req)
		_, err := a.master.combat.UndoLastAction(t.Context(), connect.NewRequest(req))
		return err
	}
	const (
		keyMsg      = "idempotency_key must be a UUID"
		encounter   = "encounter not found"
		expectedMsg = "expected_event_id must be a UUID"
	)
	tests := []struct {
		name string
		edit func(*playv1.UndoLastActionRequest)
		code connect.Code
		msg  string
	}{
		{"a key that is not a UUID", func(r *playv1.UndoLastActionRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, keyMsg},
		{"no key", func(r *playv1.UndoLastActionRequest) { r.IdempotencyKey = "" }, connect.CodeInvalidArgument, keyMsg},
		{"a combat that is not a UUID", func(r *playv1.UndoLastActionRequest) { r.EncounterId = "x" }, connect.CodeNotFound, encounter},
		{"an expected event that is not a UUID", func(r *playv1.UndoLastActionRequest) { r.ExpectedEventId = "x" }, connect.CodeInvalidArgument, expectedMsg},
		{"no expected event", func(r *playv1.UndoLastActionRequest) { r.ExpectedEventId = "" }, connect.CodeInvalidArgument, expectedMsg},
		{"every field bad: the key first", func(r *playv1.UndoLastActionRequest) {
			r.IdempotencyKey, r.EncounterId, r.ExpectedEventId = "x", "x", "x"
		}, connect.CodeInvalidArgument, keyMsg},
		{"the combat and the expected event bad: the combat first", func(r *playv1.UndoLastActionRequest) {
			r.EncounterId, r.ExpectedEventId = "x", "x"
		}, connect.CodeNotFound, encounter},
		{"a combat that does not exist", func(r *playv1.UndoLastActionRequest) { r.EncounterId = newKey() }, connect.CodeNotFound, encounter},
	}
	for _, tt := range tests {
		wantPartsRefusal(t, "UndoLastAction("+tt.name+")", call(tt.edit), tt.code, tt.msg)
	}

	// With a good request, the combat's beginning is no action to undo...
	err := call(func(*playv1.UndoLastActionRequest) {})
	b := wantBlockedBy(t, "UndoLastAction(the beginning of the combat)", err, blockedNothingToUndo)
	wantPartsRefusal(t, "UndoLastAction(the beginning of the combat)", err, connect.CodeFailedPrecondition, "there is no action to undo")
	if b.GetReason() != blockedNothingToUndo {
		t.Errorf("reason = %v, want NOTHING_TO_UNDO", b.GetReason())
	}
	// ... and once the combat is over nothing is undone in it.
	a.endEncounter(t, e)
	err = call(func(*playv1.UndoLastActionRequest) {})
	wantBlockedBy(t, "UndoLastAction(an ended combat)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
	wantPartsRefusal(t, "UndoLastAction(an ended combat)", err, connect.CodeFailedPrecondition, "the combat has ended")
}

// TestUndoLastActionAnswersAnotherAction: a master whose screen is stale is told that
// another action is the last now (aborted), and nothing changes.
func TestUndoLastActionAnswersAnotherAction(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Toren")
	a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", d20(15))
	wantPartsRefusal(t, "UndoLastAction(a stale event)", a.undo(t, a.master, e, newKey()), connect.CodeAborted, "another action is the last now")
}

// partEditTable is a hit of Toren on the restrained goblin, with the damage rolled with
// Sneak Attack, ready for the master to take the extra out.
func partEditTable(t *testing.T) (a *armed, e *playv1.Encounter, pendingID string) {
	t.Helper()
	a, e = rollsTable(t)
	e = a.passTo(t, e, "Toren")
	a.setConditions(t, e, "Goblin", "condition:restrained")
	res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	return a, e, res.GetPendingDamage().GetId()
}

// rollWithSneakAttack rolls the pending damage with the extra marked, typing the sums.
func (a *armed) rollWithSneakAttack(t *testing.T, pendingID string) {
	t.Helper()
	a.mustDamage(t, a.caio, a.get(t, a.caio), pendingID, func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "sneak-attack"}}
		r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 3}, {PartKey: "sneak-attack", Sum: 5}}
	})
}

// TestRemoveDamagePartRefusesTheRequestFieldByField: the order of the checks of the
// request is the master's role, the key, the combat, the pending damage, the part and
// the reason.
func TestRemoveDamagePartRefusesTheRequestFieldByField(t *testing.T) {
	t.Parallel()
	a, e, pendingID := partEditTable(t)
	a.rollWithSneakAttack(t, pendingID)
	call := func(edit func(*playv1.RemoveDamagePartRequest)) error {
		req := &playv1.RemoveDamagePartRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: pendingID, PartKey: "sneak-attack", Reason: "motivo", IdempotencyKey: newKey(),
		}
		edit(req)
		_, err := a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(req))
		return err
	}
	const (
		keyMsg    = "idempotency_key must be a UUID"
		encounter = "encounter not found"
		pending   = "pending damage not found"
		partMsg   = "part_key must name a part of the damage"
		reasonMsg = "the reason must be 1 to 120 characters on one line"
	)
	tests := []struct {
		name string
		edit func(*playv1.RemoveDamagePartRequest)
		code connect.Code
		msg  string
	}{
		{"a key that is not a UUID", func(r *playv1.RemoveDamagePartRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, keyMsg},
		{"a combat that is not a UUID", func(r *playv1.RemoveDamagePartRequest) { r.EncounterId = "x" }, connect.CodeNotFound, encounter},
		{"a pending damage that is not a UUID", func(r *playv1.RemoveDamagePartRequest) { r.PendingDamageId = "x" }, connect.CodeNotFound, pending},
		{"no part", func(r *playv1.RemoveDamagePartRequest) { r.PartKey = "" }, connect.CodeInvalidArgument, partMsg},
		{"a part key of 101 characters", func(r *playv1.RemoveDamagePartRequest) { r.PartKey = strings.Repeat("p", 101) }, connect.CodeInvalidArgument, partMsg},
		{"no reason", func(r *playv1.RemoveDamagePartRequest) { r.Reason = "" }, connect.CodeInvalidArgument, reasonMsg},
		{"a reason of 121 characters", func(r *playv1.RemoveDamagePartRequest) { r.Reason = strings.Repeat("r", 121) }, connect.CodeInvalidArgument, reasonMsg},
		{"a reason on two lines", func(r *playv1.RemoveDamagePartRequest) { r.Reason = "a\nb" }, connect.CodeInvalidArgument, reasonMsg},
		{"every field bad: the key first", func(r *playv1.RemoveDamagePartRequest) {
			r.IdempotencyKey, r.EncounterId, r.PendingDamageId, r.PartKey, r.Reason = "x", "x", "x", "", ""
		}, connect.CodeInvalidArgument, keyMsg},
		{"all but the key bad: the combat first", func(r *playv1.RemoveDamagePartRequest) {
			r.EncounterId, r.PendingDamageId, r.PartKey, r.Reason = "x", "x", "", ""
		}, connect.CodeNotFound, encounter},
		{"the pending damage, the part and the reason bad: the pending damage first", func(r *playv1.RemoveDamagePartRequest) {
			r.PendingDamageId, r.PartKey, r.Reason = "x", "", ""
		}, connect.CodeNotFound, pending},
		{"the part and the reason bad: the part first", func(r *playv1.RemoveDamagePartRequest) {
			r.PartKey, r.Reason = "", ""
		}, connect.CodeInvalidArgument, partMsg},
		{"a pending damage that does not exist", func(r *playv1.RemoveDamagePartRequest) { r.PendingDamageId = newKey() }, connect.CodeNotFound, pending},
		{"a combat that does not exist", func(r *playv1.RemoveDamagePartRequest) { r.EncounterId = newKey() }, connect.CodeNotFound, encounter},
	}
	for _, tt := range tests {
		wantPartsRefusal(t, "RemoveDamagePart("+tt.name+")", call(tt.edit), tt.code, tt.msg)
	}
	// A part key of exactly 100 characters is read, and then no part of the damage.
	err := call(func(r *playv1.RemoveDamagePartRequest) { r.PartKey = strings.Repeat("p", 100) })
	wantBlockedBy(t, "RemoveDamagePart(a part of 100 characters)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
}

// TestRemoveDamagePartRefusesWhatCannotComeOut: a damage not rolled yet, the weapon, a
// part the damage never had and an extra already out are refused, each with its reason;
// an ended combat is refused before the damage is looked at.
func TestRemoveDamagePartRefusesWhatCannotComeOut(t *testing.T) {
	t.Parallel()
	a, e, pendingID := partEditTable(t)
	remove := func(part string) error {
		_, err := a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: pendingID, PartKey: part, Reason: "motivo", IdempotencyKey: newKey(),
		}))
		return err
	}
	// The hit is not rolled yet: there is no damage to take an extra out of.
	err := remove("sneak-attack")
	wantBlockedBy(t, "RemoveDamagePart(a damage not rolled)", err, blockedNotRolled)
	wantPartsRefusal(t, "RemoveDamagePart(a damage not rolled)", err, connect.CodeFailedPrecondition, "the damage was not rolled")

	a.rollWithSneakAttack(t, pendingID)
	const notRemovable = "that part cannot be taken out"
	for _, part := range []string{"weapon", "colossus-slayer", "divine-smite", "divine-smite-extra", "nothing-like-it"} {
		err := remove(part)
		wantBlockedBy(t, "RemoveDamagePart("+part+")", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
		wantPartsRefusal(t, "RemoveDamagePart("+part+")", err, connect.CodeFailedPrecondition, notRemovable)
	}
	if err := remove("sneak-attack"); err != nil {
		t.Fatalf("RemoveDamagePart(sneak-attack) error = %v", err)
	}
	wantPartsRefusal(t, "RemoveDamagePart(the extra twice)", remove("sneak-attack"), connect.CodeFailedPrecondition, notRemovable)

	a.endEncounter(t, e)
	err = remove("sneak-attack")
	wantBlockedBy(t, "RemoveDamagePart(an ended combat)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
}

// TestRemoveDamagePartRefusesADamageAlreadyAppliedToACharacter: the damage that went
// through a player's vitals is corrected by hand, not by taking an extra out.
func TestRemoveDamagePartRefusesADamageAlreadyAppliedToACharacter(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e = a.passTo(t, e, "Goblin")
	a.h.roller.queue(4)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(18))
	pending := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if _, err := a.settle(t, a.master, e, pending.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	_, err := a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: pending.GetId(), PartKey: "sneak-attack", Reason: "motivo", IdempotencyKey: newKey(),
	}))
	wantBlockedBy(t, "RemoveDamagePart(a damage applied to a character)", err, blockedDamageResolved)
	wantPartsRefusal(t, "RemoveDamagePart(a damage applied to a character)", err, connect.CodeFailedPrecondition, "the damage was applied to a character already")
}
