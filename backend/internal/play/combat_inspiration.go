package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Using a Bardic Inspiration die (SRD 5.1, Bard 1): the creature "can wait until after it
// rolls the d20 before deciding to use the Bardic Inspiration die, but must decide before
// the GM says whether the roll succeeds or fails". A character that holds a die and rolls an
// attack therefore does not get the result with its roll: RollAttack rolls (or takes) the d20
// and keeps it with the request in a roll_holds row, writes nothing else, and answers with
// the offer. AnswerBardicInspiration then asks RollAttack again for the same request with the
// d20 already rolled, and with the die rolled and added when the player chose to use it.
// Only attack rolls of a combatant are held: the saving throws the app rolls for a spell
// and the ability checks of a scene do not wait for the die.

// forcedRoll is the d20 of a held roll, carried from the answer to the RollAttack it calls.
type forcedRoll struct {
	holdID, answerKey string
	face              int
	physical          bool
	// bonus is the die the player rolled and added, nil when they kept it.
	bonus *bonusUse
}

// bonusUse is a Bardic Inspiration die rolled and added to an attack roll, with the die as it
// was held, for the undo.
type bonusUse struct {
	Sides, Face  int32
	From         string
	ExpiresRound int32
}

type forcedKey struct{}

func withForced(ctx context.Context, f *forcedRoll) context.Context {
	return context.WithValue(ctx, forcedKey{}, f)
}

func forcedOf(ctx context.Context) *forcedRoll {
	f, _ := ctx.Value(forcedKey{}).(*forcedRoll)
	return f
}

// heldRoll is an attack roll that waits for the answer about a die.
type heldRoll struct {
	hold     playdb.RollHold
	die      *playv1.InspirationDie
	face     int
	toHit    int
	physical bool
}

// rollOrHold is RollAttack's d20: rolled (or typed), as always, unless the attacker holds a
// Bardic Inspiration die, when the roll is kept in a hold and returned as held; or the d20 of
// an answered hold (forcedOf), with the die added when the player used it. A retry of the
// request that made a hold finds it, and rolls nothing again.
func (s *Service) rollOrHold(ctx context.Context, c *combatTx, attacker playdb.Combatant, raw *playv1.RollAttackRequest, key string, in rollInput, toHit int) (face int, roll dice.Result, bonus *bonusUse, held *heldRoll, err error) {
	if f := forcedOf(ctx); f != nil {
		if err := c.q.AnswerRollHold(ctx, playdb.AnswerRollHoldParams{ID: f.holdID, AnswerKey: &f.answerKey}); err != nil {
			return 0, dice.Result{}, nil, nil, fmt.Errorf("answer the held roll: %w", err)
		}
		if f.bonus != nil {
			if err := c.q.SetCombatantInspirationDie(ctx, playdb.SetCombatantInspirationDieParams{ID: attacker.ID}); err != nil {
				return 0, dice.Result{}, nil, nil, fmt.Errorf("use the die: %w", err)
			}
		}
		total := f.face + toHit
		if f.bonus != nil {
			total += int(f.bonus.Face)
		}
		return f.face, dice.Result{Expr: dice.Expr{Count: 1, Sides: 20, Modifier: toHit}, Faces: []int{f.face}, Modifier: toHit, Total: total, Physical: f.physical}, f.bonus, nil, nil
	}
	if !holdsDie(attacker, c.enc.Round) {
		face, roll, err = s.d20(in, toHit)
		return face, roll, nil, nil, err
	}
	// The same request again: the hold it made, with the d20 it rolled.
	if hold, err := c.q.GetRollHoldByKey(ctx, playdb.GetRollHoldByKeyParams{EncounterID: c.enc.ID, IdempotencyKey: key}); err == nil {
		return 0, dice.Result{}, nil, s.heldOf(c, attacker, hold, toHit, in), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, dice.Result{}, nil, nil, fmt.Errorf("find the held roll: %w", err)
	}
	// One question at a time: another roll waits for its answer. A hold of an earlier round
	// is forgotten.
	open, err := c.q.GetOpenRollHoldOf(ctx, playdb.GetOpenRollHoldOfParams{EncounterID: c.enc.ID, CombatantID: attacker.ID})
	switch {
	case err == nil && open.Round == c.enc.Round:
		return 0, dice.Result{}, nil, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING, "answer the Bardic Inspiration question of the roll first")
	case err == nil:
		if err := c.q.DeleteRollHold(ctx, open.ID); err != nil {
			return 0, dice.Result{}, nil, nil, fmt.Errorf("forget the old held roll: %w", err)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return 0, dice.Result{}, nil, nil, fmt.Errorf("find the open held roll: %w", err)
	}
	if face, roll, err = s.d20(in, toHit); err != nil {
		return 0, dice.Result{}, nil, nil, err
	}
	body, err := proto.Marshal(raw)
	if err != nil {
		return 0, dice.Result{}, nil, nil, fmt.Errorf("keep the request: %w", err)
	}
	hold, err := c.q.InsertRollHold(ctx, playdb.InsertRollHoldParams{
		EncounterID: c.enc.ID, CombatantID: attacker.ID, IdempotencyKey: key, Request: body,
		Face: clamp32(face, 1, 20), Modifier: clamp32(toHit, math.MinInt32, math.MaxInt32), Round: c.enc.Round, CreatedAt: c.now,
	})
	if err != nil {
		return 0, dice.Result{}, nil, nil, fmt.Errorf("hold the roll: %w", err)
	}
	// The question is part of the combat the master and the player read: its revision moves.
	if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
		return 0, dice.Result{}, nil, nil, fmt.Errorf("touch the encounter: %w", err)
	}
	return face, roll, nil, s.heldOf(c, attacker, hold, toHit, in), nil
}

// heldOf describes a hold for the answer to the roll: the die the attacker holds and the d20.
func (s *Service) heldOf(c *combatTx, attacker playdb.Combatant, hold playdb.RollHold, toHit int, in rollInput) *heldRoll {
	die := &playv1.InspirationDie{Sides: *attacker.InspirationSides, ExpiresAtRound: *attacker.InspirationExpiresRound, FromCombatantId: deref(attacker.InspirationFrom)}
	return &heldRoll{hold: hold, die: die, face: int(hold.Face), toHit: toHit, physical: !in.inApp}
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
		HoldId: hold.ID, Die: die, AttackKey: orig.GetAttackKey(), TargetId: orig.GetTargetId(),
		D20: diceRoll(1, 20, []int32{hold.Face}, hold.Modifier, hold.Face+hold.Modifier, physical),
	}
}

// heldResponse is what RollAttack answers for a held roll: the d20 and the offer, the
// attack not resolved (no outcome, no action spent, no damage).
func heldResponse(enc *playv1.Encounter, h *heldRoll, attackerID, targetID, attackKey string, cs []playdb.Combatant) *playv1.RollAttackResponse {
	offer := offerOf(h.hold, h.die, cs, h.physical)
	return &playv1.RollAttackResponse{
		Encounter:        enc,
		Roll:             &playv1.AttackRoll{AttackerId: attackerID, TargetId: targetID, AttackKey: attackKey, D20: offer.D20},
		InspirationOffer: offer,
	}
}

// inspirationOfferView is the held roll of a combatant, for its player and the master.
func inspirationOfferView(d *encounterData, c playdb.Combatant, v combatViewer) *playv1.InspirationOffer {
	if !(v.master || v.owns(c)) || !holdsDie(c, d.enc.Round) {
		return nil
	}
	for _, h := range d.holds {
		if h.CombatantID != c.ID || h.Round != d.enc.Round {
			continue
		}
		die := &playv1.InspirationDie{Sides: *c.InspirationSides, ExpiresAtRound: *c.InspirationExpiresRound, FromCombatantId: deref(c.InspirationFrom)}
		_, physical := rollOfHold(h)
		return offerOf(h, die, d.cs, physical)
	}
	return nil
}

// rollOfHold says how the d20 of a hold was given: typed from a real die or rolled in the app.
func rollOfHold(h playdb.RollHold) (face int32, physical bool) {
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(h.Request, &orig)
	_, physical = orig.GetRoll().(*playv1.RollAttackRequest_D20Face)
	return h.Face, physical
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
	var in rollInput
	if req.Msg.GetUse() {
		switch roll := req.Msg.GetRoll().(type) {
		case *playv1.AnswerBardicInspirationRequest_RollInApp:
			if !roll.RollInApp {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
			}
			in.inApp = true
		case *playv1.AnswerBardicInspirationRequest_TypedFace:
			in.typed = int(roll.TypedFace)
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_face to use the die"))
		}
	}
	hold, err := s.queries.GetRollHold(ctx, playdb.GetRollHoldParams{EncounterID: encID, ID: holdID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("held roll not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the held roll", err)
	}
	var orig playv1.RollAttackRequest
	if err := proto.Unmarshal(hold.Request, &orig); err != nil || orig.GetCampaignId() != m.CampaignID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("held roll not found"))
	}
	if hold.AnswerKey != nil {
		// Answered already: the same answer again gives the attack as it was resolved.
		if *hold.AnswerKey != key {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll was answered already"))
		}
		out, err := s.RollAttack(withForced(ctx, nil), connect.NewRequest(&orig))
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&playv1.AnswerBardicInspirationResponse{Attack: out.Msg}), nil
	}
	forced := &forcedRoll{holdID: hold.ID, answerKey: key, face: int(hold.Face)}
	// The d20 was typed when the roll was: the attack knows it from the request.
	_, forced.physical = orig.GetRoll().(*playv1.RollAttackRequest_D20Face)
	if req.Msg.GetUse() {
		if forced.bonus, err = s.rollBonusDie(ctx, m, hold, in); err != nil {
			return nil, err
		}
	}
	// The attack as it was asked, with the d20 it rolled: it resolves against the combat as
	// it is now, and spends the action and the die in its own transaction, under the key
	// the roll came with.
	out, err := s.RollAttack(withForced(ctx, forced), connect.NewRequest(&orig))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AnswerBardicInspirationResponse{Attack: out.Msg}), nil
}

// rollBonusDie rolls the Bardic Inspiration die of the held roll's attacker (or takes the face
// the player typed from a real die): the die the combatant holds now.
func (s *Service) rollBonusDie(ctx context.Context, m authz.Membership, hold playdb.RollHold, in rollInput) (*bonusUse, error) {
	cs, err := s.queries.ListCombatants(ctx, hold.EncounterID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	i := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == hold.CombatantID })
	if i < 0 || !holdsDie(cs[i], hold.Round) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the combatant holds no die any more"))
	}
	who := cs[i]
	v := viewerOf(m)
	if err := v.mayAct(who); err != nil {
		return nil, err
	}
	if m.Role != authz.RoleMaster {
		if err := s.mustRollThisWay(ctx, nil, m, in); err != nil {
			return nil, err
		}
	}
	sides := int(*who.InspirationSides)
	face := in.typed
	if in.inApp {
		rolled, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: sides})
		if err != nil {
			return nil, s.dbError(ctx, "roll the die", err)
		}
		face = rolled.Faces[0]
	} else if face < 1 || face > sides {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_face must be a face of the die"))
	}
	return &bonusUse{
		Sides: *who.InspirationSides, Face: clamp32(face, 1, sides), From: deref(who.InspirationFrom), ExpiresRound: *who.InspirationExpiresRound,
	}, nil
}

// bonusDiceOf is the Bardic Inspiration die an attack roll used, as the roll shows it.
func bonusDiceOf(ev actionEvent) []*playv1.BonusDie {
	if ev.Res == nil || ev.Res.Kind != resBardicUse {
		return nil
	}
	return []*playv1.BonusDie{{SourceKey: bardicInspirationFt, Sides: ev.Res.Sides, Face: ev.Res.Face, Used: true}}
}

// heldAnswer is RollAttack's answer for a roll that waits: the combat as the caller sees it,
// the d20 and the question about the die.
func (s *Service) heldAnswer(ctx context.Context, m authz.Membership, res combatResult, h *heldRoll, req *playv1.RollAttackRequest) (*connect.Response[playv1.RollAttackResponse], error) {
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	cs, err := s.queries.ListCombatants(ctx, res.encounterID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	return connect.NewResponse(heldResponse(out, h, req.GetAttackerId(), req.GetTargetId(), req.GetAttackKey(), cs)), nil
}
