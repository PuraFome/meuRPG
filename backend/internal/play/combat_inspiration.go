package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

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
	// faces are both d20 of a roll with advantage or disadvantage, in the order rolled (empty
	// for a normal roll): the roll that was held is the one the answer settles.
	faces []int
	// toHit is the attack bonus the roll was made with (a Cutting Words die already off it).
	toHit    int
	physical bool
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

// inspirationHold is an attack roll that waits for the answer about a die.
type inspirationHold struct {
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
func (s *Service) rollOrHold(ctx context.Context, c *combatTx, attacker playdb.Combatant, raw *playv1.RollAttackRequest, key string, in rollInput, toHit int, choice modeChoice) (roll d20Roll, bonus *bonusUse, held *inspirationHold, err error) {
	if f := forcedOf(ctx); f != nil {
		if err := c.q.AnswerRollHold(ctx, playdb.AnswerRollHoldParams{ID: f.holdID, AnswerKey: &f.answerKey}); err != nil {
			return d20Roll{}, nil, nil, fmt.Errorf("answer the held roll: %w", err)
		}
		if f.bonus != nil {
			if err := c.q.SetCombatantInspirationDie(ctx, playdb.SetCombatantInspirationDieParams{ID: attacker.ID}); err != nil {
				return d20Roll{}, nil, nil, fmt.Errorf("use the die: %w", err)
			}
		}
		faces := f.faces
		if len(faces) == 0 {
			faces = []int{f.face}
		}
		idx := choice.Mode.Pick(faces)
		total := faces[idx] + toHit
		if f.bonus != nil {
			total += int(f.bonus.Face)
		}
		return d20Roll{Faces: faces, Index: idx, Total: total, Physical: f.physical}, f.bonus, nil, nil
	}
	if !holdsDie(attacker, c.enc.Round) {
		roll, err = s.d20With(in, toHit, choice.Mode)
		return roll, nil, nil, err
	}
	// The same request again: the hold it made, with the d20 it rolled.
	if hold, err := c.q.GetRollHoldByKey(ctx, playdb.GetRollHoldByKeyParams{EncounterID: c.enc.ID, IdempotencyKey: key}); err == nil {
		return d20Roll{}, nil, s.heldOf(attacker, hold, toHit, in), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return d20Roll{}, nil, nil, fmt.Errorf("find the held roll: %w", err)
	}
	// One question at a time: another roll waits for its answer. A hold of an earlier round
	// is forgotten.
	open, err := c.q.GetOpenRollHoldOf(ctx, playdb.GetOpenRollHoldOfParams{EncounterID: c.enc.ID, CombatantID: attacker.ID})
	switch {
	case err == nil && open.Round == c.enc.Round:
		return d20Roll{}, nil, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING, "answer the Bardic Inspiration question of the roll first")
	case err == nil:
		if err := c.q.DeleteRollHold(ctx, open.ID); err != nil {
			return d20Roll{}, nil, nil, fmt.Errorf("forget the old held roll: %w", err)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return d20Roll{}, nil, nil, fmt.Errorf("find the open held roll: %w", err)
	}
	if roll, err = s.d20With(in, toHit, choice.Mode); err != nil {
		return d20Roll{}, nil, nil, err
	}
	// A roll a reaction window held first already has an event under its key (the one that stood
	// for the held attack): the question gets a key of its own, and the answer resolves under its own.
	holdKey := key
	if c.replay != nil {
		holdKey = key + windowHoldSuffix
	}
	// The request is kept with what the roll was made with: the mode, its reason and both
	// d20 when there are two, so that the answer settles this roll and not another.
	pinned, _ := proto.Clone(raw).(*playv1.RollAttackRequest)
	pinned.RollMode, pinned.ModeReason = modeToProto[choice.Mode], choice.Reason
	pinned.D20Faces = nil
	if len(roll.Faces) > 1 {
		pinned.D20Faces = faces32(roll.Faces)
	}
	body, err := proto.Marshal(pinned)
	if err != nil {
		return d20Roll{}, nil, nil, fmt.Errorf("keep the request: %w", err)
	}
	hold, err := c.q.InsertRollHold(ctx, playdb.InsertRollHoldParams{
		EncounterID: c.enc.ID, CombatantID: attacker.ID, IdempotencyKey: holdKey, Request: body,
		Face: clamp32(roll.Face(), 1, d20Sides), Modifier: clamp32(toHit, math.MinInt32, math.MaxInt32), Round: c.enc.Round, CreatedAt: c.now,
	})
	if err != nil {
		return d20Roll{}, nil, nil, fmt.Errorf("hold the roll: %w", err)
	}
	// The question is part of the combat the master and the player read: its revision moves.
	if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
		return d20Roll{}, nil, nil, fmt.Errorf("touch the encounter: %w", err)
	}
	return roll, nil, s.heldOf(attacker, hold, toHit, in), nil
}

// refuseWhileAsking refuses a roll while an earlier roll of the combatant waits for its Bardic
// Inspiration answer, before a Cutting Words window can hold the new one. The request that
// made the question (a retry) and the answer itself pass.
func (s *Service) refuseWhileAsking(ctx context.Context, c *combatTx, attacker playdb.Combatant, key string) error {
	if forcedOf(ctx) != nil || c.replay != nil || !holdsDie(attacker, c.enc.Round) {
		return nil
	}
	open, err := c.q.GetOpenRollHoldOf(ctx, playdb.GetOpenRollHoldOfParams{EncounterID: c.enc.ID, CombatantID: attacker.ID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("find the open held roll: %w", err)
	case open.Round == c.enc.Round && open.IdempotencyKey != key:
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING, "answer the Bardic Inspiration question of the roll first")
	}
	return nil
}

// windowHoldSuffix marks the key of a held roll that a reaction window held before it was rolled.
const windowHoldSuffix = "#window"

// d20Sides is the d20 the held roll was made with.
const d20Sides = 20

// heldOf describes a hold for the answer to the roll: the die the attacker holds and the d20.
func (s *Service) heldOf(attacker playdb.Combatant, hold playdb.RollHold, toHit int, in rollInput) *inspirationHold {
	die := &playv1.InspirationDie{Sides: *attacker.InspirationSides, ExpiresAtRound: *attacker.InspirationExpiresRound, FromCombatantId: deref(attacker.InspirationFrom)}
	return &inspirationHold{hold: hold, die: die, face: int(hold.Face), toHit: toHit, physical: !in.inApp}
}

// offerOf is the question a hold asks the player: the d20 with the attack's bonus, the attack, its
// target and the die the attacker holds.
func offerOf(hold playdb.RollHold, die *playv1.InspirationDie, cs []playdb.Combatant, physical bool) *playv1.InspirationOffer {
	if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == die.GetFromCombatantId() }); i >= 0 {
		die.FromLabel = cs[i].Label
	}
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(hold.Request, &orig) // a request this module wrote
	roll := diceRoll(1, 20, []int32{hold.Face}, hold.Modifier, hold.Face+hold.Modifier, physical)
	if faces := orig.GetD20Faces(); len(faces) == pairDice { // advantage or disadvantage: both dice, the one that counts marked
		mode, _ := modeFromProto(orig.GetRollMode())
		counted := mode.Pick(intsOf(faces))
		roll = diceRoll(pairDice, 20, faces, hold.Modifier, hold.Face+hold.Modifier, physical)
		roll.CountedIndex = clamp32(counted, 0, 1)
	}
	return &playv1.InspirationOffer{
		HoldId: hold.ID, Die: die, AttackKey: orig.GetAttackKey(), TargetId: orig.GetTargetId(), D20: roll,
	}
}

// heldResponse is what RollAttack answers for a held roll: the d20 and the offer, the
// attack not resolved (no outcome, no action spent, no damage).
func heldResponse(enc *playv1.Encounter, h *inspirationHold, attackerID, targetID, attackKey string, cs []playdb.Combatant) *playv1.RollAttackResponse {
	offer := offerOf(h.hold, h.die, cs, h.physical)
	return &playv1.RollAttackResponse{
		Encounter:        enc,
		Roll:             &playv1.AttackRoll{AttackerId: attackerID, TargetId: targetID, AttackKey: attackKey, D20: offer.D20},
		InspirationOffer: offer,
	}
}

// inspirationOfferView is the held roll of a combatant, for its player and the master.
func inspirationOfferView(d *encounterData, c playdb.Combatant, v combatViewer) *playv1.InspirationOffer {
	if (!v.master && !v.owns(c)) || !holdsDie(c, d.enc.Round) {
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
	return h.Face, typedRoll(&orig)
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
	if strings.HasSuffix(hold.IdempotencyKey, windowHoldSuffix) { // the original key belongs to the event that stood for the held attack
		orig.IdempotencyKey = key
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
	forced := &forcedRoll{holdID: hold.ID, answerKey: key, face: int(hold.Face), toHit: int(hold.Modifier), faces: intsOf(orig.GetD20Faces())}
	// The d20 was typed when the roll was: the attack knows it from the request.
	forced.physical = typedRoll(&orig)
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
func (s *Service) heldAnswer(ctx context.Context, m authz.Membership, res combatResult, h *inspirationHold, req *playv1.RollAttackRequest) (*connect.Response[playv1.RollAttackResponse], error) {
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
