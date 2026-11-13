package play

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The puzzle writers (CreatePuzzle, UpdatePuzzle, archiving, ShowPuzzle and the master's other changes of a run, MakePuzzleMove and
// TryPuzzleHint; #735 part 2, 4d) go through Service.tx: the session's number moves once with the write, puzzle_changed carries it,
// a repeat of a key moves nothing and tells again, a refused call tells nothing, and a puzzle nobody was shown tells nobody.

const kPuzzleChanged = "*playv1.WatchGameSessionResponse_PuzzleChanged_"

// puzzleHints is the table, with the puzzle hint's time gate open at once (its own tests are TestMR038_TheHintIsThrottled and
// TestPuzzleGate: here every write's hint goes out with it, so a lost one is seen) and a stream for the master and each player.
func (p *puzzleTable) puzzleHints(t *testing.T, quiet ...string) *hintRig {
	t.Helper()
	p.h.svc.puzzles.hintEvery = time.Nanosecond
	p.h.svc.puzzles.hints.after = p.h.hints.After
	g := newHintRig(t, p.h, p.campaignID, p.master, map[string]*user{"Ana": p.ana, "Caio": p.caio, "Bia": p.bia}, quiet...)
	g.unnumbered = []string{kMapChanged}
	return g
}

var everyoneHearsAPuzzle = map[string][]string{"master": {kPuzzleChanged}, "Ana": {kPuzzleChanged}, "Caio": {kPuzzleChanged}, "Bia": {kPuzzleChanged}}

func (p *puzzleTable) showCall(t *testing.T, id string) func() error {
	return func() error {
		_, err := p.pc(p.master).ShowPuzzle(t.Context(), connect.NewRequest(&playv1.ShowPuzzleRequest{CampaignId: p.campaignID, PuzzleId: id}))
		return err
	}
}

func (p *puzzleTable) moveCall(t *testing.T, u *user, id string, mv *playv1.PuzzleMove, key string) func() error {
	return func() error {
		_, err := p.moveKey(t, u, id, mv, key)
		return err
	}
}

func (p *puzzleTable) hintCall(t *testing.T, u *user, id string, face int32, key string) func() error {
	return func() error {
		_, err := p.tryHintKey(t, u, id, face, key)
		return err
	}
}

func TestShowPuzzleHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	g := p.puzzleHints(t)
	heard := g.first(p.showCall(t, puz.GetId()), true)
	heardKinds(t, heard, everyoneHearsAPuzzle)
}

func TestShowPuzzleAgainMovesNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	before := g.seq()
	do(t, p.showCall(t, puz.GetId()))
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d, want %d: the puzzle was shown already", got, before)
	}
	for name, evs := range g.drain() {
		if len(evs) != 0 {
			t.Errorf("%s heard %v of a change that changed nothing", name, eventKinds(evs))
		}
	}
}

func TestResetAndCloseAPuzzleHint(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	heard := g.first(func() error {
		_, err := p.pc(p.master).ClosePuzzle(t.Context(), connect.NewRequest(&playv1.ClosePuzzleRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId()}))
		return err
	}, true)
	heardKinds(t, heard, everyoneHearsAPuzzle)
}

// A run prepared at another start is the master's alone: nobody is told, and the number moves.
func TestReseedOfAPuzzleNotShownMovesTheNumberAndTellsNobody(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lights(t, "Selos", 3)
	g := p.puzzleHints(t, "Ana", "Caio", "Bia")
	g.first(func() error {
		_, err := p.pc(p.master).ReseedPuzzle(t.Context(), connect.NewRequest(&playv1.ReseedPuzzleRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId()}))
		return err
	}, false)
}

func TestShowPuzzleRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	g := p.puzzleHints(t)
	g.refused(p.showCall(t, newKey())) // no such puzzle
}

func TestShowPuzzleWithNoOpenSessionRefusesAsBefore(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.master.end(t, p.master.liveSession(t, p.campaignID).GetGameSession())
	wantPuzzleBlocked(t, "ShowPuzzle", p.showCall(t, puz.GetId())(), playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_OPEN_SESSION)
}

func TestMakePuzzleMoveHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	key := newKey()
	g.keyed(p.moveCall(t, p.caio, puz.GetId(), lockMove(0, 1), key), p.moveCall(t, p.caio, puz.GetId(), lockMove(0, 1), key), p.moveCall(t, p.caio, puz.GetId(), lockMove(1, 1), key))
}

func TestMakePuzzleMoveLostHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	g.lost(p.moveCall(t, p.caio, puz.GetId(), lockMove(0, 1), newKey()))
}

func TestMakePuzzleMoveRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre") // not shown
	g := p.puzzleHints(t)
	g.refused(p.moveCall(t, p.caio, puz.GetId(), lockMove(0, 1), newKey()))
}

// The solve that opens a door tells the door again when its key is repeated.
func TestMakePuzzleMoveThatOpensADoorLostHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	p.paintDoor(t, mapsv1.DoorState_DOOR_STATE_CLOSED, 7, 4)
	puz := p.oneMoveLock(t, doorTarget(p.mapID, 7, 4))
	g := p.puzzleHints(t)
	g.lost(p.moveCall(t, p.caio, puz.GetId(), lockMove(0, 1), newKey()))
}

func TestTryPuzzleHintHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	key := newKey()
	g.keyed(p.hintCall(t, p.caio, puz.GetId(), 20, key), p.hintCall(t, p.caio, puz.GetId(), 20, key), p.hintCall(t, p.caio, puz.GetId(), 3, key))
}

func TestTryPuzzleHintLostHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	g.lost(p.hintCall(t, p.caio, puz.GetId(), 20, newKey()))
}

func TestTryPuzzleHintRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre", hintCheck("skill:investigation", 15)) // not shown
	g := p.puzzleHints(t)
	g.refused(p.hintCall(t, p.caio, puz.GetId(), 20, newKey()))
}

func (p *puzzleTable) createCall(t *testing.T, name, key string) func() error {
	return func() error {
		_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(&playv1.CreatePuzzleRequest{
			CampaignId: p.campaignID, Name: name, IdempotencyKey: key, Config: lockConfig(4, playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS),
			Solution: lockSolution(7, 3, 5, 1), Start: lockStart(0, 0, 0, 0),
		}))
		return err
	}
}

// A puzzle the master makes is in his list and in nobody else's: the number moves, and nobody is told.
func TestCreatePuzzleHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	g := p.puzzleHints(t, "Ana", "Caio", "Bia")
	key := newKey()
	var first map[string][]string
	t.Run("the write moves the number once", func(_ *testing.T) { first = g.first(p.createCall(t, "Cofre", key), false) })
	t.Run("the repeat moves nothing", func(_ *testing.T) { g.replay(p.createCall(t, "Cofre", key), first) })
	t.Run("the key used for another request tells nothing", func(_ *testing.T) { g.refused(p.createCall(t, "Outro cofre", key)) })
}

func TestCreatePuzzleWithNoOpenSessionMovesNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	ended, err := p.h.svc.queries.GetOpenGameSession(t.Context(), p.campaignID)
	if err != nil {
		t.Fatalf("GetOpenGameSession() error = %v", err)
	}
	p.master.end(t, p.master.liveSession(t, p.campaignID).GetGameSession())
	before := p.h.hintSeq(t, ended.ID)
	do(t, p.createCall(t, "Cofre", newKey()))
	if got := p.h.hintSeq(t, ended.ID); got != before {
		t.Errorf("hint_seq of the ended session = %d, want %d", got, before)
	}
}

func TestUpdatePuzzleHints(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	g := p.puzzleHints(t, "Ana", "Caio", "Bia")
	g.first(func() error {
		_, err := p.pc(p.master).UpdatePuzzle(t.Context(), connect.NewRequest(&playv1.UpdatePuzzleRequest{
			CampaignId: p.campaignID, PuzzleId: puz.GetId(), Name: "Cofre velho", Config: lockConfig(4, playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS),
			Solution: lockSolution(7, 3, 5, 1), Start: lockStart(0, 0, 0, 0),
		}))
		return err
	}, false)
}

func TestUpdatePuzzleOfAShownOneRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	g.refused(func() error {
		_, err := p.pc(p.master).UpdatePuzzle(t.Context(), connect.NewRequest(&playv1.UpdatePuzzleRequest{
			CampaignId: p.campaignID, PuzzleId: puz.GetId(), Name: "Cofre velho", Config: lockConfig(4, playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS),
			Solution: lockSolution(7, 3, 5, 1), Start: lockStart(0, 0, 0, 0),
		}))
		return err
	})
}

func (p *puzzleTable) archiveCall(t *testing.T, id string) func() error {
	return func() error {
		_, err := p.pc(p.master).ArchivePuzzle(t.Context(), connect.NewRequest(&playv1.ArchivePuzzleRequest{CampaignId: p.campaignID, PuzzleId: id}))
		return err
	}
}

func TestArchivePuzzleNotShownTellsNobody(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	g := p.puzzleHints(t, "Ana", "Caio", "Bia")
	g.first(p.archiveCall(t, puz.GetId()), false)
}

func TestArchivePuzzleShownTellsTheTable(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	p.show(t, puz.GetId())
	g := p.puzzleHints(t)
	heard := g.first(p.archiveCall(t, puz.GetId()), true)
	heardKinds(t, heard, everyoneHearsAPuzzle)
}

func TestArchivePuzzleAgainMovesNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	do(t, p.archiveCall(t, puz.GetId()))
	g := p.puzzleHints(t)
	before := g.seq()
	do(t, p.archiveCall(t, puz.GetId()))
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d, want %d: the puzzle was archived already", got, before)
	}
}

func TestArchivePuzzleRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	g := p.puzzleHints(t)
	g.refused(p.archiveCall(t, newKey())) // no such puzzle
}
