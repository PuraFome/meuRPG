package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The bard's answer to the question of an NPC's check and the master's way out of it. The questions are asked in order: only the bard
// of the first ask still open may answer, anyone else is told the roll is not found; a bard whose ask closed is told it was answered
// already.

// errStaleNpcCheck says the held check belongs to a session that is over: nothing can answer it any more.
var errStaleNpcCheck = errors.New("the held check is from an earlier session")

// askStates are the states of the asks, for cutting.Current.
func askStates(asks []playdb.NpcCheckAsk) []cutting.State {
	out := make([]cutting.State, len(asks))
	for i, a := range asks {
		switch a.State {
		case askCut:
			out[i] = cutting.Cut
		case askPassed:
			out[i] = cutting.Passed
		case askClosed:
			out[i] = cutting.Closed
		}
	}
	return out
}

// npcCheckHoldOf finds a held check of the campaign, from this session: errHeldNotFound for anything else.
func npcCheckHoldOf(ctx context.Context, q *playdb.Queries, session playdb.GameSession, holdID string) (playdb.InspirationHold, []playdb.NpcCheckAsk, error) {
	hold, err := q.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: session.CampaignID, ID: holdID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hold.Kind != string(heldNpcCheck)) {
		return hold, nil, errHeldNotFound()
	}
	if err != nil {
		return hold, nil, fmt.Errorf("find the held check: %w", err)
	}
	asks, err := q.ListNpcCheckAsks(ctx, hold.ID)
	if err != nil {
		return hold, nil, fmt.Errorf("list the asks of the check: %w", err)
	}
	if hold.CreatedAt.Before(session.StartedAt) {
		return hold, asks, errStaleNpcCheck
	}
	return hold, asks, nil
}

// npcCheckEventOf is the payload of the event that held the check.
func npcCheckEventOf(ctx context.Context, q *playdb.Queries, session playdb.GameSession, hold playdb.InspirationHold) (npcCheckEvent, error) {
	row, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: session.ID, IdempotencyKey: &hold.IdempotencyKey})
	if err != nil {
		return npcCheckEvent{}, fmt.Errorf("find the event of the held check: %w", err)
	}
	var ev npcCheckEvent
	if err := json.Unmarshal(row.Payload, &ev); err != nil {
		return ev, fmt.Errorf("decode the held check: %w", err)
	}
	return ev, nil
}

// settleNpcCheck closes the check: the asks still open close with no answer, the hold takes the key and the resolved event is written
// under it. It answers the bards whose question changed.
func (s *Service) settleNpcCheck(ctx context.Context, tc *combatTx, hold playdb.InspirationHold, key string, actor *string, skipped bool) (npcCheckEvent, error) {
	ev, err := npcCheckEventOf(ctx, tc.q, tc.session, hold)
	if err != nil {
		return ev, err
	}
	if _, err := tc.q.CloseNpcCheckAsks(ctx, playdb.CloseNpcCheckAsksParams{HoldID: hold.ID, AnsweredAt: &tc.now}); err != nil {
		return ev, fmt.Errorf("close the asks of the check: %w", err)
	}
	asks, err := tc.q.ListNpcCheckAsks(ctx, hold.ID)
	if err != nil {
		return ev, fmt.Errorf("list the asks of the check: %w", err)
	}
	ev = ev.withAsks(asks, true)
	ev.Skipped = skipped
	if err := tc.q.AnswerInspirationHold(ctx, playdb.AnswerInspirationHoldParams{ID: hold.ID, AnswerKey: &key}); err != nil {
		return ev, fmt.Errorf("settle the held check: %w", err)
	}
	_, err = insertSceneEvent(ctx, tc, eventNpcCheckResolved, actor, &key, ev)
	return ev, err
}

// bardUser is the player of a bard's character.
func (s *Service) bardUser(ctx context.Context, tx pgx.Tx, campaignID, characterID string) string {
	cs, err := s.roster.CombatCharacters(ctx, tx, campaignID, []string{characterID})
	if err != nil || len(cs) != 1 {
		return ""
	}
	return cs[0].PlayerUserID
}

// forgetNpcCheck drops a held check that can never be answered (its session is over), in a transaction of its own.
func (s *Service) forgetNpcCheck(ctx context.Context, campaignID, holdID string) {
	err := s.txOrNone(ctx, campaignID, func(tx pgx.Tx, _ *hints, _ string) error {
		return forgetHold(ctx, s.queries.WithTx(tx), holdID)
	})
	if err != nil {
		s.logger.WarnContext(ctx, "play: cannot forget a held check whose session ended", "error", err)
	}
}

// answerNpcCheck is the cutting_words arm of AnswerHeldRoll.
func (s *Service) answerNpcCheck(ctx context.Context, m authz.Membership, req *playv1.AnswerHeldRollRequest, key, holdID string) (*playv1.AnswerHeldRollResponse, error) {
	ans := req.GetCuttingWords()
	var in rollInput
	if ans.GetCut() {
		switch r := ans.GetRoll().(type) {
		case *playv1.CuttingWordsAnswer_RollInApp:
			if !r.RollInApp {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
			}
			in.inApp = true
		case *playv1.CuttingWordsAnswer_TypedFace:
			in.typed = int(r.TypedFace)
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_face to cut"))
		}
	}
	var out *playv1.CuttingWordsAnswered
	err := s.txOpen(ctx, m.CampaignID, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		out = nil
		a := npcAnswer{s: s, m: m, tx: tx, q: s.queries.WithTx(tx), h: h, session: session, key: key, hash: idem.Hash(req), cut: ans.GetCut(), in: in}
		var err error
		out, err = a.run(ctx, holdID)
		return err
	})
	if errors.Is(err, errStaleNpcCheck) || (connect.CodeOf(err) == connect.CodeFailedPrecondition && heldClosed(err)) {
		s.forgetNpcCheck(ctx, m.CampaignID, holdID)
		return nil, errHeldNotFound()
	}
	if err != nil {
		return nil, err
	}
	return &playv1.AnswerHeldRollResponse{Result: &playv1.AnswerHeldRollResponse_CuttingWords{CuttingWords: out}}, nil
}

// npcAnswer is a bard's answer being made.
type npcAnswer struct {
	s       *Service
	m       authz.Membership
	tx      pgx.Tx
	q       *playdb.Queries
	h       *hints
	session playdb.GameSession
	key     string
	hash    *string
	cut     bool
	in      rollInput
}

// run is the transaction of the answer.
func (a *npcAnswer) run(ctx context.Context, holdID string) (*playv1.CuttingWordsAnswered, error) {
	hold, asks, err := npcCheckHoldOf(ctx, a.q, a.session, holdID)
	if err != nil {
		return nil, err
	}
	who, has, err := a.s.myCharacter(ctx, a.tx, a.m)
	if err != nil {
		return nil, err
	}
	mine := slices.IndexFunc(asks, func(k playdb.NpcCheckAsk) bool { return has && k.BardCharacterID == who.ID })
	if mine < 0 {
		return nil, errHeldNotFound() // not a bard of this check: it does not exist for them
	}
	if k := asks[mine]; k.AnswerKey != nil && *k.AnswerKey == a.key {
		return a.answered(ctx, k)
	}
	if other, err := a.q.GetNpcCheckAskByAnswerKey(ctx, playdb.GetNpcCheckAskByAnswerKeyParams{CampaignID: a.m.CampaignID, AnswerKey: &a.key}); err == nil && other.ID != "" {
		return nil, errKeyReused()
	}
	if asks[mine].State != askAsked || hold.AnswerKey != nil {
		return nil, errAnsweredAlready()
	}
	if cutting.Current(askStates(asks)) != mine {
		return nil, errHeldNotFound() // their turn has not come: they are not told there is a check
	}
	if err := requireNoActiveCombat(ctx, a.q, a.session.ID); err != nil {
		return nil, err
	}
	return a.write(ctx, hold, asks, mine, who.ID)
}

// write spends what the answer spends, stores it and settles the check when it was the last question.
func (a *npcAnswer) write(ctx context.Context, hold playdb.InspirationHold, asks []playdb.NpcCheckAsk, mine int, bardID string) (*playv1.CuttingWordsAnswered, error) {
	s, ask := a.s, asks[mine]
	state, face, physical := askPassed, 0, false
	vit, err := s.vitals.GetVitalsTx(ctx, a.tx, a.m.CampaignID, bardID)
	if err != nil {
		return nil, err
	}
	tc, err := s.openTx(ctx, combatTx{tx: a.tx, q: a.q, session: a.session, now: s.now(), characterID: &bardID, hash: a.hash})
	if err != nil {
		return nil, err
	}
	if a.cut {
		if face, physical, vit, err = a.spendDie(ctx, tc, bardID, int(ask.Sides), vit); err != nil {
			return nil, err
		}
		state = askCut
	}
	var dieFace *int32
	if a.cut {
		f := clamp32(face, 1, 20)
		dieFace = &f
	}
	if _, err := a.q.AnswerNpcCheckAsk(ctx, playdb.AnswerNpcCheckAskParams{
		ID: ask.ID, State: state, DieFace: dieFace, Physical: physical, AnswerKey: &a.key, AnsweredAt: &tc.now,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errAnsweredAlready()
		}
		return nil, fmt.Errorf("answer the ask: %w", err)
	}
	told := []string{a.m.UserID}
	if mine+1 < len(asks) {
		told = append(told, s.bardUser(ctx, a.tx, a.m.CampaignID, asks[mine+1].BardCharacterID))
	} else if _, err := s.settleNpcCheck(ctx, tc, hold, a.key, &a.m.UserID, false); err != nil {
		return nil, err
	}
	tellNpcCheck(a.h, a.m.CampaignID, told...)
	out := &playv1.CuttingWordsAnswered{Cut: a.cut, UsesLeft: bardUsesLeft(vit)}
	if a.cut {
		out.Die = diceRoll(1, ask.Sides, []int32{clamp32(face, 1, 20)}, 0, clamp32(face, 1, 20), physical)
	}
	return out, nil
}

// spendDie rolls the bard's die the way the campaign has the bard roll (RN-18) and spends the use of Bardic Inspiration.
func (a *npcAnswer) spendDie(ctx context.Context, tc *combatTx, bardID string, sides int, vit *playv1.CharacterVitals) (int, bool, *playv1.CharacterVitals, error) {
	if err := a.s.mustRollThisWay(ctx, a.tx, a.m, a.in); err != nil {
		return 0, false, nil, err
	}
	face, res, err := a.s.dieRoll(a.in, sides)
	if err != nil {
		return 0, false, nil, err
	}
	if bardUsesLeft(vit) <= 0 {
		return 0, false, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT, "no use of Bardic Inspiration is left")
	}
	after, err := a.s.spendResource(ctx, tc, bardID, rules.BardicInspirationKey, 1)
	return face, res.Physical, after, err
}

// bardUsesLeft are the uses of Bardic Inspiration the vitals show left.
func bardUsesLeft(v *playv1.CharacterVitals) int32 {
	if r := resourceOf(v, rules.BardicInspirationKey); r != nil {
		return max(r.GetTotal()-r.GetUsed(), 0)
	}
	return 0
}

// answered is the answer of a key the bard already used: what it said, and nothing written.
func (a *npcAnswer) answered(ctx context.Context, k playdb.NpcCheckAsk) (*playv1.CuttingWordsAnswered, error) {
	vit, err := a.s.vitals.GetVitalsTx(ctx, a.tx, a.m.CampaignID, k.BardCharacterID)
	if err != nil {
		return nil, err
	}
	a.h.replay()
	tellNpcCheck(a.h, a.m.CampaignID, a.m.UserID)
	out := &playv1.CuttingWordsAnswered{Cut: k.State == askCut, UsesLeft: bardUsesLeft(vit)}
	if out.Cut && k.DieFace != nil {
		out.Die = diceRoll(1, k.Sides, []int32{*k.DieFace}, 0, *k.DieFace, k.Physical)
	}
	return out, nil
}

// SkipNpcCheckAsks implements playv1connect.PlayServiceHandler.
func (s *Service) SkipNpcCheckAsks(ctx context.Context, req *connect.Request[playv1.SkipNpcCheckAsksRequest]) (*connect.Response[playv1.SkipNpcCheckAsksResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
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
	hash := idem.Hash(req.Msg)
	var read *playv1.NpcCheckRead
	err = s.txOpen(ctx, m.CampaignID, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		var err error
		read, err = s.skipAsks(ctx, tx, h, session, m, key, hash, holdID)
		return err
	})
	if errors.Is(err, errStaleNpcCheck) {
		s.forgetNpcCheck(ctx, m.CampaignID, holdID)
		return nil, errHeldNotFound()
	}
	if err != nil {
		return nil, s.dbError(ctx, "go on without the bard", err)
	}
	return connect.NewResponse(&playv1.SkipNpcCheckAsksResponse{Check: read}), nil
}

// skipAsks is the transaction of SkipNpcCheckAsks: a check already settled is answered as it is, and nothing is written.
func (s *Service) skipAsks(ctx context.Context, tx pgx.Tx, h *hints, session playdb.GameSession, m authz.Membership, key string, hash *string, holdID string) (*playv1.NpcCheckRead, error) {
	q := s.queries.WithTx(tx)
	done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: session.ID, IdempotencyKey: &key})
	if err == nil {
		var ev npcCheckEvent
		if done.Kind != eventNpcCheckResolved || hashDiffers(done.IdempotencyHash, hash) || json.Unmarshal(done.Payload, &ev) != nil || ev.HoldID != holdID {
			return nil, errKeyReused()
		}
		h.replay()
		tellNpcCheck(h, m.CampaignID)
		return s.npcCheckLive(ctx, q, m.CampaignID, ev)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find the event of this idempotency key: %w", err)
	}
	hold, asks, err := npcCheckHoldOf(ctx, q, session, holdID)
	if err != nil {
		return nil, err
	}
	ev, err := npcCheckEventOf(ctx, q, session, hold)
	if err != nil {
		return nil, err
	}
	if hold.AnswerKey != nil {
		return s.npcCheckLive(ctx, q, m.CampaignID, ev) // settled already: it reads as it settled
	}
	told := []string{}
	if cur := cutting.Current(askStates(asks)); cur >= 0 {
		told = append(told, s.bardUser(ctx, tx, m.CampaignID, asks[cur].BardCharacterID))
	}
	tc, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &hold.CharacterID, hash: hash})
	if err != nil {
		return nil, err
	}
	settled, err := s.settleNpcCheck(ctx, tc, hold, key, &m.UserID, true)
	if err != nil {
		return nil, err
	}
	tellNpcCheck(h, m.CampaignID, told...)
	return settled.read(s, true), nil
}
