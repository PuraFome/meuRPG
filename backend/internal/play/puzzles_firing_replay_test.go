package play

import (
	"encoding/json"
	"testing"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The retry of a wrong move tells the firing that move made, read from the event the move wrote, and not the latest firing of the
// puzzle's trap: with two firings (the master armed the trap again between two wrong moves) the retry of the first tells the first.

// firingOfMove is the firing the stored event of the move holds, as the retry of that move reads it.
func (p *puzzleTable) firingOfMove(t *testing.T, puzzleID, key string) *trapFireEvent {
	t.Helper()
	ctx := t.Context()
	session, err := p.h.svc.queries.GetOpenGameSession(ctx, p.campaignID)
	if err != nil {
		t.Fatalf("GetOpenGameSession() error = %v", err)
	}
	tx, err := p.h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a read: nothing to undo
	q := p.h.svc.queriesIn(tx)
	d, _, err := p.h.svc.shownRun(ctx, q, session, p.campaignID, puzzleID)
	if err != nil {
		t.Fatalf("shownRun() error = %v", err)
	}
	res := &moveResult{}
	if err := p.h.svc.recallFiring(ctx, tx, q, session, d, key, res); err != nil {
		t.Fatalf("recallFiring() error = %v", err)
	}
	return res.fired
}

// storedFiring is the nth firing event of the session (0 is the first written), whichever move wrote it.
func (p *puzzleTable) storedFiring(t *testing.T, n int) *trapFireEvent {
	t.Helper()
	var payload []byte
	if err := p.h.pool.QueryRow(t.Context(), `SELECT payload FROM session_events WHERE kind = 'trap_triggered' ORDER BY seq OFFSET $1 LIMIT 1`, n).Scan(&payload); err != nil {
		t.Fatalf("read firing %d: %v", n, err)
	}
	var ev actionEvent
	if err := json.Unmarshal(payload, &ev); err != nil || ev.Trap == nil {
		t.Fatalf("firing %d cannot be read: %v", n, err)
	}
	return ev.Trap
}

func TestTheRetryOfAWrongMoveTellsTheFiringOfThatMove(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	trap := p.trapPoint(t, "Dardos envenenados", 12, 4)
	target := &playv1.PuzzleTrapTarget{MapId: p.mapID, PointId: trap.GetId()}
	seq := p.sequence(t, "Os sinos do Salão do trono", onWrong(onWrongTrap(target)))
	p.show(t, seq.GetId())
	p.playSequence(t, seq.GetId())
	p.movableClock().advance(time.Minute)

	first, second := newKey(), newKey()
	if _, err := p.moveKey(t, p.ana, seq.GetId(), bellMove(3), first); err != nil {
		t.Fatalf("the first wrong move: %v", err)
	}
	// The master armed the trap again: it fires on the next wrong move, and the trap's latest firing is that one.
	if _, err := p.h.pool.Exec(t.Context(), `UPDATE map_points SET trap_state = 'armed' WHERE id = $1`, trap.GetId()); err != nil {
		t.Fatalf("arm the trap again: %v", err)
	}
	p.movableClock().advance(time.Minute) // the sequence a wrong bell restarts has been played
	if _, err := p.moveKey(t, p.caio, seq.GetId(), bellMove(3), second); err != nil {
		t.Fatalf("the second wrong move: %v", err)
	}
	if p.eventCount(t, "trap_triggered") != 2 {
		t.Fatalf("%d firings, want 2: the test has no two firings to tell apart", p.eventCount(t, "trap_triggered"))
	}
	one, two := p.storedFiring(t, 0), p.storedFiring(t, 1)
	if (one.PrevTriggeredAt == nil) == (two.PrevTriggeredAt == nil) {
		t.Fatalf("the two firings read the same (%+v and %+v): the test cannot tell them apart", one, two)
	}

	got := p.firingOfMove(t, seq.GetId(), first)
	if got == nil || (got.PrevTriggeredAt == nil) != (one.PrevTriggeredAt == nil) {
		t.Errorf("the retry of the first wrong move tells %+v, want the first firing %+v", got, one)
	}
	got = p.firingOfMove(t, seq.GetId(), second)
	if got == nil || (got.PrevTriggeredAt == nil) != (two.PrevTriggeredAt == nil) {
		t.Errorf("the retry of the second wrong move tells %+v, want the second firing %+v", got, two)
	}
}
