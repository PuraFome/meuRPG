package play

import (
	"context"

	"connectrpc.com/connect"

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

// countedAfterLucky is countedAfter for the faces a row of inspiration_holds kept.
func countedAfterLucky(hold playdb.InspirationHold, faces []int, mode combat.RollMode) int {
	return countedAfter(intsOf(hold.Faces), int(hold.Counted), faces, mode)
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
	out, err := s.answerHeld(ctx, m, answerRequest{
		ref: holdRef{Table: inspirationHolds, Campaign: m.CampaignID, ID: holdID}, key: key, question: askLucky, yes: req.Msg.GetReroll(), in: in,
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(outsideLuckyResultOf(out)), nil
}

// outsideLuckyResultOf is the response of the path that was run again, in the arm of the oneof it belongs to.
func outsideLuckyResultOf(out any) *playv1.AnswerOutsideLuckyResponse {
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
	return res
}

// outsideLuckyRollOf reads how the new d20 of AnswerOutsideLucky comes, as luckyRollOf does for
// AnswerLucky: none when the player keeps the 1; else rolled in the app or typed from a real die,
// 1 to 20.
func outsideLuckyRollOf(msg *playv1.AnswerOutsideLuckyRequest) (rollInput, error) {
	if !msg.GetReroll() {
		return rollInput{}, nil
	}
	var p rollPick
	switch roll := msg.GetRoll().(type) {
	case *playv1.AnswerOutsideLuckyRequest_RollInApp:
		p.app = &roll.RollInApp
	case *playv1.AnswerOutsideLuckyRequest_TypedFace:
		p.face = &roll.TypedFace
	}
	return rollOfAnswer(p, askLucky)
}
