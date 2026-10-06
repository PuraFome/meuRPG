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

// The master's calls on a puzzle in the open session: show it, put it back at its
// start, draw another start, close it, release a hint, and read it live. Every
// change takes the same steps, in the same order: lock the session's row (so the
// events get their numbers in order, and a move waits for the change in progress),
// lock the puzzle, lock the run. Taking the locks in one order everywhere is what
// keeps two changes from waiting on each other.

// runTx is one change to a puzzle's run, inside its transaction.
type runTx struct {
	m       authz.Membership
	q       *playdb.Queries
	session playdb.GameSession
	def     puzzleDef
	run     *playdb.PuzzleRun // nil: the puzzle has no run in this session yet
	now     time.Time
	tell    bool // send puzzle_changed to the session once the transaction commits
}

// params is the run as a SavePuzzleRun that changes nothing.
func (c *runTx) params() playdb.SavePuzzleRunParams {
	r := c.run
	return playdb.SavePuzzleRunParams{
		ID: r.ID, Seed: r.Seed, Start: r.Start, State: r.State, ReleasedHints: r.ReleasedHints, ShownAt: r.ShownAt,
		ClosedAt: r.ClosedAt, SolvedAt: r.SolvedAt, SolvedByCharacterID: r.SolvedByCharacterID, SolveOutcome: r.SolveOutcome,
		LastMoverCharacterID: r.LastMoverCharacterID, LastMove: r.LastMove, LastMovedAt: r.LastMovedAt,
		MovesMade: r.MovesMade, Revision: r.Revision, SolveMessage: r.SolveMessage,
	}
}

// save writes the changed run, one revision up.
func (c *runTx) save(ctx context.Context, p playdb.SavePuzzleRunParams) error {
	p.Revision = c.run.Revision + 1
	p.UpdatedAt = c.now
	row, err := c.q.SavePuzzleRun(ctx, p)
	if err != nil {
		return fmt.Errorf("save the puzzle's run: %w", err)
	}
	c.run = &row
	return nil
}

// toStart puts the pieces back where the run started and forgets that it was
// solved and who moved last; the hints stay released and the moves stay in the
// history.
func toStart(p *playdb.SavePuzzleRunParams) {
	p.State = p.Start
	p.SolvedAt, p.SolvedByCharacterID, p.SolveOutcome, p.SolveMessage = nil, nil, nil, nil
	p.LastMoverCharacterID, p.LastMove, p.LastMovedAt = nil, nil, nil
}

// event writes one of the puzzle's session events about the run.
func (c *runTx) event(ctx context.Context, kind string) error {
	return puzzleSessionEvent(ctx, c.q, c.session, kind, c.m.UserID, nil, puzzleEvent{PuzzleID: c.def.row.ID, RunID: c.run.ID}, c.now)
}

// changeRun runs the master's change in one transaction, and answers with the
// puzzle as the master now sees it.
func (s *Service) changeRun(ctx context.Context, m authz.Membership, rawID string, change func(c *runTx) error) (*playv1.MasterPuzzleRun, error) {
	id, err := parsePuzzleID(rawID)
	if err != nil {
		return nil, err
	}
	var c *runTx
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queriesIn(tx)
		session, err := lockOpenSession(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		row, err := q.GetPuzzleForUpdate(ctx, playdb.GetPuzzleForUpdateParams{CampaignID: m.CampaignID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPuzzleNotFound()
		}
		if err != nil {
			return fmt.Errorf("find puzzle: %w", err)
		}
		def, err := decodePuzzle(row)
		if err != nil {
			return err
		}
		c = &runTx{m: m, q: q, session: session, def: def, now: s.now()}
		run, err := q.GetPuzzleRunForUpdate(ctx, playdb.GetPuzzleRunForUpdateParams{GameSessionID: session.ID, PuzzleID: id})
		switch {
		case err == nil:
			c.run = &run
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the puzzle's run: %w", err)
		}
		return change(c)
	})
	if err != nil {
		return nil, s.dbError(ctx, "change a puzzle", err)
	}
	if c.tell {
		s.publishPuzzleChanged(m.CampaignID, id)
	}
	return s.masterView(ctx, nil, m.CampaignID, c.def, c.run)
}

// masterView is the master's message for a puzzle and its run (nil: none).
func (s *Service) masterView(ctx context.Context, tx pgx.Tx, campaignID string, d puzzleDef, run *playdb.PuzzleRun) (*playv1.MasterPuzzleRun, error) {
	var runs []playdb.PuzzleRun
	if run != nil {
		runs = append(runs, *run)
	}
	names, err := s.namesOfRuns(ctx, tx, campaignID, runs...)
	if err != nil {
		return nil, s.dbError(ctx, "read the puzzle's run", err)
	}
	shown, err := s.shownSet(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the puzzle's run", err)
	}
	out, err := masterRunProto(d, run, names, shown[d.row.ID])
	if err != nil {
		return nil, s.dbError(ctx, "read the puzzle's run", err)
	}
	return out, nil
}

// ShowPuzzle implements playv1connect.PuzzleServiceHandler.
func (s *Service) ShowPuzzle(
	ctx context.Context,
	req *connect.Request[playv1.ShowPuzzleRequest],
) (*connect.Response[playv1.ShowPuzzleResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	run, err := s.changeRun(ctx, m, req.Msg.GetPuzzleId(), func(c *runTx) error {
		if c.def.row.ArchivedAt != nil {
			return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_ARCHIVED, "the puzzle is archived")
		}
		switch {
		case c.run == nil:
			row, err := c.q.InsertPuzzleRun(ctx, playdb.InsertPuzzleRunParams{
				GameSessionID: c.session.ID, PuzzleID: c.def.row.ID, Seed: c.def.row.Seed, Start: c.def.row.Start, ShownAt: &c.now, CreatedAt: c.now,
			})
			if err != nil {
				return fmt.Errorf("make the puzzle's run: %w", err)
			}
			c.run = &row
		case visible(c.run):
			return nil // shown already: nothing changes
		default:
			// Prepared with a new start, or closed: shown (again) at its start.
			p := c.params()
			p.ShownAt, p.ClosedAt = &c.now, nil
			if c.run.ClosedAt != nil {
				toStart(&p)
			}
			if err := c.save(ctx, p); err != nil {
				return err
			}
		}
		c.tell = true
		return c.event(ctx, eventPuzzleShown)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ShowPuzzleResponse{Run: run}), nil
}

// needsVisibleRun is the refusal of a change that needs the players to see the
// puzzle.
func needsVisibleRun(c *runTx) error {
	if !visible(c.run) {
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NOT_SHOWN, "the puzzle is not shown in this session")
	}
	return nil
}

// ResetPuzzle implements playv1connect.PuzzleServiceHandler.
func (s *Service) ResetPuzzle(
	ctx context.Context,
	req *connect.Request[playv1.ResetPuzzleRequest],
) (*connect.Response[playv1.ResetPuzzleResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	run, err := s.changeRun(ctx, m, req.Msg.GetPuzzleId(), func(c *runTx) error {
		if err := needsVisibleRun(c); err != nil {
			return err
		}
		p := c.params()
		toStart(&p)
		if err := c.save(ctx, p); err != nil {
			return err
		}
		c.tell = true
		return c.event(ctx, eventPuzzleReset)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ResetPuzzleResponse{Run: run}), nil
}

// ReseedPuzzle implements playv1connect.PuzzleServiceHandler.
func (s *Service) ReseedPuzzle(
	ctx context.Context,
	req *connect.Request[playv1.ReseedPuzzleRequest],
) (*connect.Response[playv1.ReseedPuzzleResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	run, err := s.changeRun(ctx, m, req.Msg.GetPuzzleId(), func(c *runTx) error {
		if !c.def.kind.generates() {
			return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_GENERATED_START, "this kind has no generated start")
		}
		seed := s.newSeed()
		start, err := c.def.kind.generate(c.def, seed)
		if err != nil {
			return err
		}
		startJSON := toJSON(start)
		if c.run == nil {
			// Not shown yet: prepare the run at the new start. The players see nothing
			// until ShowPuzzle.
			row, err := c.q.InsertPuzzleRun(ctx, playdb.InsertPuzzleRunParams{
				GameSessionID: c.session.ID, PuzzleID: c.def.row.ID, Seed: seed, Start: startJSON, CreatedAt: c.now,
			})
			if err != nil {
				return fmt.Errorf("make the puzzle's run: %w", err)
			}
			c.run = &row
			return nil
		}
		p := c.params()
		p.Seed, p.Start = seed, startJSON
		toStart(&p)
		if err := c.save(ctx, p); err != nil {
			return err
		}
		if !visible(c.run) {
			return nil
		}
		c.tell = true
		return c.event(ctx, eventPuzzleReset)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ReseedPuzzleResponse{Run: run}), nil
}

// ClosePuzzle implements playv1connect.PuzzleServiceHandler.
func (s *Service) ClosePuzzle(
	ctx context.Context,
	req *connect.Request[playv1.ClosePuzzleRequest],
) (*connect.Response[playv1.ClosePuzzleResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	run, err := s.changeRun(ctx, m, req.Msg.GetPuzzleId(), func(c *runTx) error {
		if c.run != nil && c.run.ShownAt != nil && c.run.ClosedAt != nil {
			return nil // closed already: nothing changes
		}
		if err := needsVisibleRun(c); err != nil {
			return err
		}
		p := c.params()
		p.ClosedAt = &c.now
		if err := c.save(ctx, p); err != nil {
			return err
		}
		c.tell = true
		return c.event(ctx, eventPuzzleClosed)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ClosePuzzleResponse{Run: run}), nil
}

// ReleaseNextPuzzleHint implements playv1connect.PuzzleServiceHandler.
func (s *Service) ReleaseNextPuzzleHint(
	ctx context.Context,
	req *connect.Request[playv1.ReleaseNextPuzzleHintRequest],
) (*connect.Response[playv1.ReleaseNextPuzzleHintResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	run, err := s.changeRun(ctx, m, req.Msg.GetPuzzleId(), func(c *runTx) error {
		if err := needsVisibleRun(c); err != nil {
			return err
		}
		if int(c.run.ReleasedHints) >= len(c.def.hints) {
			return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_MORE_HINTS, "every hint is released already")
		}
		p := c.params()
		p.ReleasedHints++
		if err := c.save(ctx, p); err != nil {
			return err
		}
		c.tell = true
		return nil // a hint is not an event: the session's history keeps only shown, solved, reset and closed
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ReleaseNextPuzzleHintResponse{Run: run}), nil
}

// GetMasterPuzzleRun implements playv1connect.PuzzleServiceHandler.
func (s *Service) GetMasterPuzzleRun(
	ctx context.Context,
	req *connect.Request[playv1.GetMasterPuzzleRunRequest],
) (*connect.Response[playv1.GetMasterPuzzleRunResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, err := parsePuzzleID(req.Msg.GetPuzzleId())
	if err != nil {
		return nil, err
	}
	session, err := s.puzzleSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	row, err := s.queries.GetPuzzle(ctx, playdb.GetPuzzleParams{CampaignID: m.CampaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errPuzzleNotFound()
	}
	if err != nil {
		return nil, s.dbError(ctx, "find a puzzle", err)
	}
	d, err := decodePuzzle(row)
	if err != nil {
		return nil, s.dbError(ctx, "read a puzzle", err)
	}
	var run *playdb.PuzzleRun
	r, err := s.queries.GetPuzzleRun(ctx, playdb.GetPuzzleRunParams{GameSessionID: session.ID, PuzzleID: id})
	switch {
	case err == nil:
		run = &r
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, s.dbError(ctx, "find the puzzle's run", err)
	}
	out, err := s.masterView(ctx, nil, m.CampaignID, d, run)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.GetMasterPuzzleRunResponse{Run: out}), nil
}

// ListSessionPuzzles implements playv1connect.PuzzleServiceHandler.
func (s *Service) ListSessionPuzzles(
	ctx context.Context,
	req *connect.Request[playv1.ListSessionPuzzlesRequest],
) (*connect.Response[playv1.ListSessionPuzzlesResponse], error) {
	m, err := requireMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.puzzleSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListPuzzles(ctx, playdb.ListPuzzlesParams{CampaignID: m.CampaignID})
	if err != nil {
		return nil, s.dbError(ctx, "list puzzles", err)
	}
	runs, err := s.queries.ListPuzzleRuns(ctx, session.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the puzzles' runs", err)
	}
	byPuzzle := make(map[string]*playdb.PuzzleRun, len(runs))
	for i := range runs {
		byPuzzle[runs[i].PuzzleID] = &runs[i]
	}
	names, err := s.namesOfRuns(ctx, nil, m.CampaignID, runs...)
	if err != nil {
		return nil, s.dbError(ctx, "list the puzzles' runs", err)
	}
	shown, err := s.shownSet(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the puzzles' runs", err)
	}
	res := &playv1.ListSessionPuzzlesResponse{}
	for _, row := range rows {
		d, err := decodePuzzle(row)
		if err != nil {
			return nil, s.dbError(ctx, "read a puzzle", err)
		}
		out, err := masterRunProto(d, byPuzzle[row.ID], names, shown[row.ID])
		if err != nil {
			return nil, s.dbError(ctx, "read a puzzle", err)
		}
		res.Puzzles = append(res.Puzzles, out)
	}
	return connect.NewResponse(res), nil
}
