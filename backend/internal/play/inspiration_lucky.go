package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The halfling's Lucky (SRD 5.1, Halfling; combat_lucky.go has the rule) on the d20 rolls that are
// not a weapon attack: the scene check, the group check, the trap search and the puzzle hint try,
// and, in a combat, the spell attack against one target, the effect's saving throw and the
// concentration save. Each already keeps its d20 in an inspiration_holds row when the character
// holds a Bardic Inspiration die (outsideRoll, inspiration_outside.go); a lucky character's d20
// that shows a 1 is kept in the same row (lucky_index), die or not, and the path answers with the
// question "Tirou 1: rolar de novo?" instead of its result. AnswerOutsideLucky rolls the die again
// (in the app or typed, RN-18), or keeps the 1, and runs the same request again with the new d20:
// the roll is written, or, when the character also holds a die, stays held for the die's question
// on the new roll. Lucky comes first, the die after it; Reliable Talent counts the new d20 as it
// counts any d20 of the check. As with the die, only the player who rolls for their own character
// is asked: the master's rolls never wait here (a roll the master makes for a halfling waits for
// a decision, MR-014).

// outsideLuckyOf is the die a hold's Lucky answer rolled again; nil before the answer, when the
// player kept the 1, and for a hold that asked no Lucky question.
func outsideLuckyOf(h playdb.InspirationHold) *luckyReroll {
	if h.LuckyIndex == nil || h.LuckyTo == nil {
		return nil
	}
	return &luckyReroll{Index: *h.LuckyIndex, From: 1, To: *h.LuckyTo}
}

// outsideLuckyOfferOf is the Lucky question a hold asks: the d20 with the roll's bonus and the die
// that shows the 1. A roll that is not a weapon attack names no attack and no target.
func outsideLuckyOfferOf(h playdb.InspirationHold) *playv1.LuckyOffer {
	return &playv1.LuckyOffer{HoldId: h.ID, D20: outsideD20(h), LuckyIndex: derefInt32(h.LuckyIndex)}
}

// outsideAskingRefusal is the refusal of a roll while a held roll of the character waits: for its
// Lucky answer (which comes first), else for the Bardic Inspiration one.
func outsideAskingRefusal(h playdb.InspirationHold) error {
	if outsideLuckyPending(h) {
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING, "answer the Lucky question of the roll first")
	}
	return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING, "answer the Bardic Inspiration question of the roll first")
}

// answerOutsideLuckyHold writes the Lucky answer in the hold, inside the roll's transaction: the
// d20 after it (the die that showed the 1 replaced by the new face when the player rolled again,
// the die that counts by the roll's mode, and the total, with Reliable Talent counting the new
// face), the answer's key and the new face. A roll that stays held for the Bardic Inspiration
// question reads it from here, and so does the roll settled now. The question must still wait: an
// answer that got there first refuses this one. The same answer again (a retry) finds the hold as
// it left it.
func answerOutsideLuckyHold(ctx context.Context, q *playdb.Queries, hold playdb.InspirationHold, f *forcedOutside, modifier int, mode combat.RollMode, reliable bool) (playdb.InspirationHold, error) {
	if hold.LuckyAnswerKey != nil && *hold.LuckyAnswerKey == f.luckyKey {
		return hold, nil
	}
	if !outsideLuckyPending(hold) {
		return playdb.InspirationHold{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("the Lucky question was answered already"))
	}
	faces := intsOf(hold.Faces)
	var to *int32
	if r := f.lucky; r != nil && int(r.Index) < len(faces) {
		faces[r.Index] = int(r.To)
		to = new(r.To)
	}
	roll := d20Roll{Faces: faces, Index: countedAfterLucky(hold, faces, mode), Physical: hold.Physical}
	roll.Total = roll.Face() + modifier
	roll = reliableD20(roll, modifier, reliable)
	hold, err := q.AnswerInspirationHoldLucky(ctx, playdb.AnswerInspirationHoldLuckyParams{
		ID: hold.ID, Faces: faces32(faces), Counted: clamp32(roll.Index, 0, 1), Total: clamp32(roll.Total, -1000, 1000),
		LuckyAnswerKey: &f.luckyKey, LuckyTo: to,
	})
	if err != nil {
		return playdb.InspirationHold{}, fmt.Errorf("answer the Lucky question: %w", err)
	}
	return hold, nil
}

// countedAfterLucky is the die that counts once Lucky changed a face: 0 for a single d20; for a
// pair, the pick of the roll's mode (the higher with advantage, the lower with disadvantage), as
// the path works it out for the answer. Should the mode no longer roll two dice (a condition ended
// meanwhile), the pair's own mode is read from the hold: the die that counted was the higher, or
// the lower.
func countedAfterLucky(hold playdb.InspirationHold, faces []int, mode combat.RollMode) int {
	if len(faces) != pairDice || len(hold.Faces) != pairDice {
		return 0
	}
	if mode.Dice() != pairDice {
		c := clamp32(int(hold.Counted), 0, 1)
		mode = combat.ModeAdvantage
		if hold.Faces[c] < hold.Faces[1-c] {
			mode = combat.ModeDisadvantage
		}
	}
	return mode.Pick(faces)
}

// AnswerOutsideLucky implements playv1connect.ResourceServiceHandler: the player of a held roll
// that is not a weapon attack, whose d20 shows a 1, rolls that die again, or keeps the 1. The roll
// is then written by its own path, or, when the character holds a Bardic Inspiration die, held
// again for the die's question on the new roll.
func (s *Service) AnswerOutsideLucky(
	ctx context.Context,
	req *connect.Request[playv1.AnswerOutsideLuckyRequest],
) (*connect.Response[playv1.AnswerOutsideLuckyResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RolePlayer)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	holdID, err := parseCombatID(req.Msg.GetHoldId(), "held roll")
	if err != nil {
		return nil, err
	}
	in, err := outsideLuckyRollOf(req.Msg)
	if err != nil {
		return nil, err
	}
	hold, err := s.queries.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: m.CampaignID, ID: holdID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hold.UserID != m.UserID) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("held roll not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the held roll", err)
	}
	if hold.LuckyIndex == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll asks no Lucky question"))
	}
	forced := &forcedOutside{holdID: hold.ID, answerKey: key, luckyKey: key}
	switch {
	case hold.AnswerKey != nil:
		// Written already: the same Lucky answer again gives the roll as it was written (its path
		// finds its event by the roll's key, and rolls nothing).
		if *hold.AnswerKey != key && deref(hold.LuckyAnswerKey) != key {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll was answered already"))
		}
		forced = &forcedOutside{holdID: hold.ID, answerKey: *hold.AnswerKey}
	case hold.LuckyAnswerKey != nil:
		// Answered, and held for the Bardic Inspiration question: the same answer again reads the
		// hold as it is, and rolls nothing.
		if *hold.LuckyAnswerKey != key {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the Lucky question was answered already"))
		}
		forced.lucky = outsideLuckyOf(hold)
	case req.Msg.GetReroll():
		if err := s.mustRollThisWay(ctx, nil, m, in); err != nil {
			return nil, err
		}
		if forced.lucky, err = s.rollLuckyDie(ctx, *hold.LuckyIndex, in); err != nil {
			return nil, err
		}
	}
	out, err := s.rerunHold(ctx, hold, forced)
	if err != nil {
		return nil, err
	}
	res := &playv1.AnswerOutsideLuckyResponse{}
	switch o := out.(type) {
	case *playv1.RollSceneCheckResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_SceneCheck{SceneCheck: o}
	case *playv1.RollGroupCheckResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_GroupCheck{GroupCheck: o}
	case *playv1.SearchForTrapsResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_TrapSearch{TrapSearch: o}
	case *playv1.TryPuzzleHintResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_HintTry{HintTry: o}
	case *playv1.CastSpellResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_SpellCast{SpellCast: o}
	case *playv1.RollEffectSaveResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_EffectSave{EffectSave: o}
	case *playv1.ResolveConcentrationSaveResponse:
		res.Result = &playv1.AnswerOutsideLuckyResponse_ConcentrationSave{ConcentrationSave: o}
	}
	return connect.NewResponse(res), nil
}

// outsideLuckyRollOf reads how the new d20 of AnswerOutsideLucky comes, as luckyRollOf does for
// AnswerLucky: none when the player keeps the 1; else rolled in the app or typed from a real die,
// 1 to 20.
func outsideLuckyRollOf(msg *playv1.AnswerOutsideLuckyRequest) (rollInput, error) {
	var in rollInput
	if !msg.GetReroll() {
		return in, nil
	}
	switch roll := msg.GetRoll().(type) {
	case *playv1.AnswerOutsideLuckyRequest_RollInApp:
		if !roll.RollInApp {
			return in, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.AnswerOutsideLuckyRequest_TypedFace:
		if roll.TypedFace < 1 || roll.TypedFace > d20Sides {
			return in, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_face must be 1 to 20"))
		}
		in.typed = int(roll.TypedFace)
	default:
		return in, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_face to roll again"))
	}
	return in, nil
}
