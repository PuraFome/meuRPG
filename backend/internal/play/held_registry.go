package play

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// heldKindDef is what a kind of held roll does that the gate cannot know.
type heldKindDef struct {
	// write is the table the kind's holds are written to.
	write heldTable
	// alsoRead is the table the kind was written to by an older release, which is only read: the holds left
	// open there are answered by the same path (the weapon attacks of roll_holds, until the contract).
	alsoRead heldTable
	// refusedBy are the tables whose open question refuses a new roll of this kind in the gate, own table
	// first.
	refusedBy []heldTable
	// ownKindOnly says only the holds of this very kind refuse it, and are forgotten when stale: the weapon attack
	// shares its table with the combatant's other rolls, which heldBlocked, called first by its path, refuses it
	// for (and does not when a reaction window replays the attack, a case the gate keeps as it was).
	ownKindOnly bool
	// rerun runs the request the hold kept again, with the answer in the context, by the very path that
	// held it, and gives the path's own response; key is the key the roll settles under.
	rerun func(ctx context.Context, s *Service, h heldRoll, key string) (proto.Message, error)
	// offerOf is the attack and the target a question names (the weapon attack only).
	offerOf func(h heldRoll) (attackKey, targetID string)
	// withFaces pins the d20 a roll was made with in the request the hold keeps (the weapon attack only: a
	// roll with advantage or disadvantage keeps both dice).
	withFaces func(req proto.Message, faces []int)
	// ownRoll says the path rolls its own d20 when nothing is asked, so the gate gives it nothing back then.
	// Today's behavior, pinned by the tests of the group check and of the trap search; they keep rollCheck
	// and the second die of a Perception search.
	ownRoll bool
}

// pinned is the request to keep with a roll: as the caller pinned it, with the dice of the roll.
func (d heldKindDef) pinned(req proto.Message, faces []int) ([]byte, error) {
	if d.withFaces != nil {
		d.withFaces(req, faces)
	}
	return proto.Marshal(req) //nolint:wrapcheck // the gate wraps it
}

// heldRegistry is the registry of the kinds of held roll. It is filled by init, not by its declaration: the
// paths it runs again read it, and Go refuses an initialization that would go round.
var heldRegistry map[heldKind]heldKindDef

func init() {
	outside := []heldTable{inspirationHolds, rollHolds}
	heldRegistry = map[heldKind]heldKindDef{
		heldWeaponAttack: {
			write: inspirationHolds, alsoRead: rollHolds, refusedBy: outside, ownKindOnly: true,
			rerun: rerunAttack, offerOf: attackOfRequest, withFaces: pinAttackFaces,
		},
		heldSpellAttack:       {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).CastSpell)},
		heldEffectSave:        {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).RollEffectSave)},
		heldConcentrationSave: {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).ResolveConcentrationSave)},
		heldSceneCheck:        {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).RollSceneCheck)},
		heldGroupCheck:        {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).RollGroupCheck), ownRoll: true},
		heldTrapSearch:        {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).SearchForTraps), ownRoll: true},
		heldHintTry:           {write: inspirationHolds, refusedBy: outside, rerun: registerRerun((*Service).TryPuzzleHint)},
	}
}

// registerRerun is the rerun of a path: it reads the request the hold kept and calls the path with it.
func registerRerun[Req, Res any, PReq interface {
	*Req
	proto.Message
}, PRes interface {
	*Res
	proto.Message
}](path func(*Service, context.Context, *connect.Request[Req]) (*connect.Response[Res], error)) func(context.Context, *Service, heldRoll, string) (proto.Message, error) {
	return func(ctx context.Context, s *Service, h heldRoll, _ string) (proto.Message, error) {
		orig := PReq(new(Req))
		if err := proto.Unmarshal(h.Request, orig); err != nil {
			return nil, s.dbError(ctx, "read the held roll", err)
		}
		out, err := path(s, ctx, connect.NewRequest((*Req)(orig)))
		if err != nil {
			return nil, err //nolint:wrapcheck // the path's own refusal
		}
		return PRes(out.Msg), nil
	}
}

// rerunAttack runs the weapon attack again. The attack that a reaction window held first has its event
// under the request's key: the question's answer settles under the answer's own.
func rerunAttack(ctx context.Context, s *Service, h heldRoll, key string) (proto.Message, error) {
	var orig playv1.RollAttackRequest
	if err := proto.Unmarshal(h.Request, &orig); err != nil {
		return nil, s.dbError(ctx, "read the held roll", err)
	}
	if h.Window {
		orig.IdempotencyKey = key
	}
	out, err := s.RollAttack(ctx, connect.NewRequest(&orig))
	if err != nil {
		return nil, err //nolint:wrapcheck // the path's own refusal
	}
	return out.Msg, nil
}

// attackOfRequest is the attack and the target of the request a weapon attack's hold kept.
func attackOfRequest(h heldRoll) (attackKey, targetID string) {
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(h.Request, &orig) // a request this module wrote
	return orig.GetAttackKey(), orig.GetTargetId()
}

// pinAttackFaces keeps both d20 of a weapon attack rolled with advantage or disadvantage in the request,
// so that the answer settles this roll and not another.
func pinAttackFaces(req proto.Message, faces []int) {
	r, ok := req.(*playv1.RollAttackRequest)
	if !ok {
		return
	}
	sent := len(r.D20Faces) > 0 // a die typed as a list of one is kept as such: the answer asks for the roll again
	r.D20Faces = nil
	if len(faces) > 1 || sent {
		r.D20Faces = faces32(faces)
	}
}
