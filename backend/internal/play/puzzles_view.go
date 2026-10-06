package play

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// How a puzzle and a run are shown. There are exactly two audiences, and each has
// its own builder, so that nothing the master alone may know can slip into what a
// player gets (RN-10, RN-27): puzzleProto and masterRunProto are the master's;
// playerRunProto is the players'. The player's builder reads only the config
// without its secrets, the state, the clue, the released hints and the last move.

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

// playerRunProto is a shown puzzle as a player reads it: the state, the clue, the
// released hints, who moved last and whether it is solved, and nothing else.
func playerRunProto(d puzzleDef, run playdb.PuzzleRun, names runNames) (*playv1.PuzzleRun, error) {
	state := &playv1.PuzzleState{}
	if err := fromJSON(run.State, state); err != nil {
		return nil, err
	}
	released := min(max(int(run.ReleasedHints), 0), len(d.hints))
	out := &playv1.PuzzleRun{
		PuzzleId: d.row.ID, Name: d.row.Name, Kind: d.kind.id(),
		Config: d.kind.playerConfig(d.config), Symbols: d.kind.symbols(d.config),
		State: state, Clue: d.row.Clue, Hints: append([]string(nil), d.hints[:released]...),
		Revision: run.Revision, Solved: run.SolvedAt != nil,
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
		last := &playv1.PuzzleLastMove{}
		if err := fromJSON(run.LastMove, last); err != nil {
			return nil, err
		}
		if run.LastMoverCharacterID != nil {
			last.CharacterName = names[*run.LastMoverCharacterID]
		}
		last.At = timestampOrNil(run.LastMovedAt)
		out.LastMove = last
	}
	return out, nil
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
func masterRunProto(d puzzleDef, run *playdb.PuzzleRun, names runNames, shown bool) (*playv1.MasterPuzzleRun, error) {
	puz, err := puzzleProto(d, shown)
	if err != nil {
		return nil, err
	}
	out := &playv1.MasterPuzzleRun{Puzzle: puz, Status: runStatus(run)}
	if run == nil {
		return out, nil
	}
	players, err := playerRunProto(d, *run, names)
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
	return out, nil
}
