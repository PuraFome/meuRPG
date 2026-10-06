package play

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/puzzle"
)

// How a puzzle and a run are shown. There are exactly two audiences, and each has
// its own builder, so that nothing the master alone may know can slip into what a
// player gets (RN-10, RN-27): puzzleProto and masterRunProto are the master's;
// playerRunProto is the players'. The player's builder reads only the config
// without its secrets, the state, the clue, the hints that player may read, the last
// move without what was typed, and what is the player's own (playerView): their
// attempts, their part of the clue, whether they found the cipher's key.

// puzzleProto is a puzzle as the master keeps it, with its answer.
func puzzleProto(d puzzleDef, shown bool) (*playv1.Puzzle, error) {
	least, err := d.kind.minimum(d, d.start)
	if err != nil {
		return nil, err
	}
	out := &playv1.Puzzle{
		Id: d.row.ID, CampaignId: d.row.CampaignID, Kind: d.kind.id(), Name: d.row.Name,
		Config: d.config, Start: d.start, Minimum: least, Symbols: d.kind.symbols(d.config),
		Clue: d.row.Clue, Hints: d.hints, OnSolve: d.onSolve,
		HintCheck: d.hintCheck, Parts: d.parts, OnWrong: d.onWrong,
		Archived: d.row.ArchivedAt != nil, Shown: shown,
		CreatedAt: timestamppb.New(d.row.CreatedAt), UpdatedAt: timestamppb.New(d.row.UpdatedAt),
	}
	if d.solution.GetKind() != nil {
		out.Solution = d.solution
	}
	return out, nil
}

// runNames are the names of the characters a run mentions, by ID.
type runNames map[string]string

// namesOfRuns reads the names of the characters that moved last or solved the
// runs, whatever their status now (a character that died still moved). It reads
// inside tx when the caller has one.
func (s *Service) namesOfRuns(ctx context.Context, tx pgx.Tx, campaignID string, runs ...playdb.PuzzleRun) (runNames, error) {
	var ids []string
	for _, r := range runs {
		for _, id := range []*string{r.LastMoverCharacterID, r.SolvedByCharacterID} {
			if id != nil {
				ids = append(ids, *id)
			}
		}
	}
	names := runNames{}
	if len(ids) == 0 {
		return names, nil
	}
	chars, err := s.roster.SessionCharacters(ctx, tx, campaignID, ids)
	if err != nil {
		return nil, fmt.Errorf("read the names of the characters that played: %w", err)
	}
	for _, c := range chars {
		names[c.ID] = c.Name
	}
	return names, nil
}

// playerView is what the run's reader has that is theirs alone, read for them before
// the message is built (viewerOf): the zero value is a reader with nothing of their own,
// which is what the master's copy of the players' view gets.
type playerView struct {
	// now is the clock the limits and the sequence's play are measured against.
	now time.Time
	// player says the reader is a player (the master's copy is not).
	player bool
	// wrongMine is the reader's wrong moves in the round.
	wrongMine int
	// won is how many hints the reader has won by a skill check, and tried says they
	// tried for the one they read next already.
	won   int
	tried bool
	// keyFound says the reader's character found the clue with the cipher's key.
	keyFound bool
	// hasCharacter says the reader has a living character in the party to roll for, and
	// trapSeen that they see the trap point a wrong move fired (RN-10): without it the
	// trap's name is not theirs to read.
	hasCharacter, trapSeen bool
	// myPart is the reader's part of the split information, and partHolders the names of
	// the others that have one.
	myPart      string
	partHolders []string
}

// playerRunProto is a shown puzzle as a player reads it: the state, the clue, the
// hints they may read, who moved last and whether it is solved, and what is theirs
// (playerView), and nothing else.
func playerRunProto(d puzzleDef, run playdb.PuzzleRun, names runNames, v playerView) (*playv1.PuzzleRun, error) {
	state := &playv1.PuzzleState{}
	if err := fromJSON(run.State, state); err != nil {
		return nil, err
	}
	released := min(max(int(run.ReleasedHints), 0), len(d.hints))
	// A player reads the hints the master released, and the ones they won themselves.
	visible := max(released, min(v.won, len(d.hints)))
	out := &playv1.PuzzleRun{
		PuzzleId: d.row.ID, Name: d.row.Name, Kind: d.kind.id(),
		Config: d.kind.playerConfig(d.config), Symbols: d.kind.symbols(d.config),
		State: state, Clue: d.row.Clue, Hints: append([]string(nil), d.hints[:visible]...),
		SharedHints: clamp32(released, 0, maxHints),
		Revision:    run.Revision, Solved: run.SolvedAt != nil,
	}
	// The turning symbols show the players the mural they copy (the artboard E10-06);
	// nothing else of the solution leaves the server.
	out.Mural = d.solution.GetPillars()
	if run.SolvedAt != nil {
		out.SolvedAt = timestamppb.New(*run.SolvedAt)
		out.SolvedMessage = deref(run.SolveMessage)
		if run.SolvedByCharacterID != nil {
			out.SolvedByName = names[*run.SolvedByCharacterID]
		}
	}
	if len(run.LastMove) > 0 {
		last, err := lastMoveOf(run)
		if err != nil {
			return nil, err
		}
		if run.LastMoverCharacterID != nil {
			last.CharacterName = names[*run.LastMoverCharacterID]
		}
		// A typed answer is its author's, and which bell was struck would give the
		// sequence away: the players read that it was wrong, and for which step.
		if d.kind.judges() {
			last.Move = nil
		}
		// The trap's name goes only to a player who sees its point, and nothing at all to
		// the others (an empty name would hint that there is a trap).
		if v.player && !v.trapSeen {
			last.TrapName = ""
		}
		out.LastMove = last
	}
	reason := stopOf(d, run, v.now)
	if reason != playv1.PuzzleStopReason_PUZZLE_STOP_REASON_UNSPECIFIED {
		out.Stopped, out.StoppedMessage = true, stoppedLine
	}
	out.Limits = limitsProto(d, run, v.now, v.wrongMine)
	if d.hintCheck != nil {
		out.HintByCheck, out.HintSkillKey = true, d.hintCheck.GetSkillKey()
		out.CanTryHint = v.player && v.hasCharacter && !out.Solved && !out.Stopped && visible < len(d.hints) && !v.tried
	}
	out.MyPart, out.PartHolders = v.myPart, v.partHolders
	if d.solution.GetSequence() != nil {
		out.Sequence = playbackProto(d, run, v.now)
	}
	if id := d.config.GetCipher().GetKeyClueId(); id != "" {
		out.HasKeyClue = true
		if v.keyFound {
			out.KeyClueId = id
		}
	}
	return out, nil
}

// lastMoveOf decodes the run's last move, as stored (with the typed text or the bell).
func lastMoveOf(run playdb.PuzzleRun) (*playv1.PuzzleLastMove, error) {
	last := &playv1.PuzzleLastMove{}
	if err := fromJSON(run.LastMove, last); err != nil {
		return nil, err
	}
	last.At = timestampOrNil(run.LastMovedAt)
	return last, nil
}

// playbackProto is what the players have seen of the sequence: the steps revealed by
// the play that runs, and never a step before its time or once the play ended.
func playbackProto(d puzzleDef, run playdb.PuzzleRun, now time.Time) *playv1.SequencePlayback {
	steps := d.solution.GetSequence().GetSteps()
	out := &playv1.SequencePlayback{
		TotalSteps: clamp32(len(steps), 0, puzzle.MaxSteps), Plays: run.Plays, StepMs: clamp32(int(puzzle.SequenceStep/time.Millisecond), 0, 1<<20),
	}
	if playing, rev := sequencePlaying(d, run, now); playing {
		out.Playing = true
		out.Shown = slices.Clone(steps[:rev.shown])
		out.NextInMs = clamp32(int(rev.next/time.Millisecond)+1, 1, 1<<20) // a millisecond past the change
	}
	return out
}

// viewerOf reads what is the reader's own in a run, inside tx when the caller has one
// (nil: the pool): their wrong moves, the hints they won, their part of the clue, and
// whether they found the cipher's key. The master has none of it.
func (s *Service) viewerOf(ctx context.Context, tx pgx.Tx, m authz.Membership, d puzzleDef, run playdb.PuzzleRun) (playerView, error) {
	v := playerView{now: s.now()}
	if m.Role == authz.RoleMaster {
		return v, nil
	}
	v.player = true
	q := s.queriesIn(tx)
	if d.onWrong.GetAttemptsPerPlayer() > 0 {
		rows, err := q.CountWrongPuzzleMoves(ctx, playdb.CountWrongPuzzleMovesParams{RunID: run.ID, Seq: run.RoundStartSeq})
		if err != nil {
			return v, fmt.Errorf("count the player's wrong moves: %w", err)
		}
		for _, r := range rows {
			if r.UserID != nil && *r.UserID == m.UserID {
				v.wrongMine = int(r.Wrong)
			}
		}
	}
	if d.hintCheck != nil {
		won, err := q.GetPuzzleHintCursor(ctx, playdb.GetPuzzleHintCursorParams{RunID: run.ID, UserID: &m.UserID})
		if err != nil {
			return v, fmt.Errorf("read the hints the player won: %w", err)
		}
		v.won = int(won)
		next := max(int(run.ReleasedHints), v.won)
		if next < len(d.hints) {
			if v.tried, err = q.HasTriedPuzzleHint(ctx, playdb.HasTriedPuzzleHintParams{RunID: run.ID, UserID: &m.UserID, HintIndex: clamp32(next, 0, maxHints)}); err != nil {
				return v, fmt.Errorf("read whether the player tried: %w", err)
			}
		}
	}
	if id := d.config.GetCipher().GetKeyClueId(); id != "" && s.puzzles.maps != nil {
		found, err := s.puzzles.maps.PuzzleClueFoundBy(ctx, tx, m.CampaignID, id, m.UserID)
		if err != nil {
			return v, err
		}
		v.keyFound = found
	}
	if last := (&playv1.PuzzleLastMove{}); len(run.LastMove) > 0 {
		if err := fromJSON(run.LastMove, last); err != nil {
			return v, err
		}
		if last.GetTrapName() != "" && d.onWrong.GetTrap() != nil && s.puzzles.maps != nil {
			t := d.onWrong.GetTrap()
			seen, err := s.puzzles.maps.PuzzleTrapSeenBy(ctx, m, t.GetMapId(), t.GetPointId())
			if err != nil {
				return v, err
			}
			v.trapSeen = seen
		}
	}
	if len(d.parts) > 0 || d.hintCheck != nil {
		party, err := s.roster.CombatParty(ctx, tx, m.CampaignID)
		if err != nil {
			return v, err
		}
		v.hasCharacter = slices.ContainsFunc(party, func(c link.Character) bool { return c.PlayerUserID != "" && c.PlayerUserID == m.UserID })
		mine := slices.IndexFunc(party, func(c link.Character) bool { return c.PlayerUserID != "" && c.PlayerUserID == m.UserID })
		for _, p := range d.parts {
			i := slices.IndexFunc(party, func(c link.Character) bool { return c.ID == p.GetCharacterId() })
			switch {
			case i < 0: // no owner, or one that is not in the party any more: nobody reads it
			case i == mine:
				v.myPart = p.GetText()
			default:
				v.partHolders = append(v.partHolders, party[i].Name)
			}
		}
	}
	return v, nil
}

// masterExtra is what the master reads of a run besides what the players read: the
// tries for a hint and each player's wrong moves.
type masterExtra struct {
	now   time.Time
	tries []playdb.PuzzleHintTry
	wrong map[string]int // wrong moves of the round, by user
	party []link.Character
	names runNames // of the characters of the tries
}

// masterExtras reads them inside tx when the caller has one.
func (s *Service) masterExtras(ctx context.Context, tx pgx.Tx, campaignID string, d puzzleDef, run playdb.PuzzleRun) (masterExtra, error) {
	ex := masterExtra{now: s.now(), wrong: map[string]int{}, names: runNames{}}
	q := s.queriesIn(tx)
	tries, err := q.ListRecentPuzzleHintTries(ctx, run.ID)
	if err != nil {
		return ex, fmt.Errorf("list the tries for a hint: %w", err)
	}
	slices.Reverse(tries) // oldest first
	ex.tries = tries
	var ids []string
	for _, t := range tries {
		if t.CharacterID != nil {
			ids = append(ids, *t.CharacterID)
		}
	}
	if len(ids) > 0 {
		chars, err := s.roster.SessionCharacters(ctx, tx, campaignID, ids)
		if err != nil {
			return ex, fmt.Errorf("read the names of the characters that tried: %w", err)
		}
		for _, c := range chars {
			ex.names[c.ID] = c.Name
		}
	}
	if d.onWrong.GetAttemptsPerPlayer() > 0 {
		rows, err := q.CountWrongPuzzleMoves(ctx, playdb.CountWrongPuzzleMovesParams{RunID: run.ID, Seq: run.RoundStartSeq})
		if err != nil {
			return ex, fmt.Errorf("count the wrong moves: %w", err)
		}
		for _, r := range rows {
			if r.UserID != nil {
				ex.wrong[*r.UserID] = int(r.Wrong)
			}
		}
		if ex.party, err = s.roster.CombatParty(ctx, tx, campaignID); err != nil {
			return ex, err
		}
	}
	return ex, nil
}

// runStatus is where a puzzle stands in the session: no run, or a run the master
// only prepared, is not shown.
func runStatus(run *playdb.PuzzleRun) playv1.PuzzleRunStatus {
	switch {
	case run == nil || run.ShownAt == nil:
		return playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_NOT_SHOWN
	case run.ClosedAt != nil:
		return playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_CLOSED
	case run.SolvedAt != nil:
		return playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_SOLVED
	}
	return playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_SHOWN
}

// visible says whether the players see the run: shown, and not closed.
func visible(run *playdb.PuzzleRun) bool {
	return run != nil && run.ShownAt != nil && run.ClosedAt == nil
}

// masterRunProto is a puzzle in the session as the master sees it: the puzzle, its
// status, what the players read (for a run that exists) and the numbers only the
// master may know.
func masterRunProto(d puzzleDef, run *playdb.PuzzleRun, names runNames, shown bool, ex masterExtra) (*playv1.MasterPuzzleRun, error) {
	puz, err := puzzleProto(d, shown)
	if err != nil {
		return nil, err
	}
	out := &playv1.MasterPuzzleRun{Puzzle: puz, Status: runStatus(run)}
	if run == nil {
		return out, nil
	}
	players, err := playerRunProto(d, *run, names, playerView{now: ex.now})
	if err != nil {
		return nil, err
	}
	start := &playv1.PuzzleState{}
	if err := fromJSON(run.Start, start); err != nil {
		return nil, err
	}
	if out.Minimum, err = d.kind.minimum(d, players.GetState()); err != nil {
		return nil, err
	}
	if out.MinimumFromStart, err = d.kind.minimum(d, start); err != nil {
		return nil, err
	}
	out.Start = start
	out.ReleasedHints = int32(min(max(int(run.ReleasedHints), 0), len(d.hints))) //nolint:gosec // at most 10
	out.MovesMade = run.MovesMade
	if run.ShownAt != nil {
		out.Run = players
	}
	if run.SolvedAt != nil {
		out.Outcome = outcomeOf(deref(run.SolveOutcome))
	}
	if len(run.LastMove) > 0 {
		if out.LastMove, err = lastMoveOf(*run); err != nil {
			return nil, err
		}
		if run.LastMoverCharacterID != nil {
			out.LastMove.CharacterName = names[*run.LastMoverCharacterID]
		}
	}
	out.StopReason = stopOf(d, *run, ex.now)
	for _, t := range ex.tries {
		try := &playv1.PuzzleHintTry{
			Hint: t.HintIndex + 1, Passed: t.Passed, At: timestamppb.New(t.CreatedAt),
			Roll: diceRoll(1, 20, []int32{t.D20}, t.Modifier, t.Total, t.Physical),
		}
		if t.CharacterID != nil {
			try.CharacterName = ex.names[*t.CharacterID]
		}
		out.HintTries = append(out.HintTries, try)
	}
	if limit := d.onWrong.GetAttemptsPerPlayer(); limit > 0 {
		for _, c := range ex.party {
			if c.PlayerUserID == "" {
				continue
			}
			wrong := ex.wrong[c.PlayerUserID]
			out.Attempts = append(out.Attempts, &playv1.PuzzleAttempts{
				CharacterName: c.Name, Wrong: clamp32(wrong, 0, 1<<20), Left: max(limit-clamp32(wrong, 0, 1<<20), 0),
			})
		}
	}
	return out, nil
}
