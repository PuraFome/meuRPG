package play

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A hint won by a skill check (MR-038, RN-27, RN-18): a player rolls their character's
// skill against the DC the master set; a pass gives that player, and only them, the next
// hint they do not read yet. The DC never goes to a player: the answer carries the roll
// and whether it passed. A player tries once for each hint (the unique key of
// puzzle_hint_tries), so a failed try cannot be repeated until the master releases that
// hint (another player passing wins that player their own hint, not this one back); and each try is one revision of the run, so the app
// applies the read that follows.

// hintTryResult is what a try's transaction leaves for after the commit.
type hintTryResult struct {
	d        puzzleDef
	run      playdb.PuzzleRun
	try      playdb.PuzzleHintTry
	replayed bool
	sources  []*playv1.AdvantageSource
	// offer is the question about a Bardic Inspiration die the try waits on: the d20 was rolled and
	// kept, and nothing else of the result exists. bonus is the die the answer added. lucky is the
	// halfling's Lucky question the try waits on instead (it comes first).
	offer *playv1.OutsideInspirationOffer
	bonus *playv1.BonusDie
	lucky *playv1.LuckyOffer
}

// TryPuzzleHint implements playv1connect.PuzzleServiceHandler.
func (s *Service) TryPuzzleHint(
	ctx context.Context,
	req *connect.Request[playv1.TryPuzzleHintRequest],
) (*connect.Response[playv1.TryPuzzleHintResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RolePlayer)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	id, err := parsePuzzleID(req.Msg.GetPuzzleId())
	if err != nil {
		return nil, err
	}
	var in rollInput
	in.extraFaces = req.Msg.GetExtraDieFaces()
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.TryPuzzleHintRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.TryPuzzleHintRequest_D20Face:
		in.typed = int(roll.D20Face)
		if in.typed < 1 || in.typed > 20 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("d20_face must be 1 to 20"))
		}
	default:
		var err error
		if in.typedFaces, err = typedD20Faces(req.Msg.GetD20Faces()); err != nil {
			return nil, err
		}
	}

	res := &hintTryResult{}
	var names runNames
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		*res = hintTryResult{}
		var err error
		names, err = s.applyHintTry(ctx, tx, m, id, key, in, req.Msg, res)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "try for a puzzle hint", err)
	}
	if res.offer != nil || res.lucky != nil { // nothing was written: the question is the whole answer
		return connect.NewResponse(&playv1.TryPuzzleHintResponse{InspirationOffer: res.offer, LuckyOffer: res.lucky}), nil
	}
	if !res.replayed {
		s.publishPuzzleChanged(m.CampaignID, id) // the master reads the try; the revision moved
	}
	view, err := s.viewerOf(ctx, nil, m, res.d, res.run)
	if err != nil {
		return nil, s.dbError(ctx, "read the puzzle after a try", err)
	}
	out, err := playerRunProto(res.d, res.run, names, view)
	if err != nil {
		return nil, s.dbError(ctx, "read the puzzle after a try", err)
	}
	resp := &playv1.TryPuzzleHintResponse{
		Run:      out,
		Roll:     hintRollProto(res.try),
		Mode:     modeToProto[modeOfKey(res.try.RollMode)],
		Sources:  res.sources,
		Passed:   res.try.Passed,
		Replayed: res.replayed,
	}
	if res.bonus != nil {
		resp.BonusDice = []*playv1.BonusDie{res.bonus}
	}
	return connect.NewResponse(resp), nil
}

// hintTryByKey is the try a retry made before with the same key (found), or nothing. The key
// stands for that try, of that player, rolled that way (in the app, or with that typed die);
// a key spent on anything else, a move of the run included, is refused.
func hintTryByKey(ctx context.Context, q *playdb.Queries, m authz.Membership, runID, key string, in rollInput) (playdb.PuzzleHintTry, bool, error) {
	done, err := q.GetPuzzleHintTryByKey(ctx, playdb.GetPuzzleHintTryByKeyParams{RunID: runID, IdempotencyKey: key})
	switch {
	case err == nil:
		sameRoll := done.Physical != in.inApp && (in.inApp || sameTypedFaces(done, in))
		if done.UserID == nil || *done.UserID != m.UserID || !sameRoll {
			return playdb.PuzzleHintTry{}, false, errKeyReused()
		}
		return done, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return playdb.PuzzleHintTry{}, false, fmt.Errorf("find the try of this idempotency key: %w", err)
	}
	if _, err := q.GetPuzzleMoveByKey(ctx, playdb.GetPuzzleMoveByKeyParams{RunID: runID, IdempotencyKey: key}); err == nil {
		return playdb.PuzzleHintTry{}, false, errKeyReused()
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return playdb.PuzzleHintTry{}, false, fmt.Errorf("find the move of this idempotency key: %w", err)
	}
	return playdb.PuzzleHintTry{}, false, nil
}

// hintTryOpen refuses a try on a solved or stopped puzzle, or on one without a hint check.
func hintTryOpen(d puzzleDef, run playdb.PuzzleRun, now time.Time) error {
	switch {
	case run.SolvedAt != nil:
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_SOLVED, "the puzzle is solved")
	case stopOf(d, run, now) != playv1.PuzzleStopReason_PUZZLE_STOP_REASON_UNSPECIFIED:
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_STOPPED, "a limit stopped the puzzle")
	case d.hintCheck == nil:
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_HINT_CHECK, "this puzzle has no skill check for hints")
	}
	return nil
}

// nextHintToTry is the hint this player would read next, after the master's released ones
// and the ones they won; refused when none is left or they tried for it already.
func nextHintToTry(ctx context.Context, q *playdb.Queries, m authz.Membership, d puzzleDef, run playdb.PuzzleRun) (int, error) {
	won, err := q.GetPuzzleHintCursor(ctx, playdb.GetPuzzleHintCursorParams{RunID: run.ID, UserID: &m.UserID})
	if err != nil {
		return 0, fmt.Errorf("read the hints the player won: %w", err)
	}
	next := max(int(run.ReleasedHints), int(won))
	if next >= len(d.hints) {
		return 0, puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_MORE_HINTS, "there is no hint left for you")
	}
	tried, err := q.HasTriedPuzzleHint(ctx, playdb.HasTriedPuzzleHintParams{RunID: run.ID, UserID: &m.UserID, HintIndex: clamp32(next, 0, maxHints)})
	if err != nil {
		return 0, fmt.Errorf("read whether the player tried: %w", err)
	}
	if tried {
		return 0, puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_HINT_ALREADY_TRIED, "you tried for this hint already")
	}
	return next, nil
}

// applyHintTry is the try's transaction. The session's row is locked first and then
// the run's, as every change to a run does, so a try, a move and the master's release
// of the same hint take turns.
func (s *Service) applyHintTry(ctx context.Context, tx pgx.Tx, m authz.Membership, puzzleID, key string, in rollInput, raw *playv1.TryPuzzleHintRequest, res *hintTryResult) (runNames, error) {
	q := s.queriesIn(tx)
	session, err := lockOpenSession(ctx, q, m.CampaignID)
	if err != nil {
		return nil, err
	}
	d, _, err := s.shownRun(ctx, q, session, m.CampaignID, puzzleID)
	if err != nil {
		return nil, err
	}
	run, err := q.GetPuzzleRunForUpdate(ctx, playdb.GetPuzzleRunForUpdateParams{GameSessionID: session.ID, PuzzleID: puzzleID})
	if err != nil {
		return nil, fmt.Errorf("lock the puzzle's run: %w", err)
	}
	res.d, res.run = d, run

	// A retry of a try already made answers as the first call did, and rolls nothing.
	done, replayed, err := hintTryByKey(ctx, q, m, run.ID, key, in)
	if err != nil {
		return nil, err
	}
	if replayed {
		res.try, res.replayed = done, true
		return s.namesOfRuns(ctx, tx, m.CampaignID, run)
	}

	now := s.now()
	if err := hintTryOpen(d, run, now); err != nil {
		return nil, err
	}
	who, has, err := s.myCharacter(ctx, tx, m)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_CHARACTER, "you have no living character to roll for")
	}
	next, err := nextHintToTry(ctx, q, m, d, run)
	if err != nil {
		return nil, err
	}
	force, err := s.dice.ForcedDice(ctx, tx, m.CampaignID, m.UserID)
	if err != nil {
		return nil, err
	}
	if force.refuses(in.inApp) {
		return nil, puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
	}
	options, err := s.roster.SceneOptions(ctx, tx, m.CampaignID, who.ID, []string{d.hintCheck.GetSkillKey()})
	if err != nil {
		return nil, err
	}
	if len(options) != 1 || !options[0].Known {
		return nil, puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_SKILL, "your character has no numbers for this skill")
	}
	ability, isCheck := checkKey(d.hintCheck.GetSkillKey())
	cm, err := s.checkModeOf(ctx, tx, m.CampaignID, session.ID, who.ID, d.hintCheck.GetSkillKey(), ability, isCheck)
	if err != nil {
		return nil, err
	}
	bonus, err := s.withEffectDice(in, in.extraFaces, &cm, options[0].Bonus)
	if err != nil {
		return nil, err
	}
	// A character that holds a Bardic Inspiration die has the d20 kept until the player answers,
	// before anything is spent or written, and so has a halfling's d20 that shows a 1, for the
	// Lucky question; the answer brings it back (with the die added, or rolled again).
	step, err := s.heldGate(ctx, s.outsideGate(tx, q), rollSpec{
		Kind: heldHintTry, Key: key, Request: raw, Roller: heldRoller{CampaignID: m.CampaignID, CharacterID: who.ID, UserID: m.UserID},
		In: in, Modifier: bonus, Mode: cm.Mode, Reliable: options[0].ReliableTalent, Lucky: options[0].Lucky, MayAsk: true,
	})
	if err != nil {
		return nil, err
	}
	if step.Held != nil {
		res.offer, res.lucky = s.outsideOffers(ctx, tx, step.Held)
		return nil, nil
	}
	spendTx, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), svc: s, master: true})
	if err != nil {
		return nil, err
	}
	if err := s.spendOnceEffects(ctx, spendTx, cm.Rolled); err != nil {
		return nil, err
	}
	if err := s.spendCheckHelps(ctx, spendTx, cm.Helps); err != nil {
		return nil, err
	}
	d20 := step.Roll
	dieFace := 0
	if step.Bonus != nil {
		dieFace, res.bonus = int(step.Bonus.Face), step.Bonus // the die the player added after rolling: the total holds it
	}
	roll := d20
	face := d20.Face()
	passed := roll.Total >= int(d.hintCheck.GetDc())
	var granted *int32
	if passed {
		g := clamp32(next+1, 1, maxHints)
		granted = &g
	}
	var luckyAt *int32 // the d20 Lucky rolled again: the try keeps where, its faces the new one
	if step.Lucky != nil {
		luckyAt = new(clamp32(int(step.Lucky.Index), 0, 1))
	}
	try, err := q.InsertPuzzleHintTry(ctx, playdb.InsertPuzzleHintTryParams{
		RunID: run.ID, UserID: &m.UserID, CharacterID: &who.ID, IdempotencyKey: key, HintIndex: clamp32(next, 0, maxHints),
		Passed: passed, GrantedCount: granted, D20: clamp32(face, 1, 20), Modifier: clamp32(bonus+dieFace, -1000, 1000),
		Total: clamp32(roll.Total, -1000, 1000), Physical: roll.Physical, CreatedAt: now,
		D20B: clamp32(d20.otherFace(), 0, 20), Counted: clamp32(d20.Index, 0, 1), RollMode: modeKey(cm.Mode),
		LuckyIndex: luckyAt,
	})
	if err != nil {
		return nil, fmt.Errorf("record the try: %w", err)
	}
	// The run's revision moves with the try, so the app applies the read that follows.
	p := paramsOf(&run)
	p.Revision, p.UpdatedAt = run.Revision+1, now
	saved, err := q.SavePuzzleRun(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("save the puzzle's run: %w", err)
	}
	res.try, res.run = try, saved
	names, err := s.namerFor(ctx, tx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	res.sources = cm.shownCheck(names)
	return s.namesOfRuns(ctx, tx, m.CampaignID, saved)
}

// hintRollProto is the try's d20: the pair when it was rolled with advantage or
// disadvantage, with the one that counts marked, and the die Lucky rolled again.
func hintRollProto(t playdb.PuzzleHintTry) *playv1.DiceRoll {
	if t.D20B == 0 {
		out := treatedRoll(t.D20, t.Modifier, t.Total, t.Physical)
		out.Rerolled = hintLucky(t).proto()
		return out
	}
	out := diceRoll(2, 20, pairOf(t.D20, t.D20B, t.Counted), t.Modifier, t.Total, t.Physical)
	out.CountedIndex = t.Counted
	out.Rerolled = hintLucky(t).proto()
	return markTreated(out, t.Modifier, t.Total)
}

// hintFaces are the try's d20 in the order rolled: the pair, or the one.
func hintFaces(t playdb.PuzzleHintTry) []int32 {
	if t.D20B == 0 {
		return []int32{t.D20}
	}
	return pairOf(t.D20, t.D20B, t.Counted)
}

// hintLucky is the d20 the halfling's Lucky rolled again on the try (it only ever rolls a 1
// again, and the try keeps the new face): nil when none was.
func hintLucky(t playdb.PuzzleHintTry) *luckyReroll {
	faces := hintFaces(t)
	if t.LuckyIndex == nil || int(*t.LuckyIndex) >= len(faces) {
		return nil
	}
	return &luckyReroll{Index: *t.LuckyIndex, From: 1, To: faces[*t.LuckyIndex]}
}

// sameTypedFaces says whether the dice a call types are the dice of the try made before: the
// faces it rolled, with the 1 Lucky rolled again put back (the request typed the 1).
func sameTypedFaces(done playdb.PuzzleHintTry, in rollInput) bool {
	want := hintFaces(done)
	if r := hintLucky(done); r != nil {
		want[r.Index] = r.From
	}
	if len(in.typedFaces) == 0 {
		return len(want) == 1 && int(want[0]) == in.typed
	}
	if len(want) != len(in.typedFaces) {
		return false
	}
	for i, f := range in.typedFaces {
		if f != int(want[i]) {
			return false
		}
	}
	return true
}
