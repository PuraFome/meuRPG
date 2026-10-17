package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestRollAttackRefusesTheRequest sends one request after another on one fight, each with a field spoiled
// as Toren's player, and reads the exact code and message of each refusal and that none writes anything.
func TestRollAttackRefusesTheRequest(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	long := strings.Repeat("x", 101)
	for _, c := range []struct {
		name string
		edit func(*playv1.RollAttackRequest)
		code connect.Code
		msg  string
	}{
		{"an empty attack key", func(r *playv1.RollAttackRequest) { r.AttackKey = "" }, connect.CodeInvalidArgument, "attack_key must name one of the attacker's attacks"},
		{"an attack key of 101 characters", func(r *playv1.RollAttackRequest) { r.AttackKey = long }, connect.CodeInvalidArgument, "attack_key must name one of the attacker's attacks"},
		{"roll_in_app false", func(r *playv1.RollAttackRequest) {
			r.Roll = &playv1.RollAttackRequest_RollInApp{RollInApp: false}
		}, connect.CodeInvalidArgument, "roll_in_app must be true"},
		{"d20_face with d20_faces", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{4, 9} }, connect.CodeInvalidArgument, "set d20_face or d20_faces, not both"},
		{"d20_face of 0", typedD20(0), connect.CodeInvalidArgument, "d20_face must be 1 to 20"},
		{"d20_face of 21", typedD20(21), connect.CodeInvalidArgument, "d20_face must be 1 to 20"},
		{"no roll at all", func(r *playv1.RollAttackRequest) { r.Roll = nil }, connect.CodeInvalidArgument, "set roll_in_app, d20_face or d20_faces"},
		{"three typed faces", func(r *playv1.RollAttackRequest) { r.Roll, r.D20Faces = nil, []int32{4, 9, 12} }, connect.CodeInvalidArgument, "set roll_in_app, d20_face or d20_faces"},
		{"a typed face out of range", func(r *playv1.RollAttackRequest) { r.Roll, r.D20Faces = nil, []int32{4, 22} }, connect.CodeInvalidArgument, "d20_faces must be 1 to 20 each"},
		{"a key that is not a UUID", func(r *playv1.RollAttackRequest) { r.IdempotencyKey = "not-a-key" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", func(r *playv1.RollAttackRequest) { r.EncounterId = "nope" }, connect.CodeNotFound, "encounter not found"},
		{"an attacker that is not a UUID", func(r *playv1.RollAttackRequest) { r.AttackerId = "nope" }, connect.CodeNotFound, "combatant not found"},
		{"a target that is not a UUID", func(r *playv1.RollAttackRequest) { r.TargetId = "nope" }, connect.CodeNotFound, "combatant not found"},
		{"an offer that is not a UUID", func(r *playv1.RollAttackRequest) { r.OpportunityOfferId = "nope" }, connect.CodeInvalidArgument, "opportunity_offer_id must be a UUID"},
		{"an attack the attacker does not have", func(r *playv1.RollAttackRequest) { r.AttackKey = "equipment:longbow" }, connect.CodeInvalidArgument, "attack_key is not one of the attacker's attacks"},
		{"the extra action for a weapon attack without Haste", func(r *playv1.RollAttackRequest) { r.UseExtraAction = true }, connect.CodeFailedPrecondition, "the combatant has no extra action"},
		{"an offer that is not there", func(r *playv1.RollAttackRequest) {
			r.OpportunityOfferId = "00000000-0000-4000-8000-000000000001"
		}, connect.CodeAborted, "that opportunity offer was answered or is gone"},
		{"a throw back with no window", func(r *playv1.RollAttackRequest) {
			r.CatchWindowId = "00000000-0000-4000-8000-000000000002"
		}, connect.CodeFailedPrecondition, "this reaction window cannot be answered now"},
	} {
		before := a.stateSnapshot(t)
		_, err := a.rawAttack(t, a.caio, e, c.edit)
		wantRefusal(t, c.name, err, c.code, c.msg)
		if a.stateSnapshot(t) != before {
			t.Errorf("%s changed the fight", c.name)
		}
	}
}
