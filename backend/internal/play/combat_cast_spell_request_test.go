package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestCastSpellRefusesTheRequest sends one cast after another on one fight of casters, each with a field of
// the request spoiled, and reads the exact code and message of each refusal and that none writes anything.
func TestCastSpellRefusesTheRequest(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	long := strings.Repeat("x", 101)
	for _, c := range []struct {
		name  string
		fight castFight
		code  connect.Code
		msg   string
	}{
		{"an empty spell key", pensantus("", slotOfLevel(1), nil), connect.CodeInvalidArgument, "spell_key must name one of the caster's spells"},
		{"a spell key of 101 characters", pensantus(long, slotOfLevel(1), nil), connect.CodeInvalidArgument, "spell_key must name one of the caster's spells"},
		{"a key that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.IdempotencyKey = "not-a-key" }), connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.EncounterId = "nope" }), connect.CodeNotFound, "encounter not found"},
		{"a caster that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.CasterId = "nope" }), connect.CodeNotFound, "combatant not found"},
		{"eleven targets", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			for range 11 {
				r.Targets = append(r.Targets, &playv1.SpellTarget{CombatantId: r.GetCasterId()})
			}
		}), connect.CodeInvalidArgument, "targets must have at most 10 entries"},
		{"a target that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = []*playv1.SpellTarget{{CombatantId: "nope"}}
		}), connect.CodeNotFound, "combatant not found"},
		{"a target twice", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = append(r.Targets, r.Targets[0])
		}), connect.CodeInvalidArgument, "targets[1] repeats a combatant"},
		{"negative darts", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = -1 }), connect.CodeInvalidArgument, "targets[0].darts must be 0 to 20"},
		{"twenty-one darts", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 21 }), connect.CodeInvalidArgument, "targets[0].darts must be 0 to 20"},
		{"roll_in_app false", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: false}
		}), connect.CodeInvalidArgument, "roll_in_app must be true"},
		{"d20_face of 0", pensantus(sleepSpell, slotOfLevel(1), d20Face(0)), connect.CodeInvalidArgument, "d20_face must be 1 to 20"},
		{"d20_face of 21", pensantus(sleepSpell, slotOfLevel(1), d20Face(21)), connect.CodeInvalidArgument, "d20_face must be 1 to 20"},
		{"a typed face out of range", pensantus(sleepSpell, slotOfLevel(1), d20Faces(4, 22)), connect.CodeInvalidArgument, "d20_faces must be 1 to 20 each"},
		{"three typed faces", pensantus(sleepSpell, slotOfLevel(1), d20Faces(4, 9, 12)), connect.CodeInvalidArgument, "set roll_in_app, d20_face or d20_faces"},
		{"reveal_hidden from a player", pensantus(burningHands, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			reveal := true
			r.RevealHidden = &reveal
			aimToward(1, 0)(r)
		}), connect.CodePermissionDenied, "only the master decides whether hidden creatures are revealed"},
		{"reveal_hidden from a player before anything else", pensantus("", slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			reveal := false
			r.RevealHidden = &reveal
		}), connect.CodePermissionDenied, "only the master decides whether hidden creatures are revealed"},
		{"a Revivify the caster does not have", pensantus(revivifyKey, slotOfLevel(3), nil), connect.CodeInvalidArgument, "Revivify takes one target: the dead creature, as dead_target_id"},
		{"eleven targets as the master", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			for range 11 {
				r.Targets = append(r.Targets, &playv1.SpellTarget{CombatantId: r.GetCasterId()})
			}
		}).by(asMasterOf), connect.CodeInvalidArgument, "targets must have at most 10 entries"},
	} {
		before := a.stateSnapshot(t)
		_, err := c.fight.send(t, a, e)
		wantRefusal(t, c.name, err, c.code, c.msg)
		if a.stateSnapshot(t) != before {
			t.Errorf("%s changed the fight", c.name)
		}
	}
}
