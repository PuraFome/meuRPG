package play

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Using a Bardic Inspiration die (SRD 5.1, Bard 1): the creature "can wait until after it
// rolls the d20 before deciding to use the Bardic Inspiration die, but must decide before
// the GM says whether the roll succeeds or fails". A character that holds a die and rolls an
// attack therefore does not get the result with its roll: RollAttack rolls (or takes) the d20
// and keeps it with the request in a roll_holds row, writes nothing else, and answers with
// the offer. AnswerBardicInspiration then asks RollAttack again for the same request with the
// d20 already rolled, and with the die rolled and added when the player chose to use it.
// A weapon attack (and the attack cantrips) is held here, in roll_holds. The other d20 rolls of
// the character wait in inspiration_holds, the table of the scene check (inspiration_held.go): the
// spell attack against one target, the saving throw of an effect, the concentration save, the trap
// search and the puzzle hint try. The saving throws the app rolls for a target in a caster's call
// (a spell's save, a hit rider's) do not wait for the die: the target's player is not rolling.
// The same roll_holds row also asks the halfling's Lucky question about a d20 that shows a 1
// (combat_lucky.go), before the die's.

// bonusUse is a Bardic Inspiration die rolled and added to an attack roll, with the die as it
// was held, for the undo.
type bonusUse struct {
	Sides, Face  int32
	From         string
	ExpiresRound int32
}

// windowHoldSuffix marks the key of a held roll that a reaction window held before it was rolled.
const windowHoldSuffix = "#window"

// d20Sides is the d20 the held roll was made with.
const d20Sides = 20

// dieHeldBy is the Bardic Inspiration die the combatant holds in the round, nil when it holds none.
func dieHeldBy(c playdb.Combatant, round int32) *playv1.InspirationDie {
	if !holdsDie(c, round) {
		return nil
	}
	return &playv1.InspirationDie{Sides: *c.InspirationSides, ExpiresAtRound: *c.InspirationExpiresRound, FromCombatantId: deref(c.InspirationFrom)}
}

// offerOf is the question a hold asks the player: the d20 with the attack's bonus, the attack, its
// target and the die the attacker holds.
func offerOf(hold playdb.RollHold, die *playv1.InspirationDie, cs []playdb.Combatant, physical bool) *playv1.InspirationOffer {
	if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == die.GetFromCombatantId() }); i >= 0 {
		die.FromLabel = cs[i].Label
	}
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(hold.Request, &orig) // a request this module wrote
	return &playv1.InspirationOffer{
		HoldId: hold.ID, Die: die, AttackKey: orig.GetAttackKey(), TargetId: orig.GetTargetId(), D20: heldD20(hold, physical),
	}
}

// heldD20 is the d20 of a hold, with the attack's bonus as the modifier: both dice of a roll with
// advantage or disadvantage, the one that counts marked, and the die Lucky rolled again once the
// player did (the hold's face and request are already the new ones then).
func heldD20(hold playdb.RollHold, physical bool) *playv1.DiceRoll {
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(hold.Request, &orig) // a request this module wrote
	roll := diceRoll(1, 20, []int32{hold.Face}, hold.Modifier, hold.Face+hold.Modifier, physical)
	if faces := orig.GetD20Faces(); len(faces) == pairDice { // advantage or disadvantage: both dice, the one that counts marked
		mode, _ := modeFromProto(orig.GetRollMode())
		counted := mode.Pick(intsOf(faces))
		roll = diceRoll(pairDice, 20, faces, hold.Modifier, hold.Face+hold.Modifier, physical)
		roll.CountedIndex = clamp32(counted, 0, 1)
	}
	roll.Rerolled = luckyOfHold(hold).proto()
	return roll
}

// heldAttackOffer is what RollAttack answers for a held roll: the d20 and the question, the attack
// not resolved (no outcome, no action spent, no damage). The Lucky question comes first; the
// die's comes once it is answered.
func heldAttackOffer(enc *playv1.Encounter, h *heldOffer, attackerID, targetID, attackKey string, cs []playdb.Combatant) *playv1.RollAttackResponse {
	out := &playv1.RollAttackResponse{Encounter: enc}
	var d20 *playv1.DiceRoll
	switch {
	case h.Asks == askLucky:
		out.LuckyOffer = h.luckyOffer()
		d20 = out.LuckyOffer.GetD20()
	case h.State.Holds:
		out.InspirationOffer = h.attackDieOffer(dieHeldBy(*h.State.Die.Combatant, h.State.Die.Round), cs)
		d20 = out.InspirationOffer.GetD20()
	default: // the die ran out before a retry read the hold: the d20 alone
		d20 = heldD20Of(h.Hold)
	}
	out.Roll = &playv1.AttackRoll{AttackerId: attackerID, TargetId: targetID, AttackKey: attackKey, D20: d20}
	return out
}

// inspirationOfferView is the held roll of a combatant, for its player and the master. A roll
// that waits for its Lucky answer asks that first (luckyOfferView): one question at a time.
func inspirationOfferView(d *encounterData, c playdb.Combatant, v combatViewer) *playv1.InspirationOffer {
	die := dieHeldBy(c, d.enc.Round)
	if (!v.master && !v.owns(c)) || die == nil {
		return nil
	}
	for _, h := range d.holds {
		if h.CombatantID != c.ID || h.Round != d.enc.Round {
			continue
		}
		if luckyPending(h) {
			return nil
		}
		_, physical := rollOfHold(h)
		return offerOf(h, die, d.cs, physical)
	}
	return nil
}

// rollOfHold says how the d20 of a hold was given: typed from a real die or rolled in the app.
func rollOfHold(h playdb.RollHold) (face int32, physical bool) {
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(h.Request, &orig)
	return h.Face, typedRoll(&orig)
}

// bonusRollOf reads how the Bardic Inspiration die of AnswerBardicInspiration comes, when the player
// uses it: rolled in the app or typed from a real die.
func bonusRollOf(msg *playv1.AnswerBardicInspirationRequest) (rollInput, error) {
	if !msg.GetUse() {
		return rollInput{}, nil
	}
	var p rollPick
	switch roll := msg.GetRoll().(type) {
	case *playv1.AnswerBardicInspirationRequest_RollInApp:
		p.app = &roll.RollInApp
	case *playv1.AnswerBardicInspirationRequest_TypedFace:
		p.face = &roll.TypedFace
	}
	return rollOfAnswer(p, askDie)
}

// AnswerBardicInspiration implements playv1connect.ResourceServiceHandler: the player of a
// held roll uses the die on it or keeps it, and the attack is resolved.
func (s *Service) AnswerBardicInspiration(
	ctx context.Context,
	req *connect.Request[playv1.AnswerBardicInspirationRequest],
) (*connect.Response[playv1.AnswerBardicInspirationResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	holdID, err := parseCombatID(req.Msg.GetHoldId(), "held roll")
	if err != nil {
		return nil, err
	}
	in, err := bonusRollOf(req.Msg)
	if err != nil {
		return nil, err
	}
	out, err := s.answerHeld(ctx, m, answerRequest{
		ref: holdRef{Table: rollHolds, Campaign: m.CampaignID, Encounter: encID, ID: holdID}, key: key, question: askDie, yes: req.Msg.GetUse(), in: in,
	})
	if err != nil {
		return nil, err
	}
	attack, _ := out.(*playv1.RollAttackResponse)
	return connect.NewResponse(&playv1.AnswerBardicInspirationResponse{Attack: attack}), nil
}

// bonusDiceOf is the Bardic Inspiration die an attack roll used, as the roll shows it.
func bonusDiceOf(ev actionEvent) []*playv1.BonusDie {
	if ev.Res == nil || ev.Res.Kind != resBardicUse {
		return nil
	}
	return []*playv1.BonusDie{{SourceKey: bardicInspirationFt, Sides: ev.Res.Sides, Face: ev.Res.Face, Used: true}}
}

// heldAttackResponse is RollAttack's answer for a roll that waits: the combat as the caller sees it,
// the d20 and the question about the die.
func (s *Service) heldAttackResponse(ctx context.Context, m authz.Membership, res combatResult, h *heldOffer, req *playv1.RollAttackRequest) (*connect.Response[playv1.RollAttackResponse], error) {
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	cs, err := s.queries.ListCombatants(ctx, res.encounterID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	return connect.NewResponse(heldAttackOffer(out, h, req.GetAttackerId(), req.GetTargetId(), req.GetAttackKey(), cs)), nil
}

// typedRoll says the d20 of a request came from real dice: a typed face, or typed faces with
// no roll in the app (a held roll keeps the faces it rolled in the app too, with the app's roll).
func typedRoll(r *playv1.RollAttackRequest) bool {
	switch r.GetRoll().(type) {
	case *playv1.RollAttackRequest_D20Face:
		return true
	case nil:
		return len(r.GetD20Faces()) > 0
	}
	return false
}

// intsOf converts the faces of a request.
func intsOf(faces []int32) []int {
	out := make([]int, len(faces))
	for i, f := range faces {
		out[i] = int(f)
	}
	return out
}
