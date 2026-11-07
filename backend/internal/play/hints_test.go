package play

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// hintTable is a session with one open stream (the master's) and a scratch table whose rows stand for "the write".
type hintTable struct {
	h          *harness
	campaignID string
	sessionID  string
	master     *live.Subscription
}

func newHintTable(t *testing.T) *hintTable {
	t.Helper()
	h := newHarness(t)
	master := h.newUser("Mestra")
	campaignID := h.newCampaign(master, "Hints")
	sessionID := master.start(t, campaignID).GetGameSession().GetId()
	ctx := t.Context()
	if _, err := h.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS hint_probe (id INT PRIMARY KEY, n INT NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = h.pool.Exec(context.WithoutCancel(ctx), `DROP TABLE IF EXISTS hint_probe`) })
	sub, err := h.svc.hub.Subscribe(campaignID, live.Subscriber{UserID: master.id, Master: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.Close)
	return &hintTable{h: h, campaignID: campaignID, sessionID: sessionID, master: sub}
}

// event is a hint whose Coalesce names it, so a test can tell which one arrived.
func (w *hintTable) event(name string, everyone bool) live.Event {
	return live.Event{
		Audience: live.Audience{Master: true, Everyone: everyone},
		Message:  &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_ContentChanged_{ContentChanged: &playv1.WatchGameSessionResponse_ContentChanged{}}},
		Coalesce: name,
	}
}

// heard drains the master's stream and returns the names of the events in it.
func (w *hintTable) heard() []string {
	var out []string
	for {
		select {
		case ev := <-w.master.Events():
			w.master.Taken(ev)
			out = append(out, ev.Coalesce)
		default:
			return out
		}
	}
}

func (w *hintTable) seq(t *testing.T) int64 {
	t.Helper()
	n, err := w.h.svc.queries.GetHintSeq(t.Context(), w.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// probe reads the scratch row id: -1 when it does not exist.
func (w *hintTable) probe(t *testing.T, id int) int {
	t.Helper()
	var n int
	err := w.h.pool.QueryRow(t.Context(), `SELECT n FROM hint_probe WHERE id = $1`, id).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return -1
	}
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *hintTable) write(ctx context.Context, tx pgx.Tx, id int) error {
	_, err := tx.Exec(ctx, `INSERT INTO hint_probe (id, n) VALUES ($1, 1) ON CONFLICT (id) DO UPDATE SET n = hint_probe.n + 1`, id)
	return err
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestTxResetsTheCollectorOnEveryAttempt(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	attempts := 0
	err := w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
		attempts++
		if err := w.write(ctx, tx, 1); err != nil {
			return err
		}
		if attempts == 1 {
			h.tell(w.campaignID, w.event("first-attempt", true))
			h.touch()
			return &pgconn.PgError{Code: "40001", Message: "restart transaction"}
		}
		h.tell(w.campaignID, w.event("second-attempt", true))
		return nil
	})
	if err != nil {
		t.Fatalf("tx() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("fn ran %d times, want 2 (one retry)", attempts)
	}
	if got := w.heard(); !equalNames(got, []string{"second-attempt"}) {
		t.Errorf("the streams heard %q, want only the second attempt's event, once", got)
	}
	if got := w.seq(t); got != 1 {
		t.Errorf("hint_seq = %d after a retried write, want 1", got)
	}
}

func TestTxAnErrorSendsNothingAndKeepsTheNumber(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	boom := errors.New("boom")
	err := w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
		if err := w.write(ctx, tx, 1); err != nil {
			return err
		}
		h.tell(w.campaignID, w.event("lost", true))
		h.touch()
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("tx() error = %v, want boom", err)
	}
	if got := w.heard(); len(got) != 0 {
		t.Errorf("the streams heard %q after a failed write", got)
	}
	if got := w.seq(t); got != 0 {
		t.Errorf("hint_seq = %d after a failed write, want 0", got)
	}
	if got := w.probe(t, 1); got != -1 {
		t.Errorf("the failed write left its row (n = %d)", got)
	}
}

// The number is saved with the write: a bump that cannot be made (the session is gone) rolls the write back and sends nothing.
func TestTxTheNumberAndTheWriteAreSavedTogether(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	err := w.h.svc.tx(ctx, "00000000-0000-4000-8000-000000000000", func(tx pgx.Tx, h *hints) error {
		if err := w.write(ctx, tx, 1); err != nil {
			return err
		}
		h.tell(w.campaignID, w.event("lost", true))
		return nil
	})
	if err == nil {
		t.Fatal("tx() on a session that does not exist returned nil")
	}
	if got := w.probe(t, 1); got != -1 {
		t.Errorf("the write stayed (n = %d) although its number could not be bumped", got)
	}
	if got := w.heard(); len(got) != 0 {
		t.Errorf("the streams heard %q", got)
	}
	if got := w.seq(t); got != 0 {
		t.Errorf("hint_seq = %d, want 0", got)
	}
}

func TestTxWithoutHintsDoesNotTouchTheRow(t *testing.T) {
	w := newHintTable(t)
	// Another transaction holds the session row's lock: a write of the row would wait for it until the deadline.
	conn, err := pgx.ConnectConfig(t.Context(), w.h.pool.Config().ConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(t.Context())) }()
	holder, err := conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.WithoutCancel(t.Context())) }()
	if _, err := holder.Exec(t.Context(), `SELECT id FROM game_sessions WHERE id = $1 FOR UPDATE`, w.sessionID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error { return w.write(ctx, tx, 1) })
	if err != nil {
		t.Fatalf("tx() of a write that told nothing, with the session row locked elsewhere: %v", err)
	}
	if got := w.probe(t, 1); got != 1 {
		t.Errorf("the write did not happen (n = %d)", got)
	}
	if got := w.heard(); len(got) != 0 {
		t.Errorf("the streams heard %q", got)
	}
	if err := holder.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := w.seq(t); got != 0 {
		t.Errorf("hint_seq = %d, want 0", got)
	}
}

func TestTxTouchBumpsOnceAndSendsNothing(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	err := w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
		h.touch()
		h.touch()
		return nil
	})
	if err != nil {
		t.Fatalf("tx() error = %v", err)
	}
	if got := w.seq(t); got != 1 {
		t.Errorf("hint_seq = %d after a touch, want 1", got)
	}
	if got := w.heard(); len(got) != 0 {
		t.Errorf("a touch sent %q", got)
	}
}

// The number counts transactions, not events and not what they say.
func TestTxBumpsOncePerTransactionWhateverItTold(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	var h2 *hints
	err := w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
		h2 = h
		h.tell(w.campaignID, w.event("public", true))
		h.tell(w.campaignID, w.event("master-only", false))
		return nil
	})
	if err != nil {
		t.Fatalf("tx() error = %v", err)
	}
	if got := w.heard(); !equalNames(got, []string{"public", "master-only"}) {
		t.Errorf("the streams heard %q, want both events in the order told", got)
	}
	if got := w.seq(t); got != 1 {
		t.Errorf("hint_seq = %d after two events in one tx, want 1", got)
	}
	if h2.seq != 1 {
		t.Errorf("the collector holds number %d, want 1", h2.seq)
	}
	// One master-only event bumps the same as one public event.
	err = w.h.svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
		h.tell(w.campaignID, w.event("secret", false))
		return nil
	})
	if err != nil {
		t.Fatalf("tx() error = %v", err)
	}
	if got := w.seq(t); got != 2 {
		t.Errorf("hint_seq = %d, want 2", got)
	}
}

// Four players and the master write one session at the same instant, each its own row, each telling an event, so every
// write adds 1 to the session's number. It measures the cost of that row: the attempts of the closure beyond one per
// write are the retries CockroachDB asked for. The control does the same writes telling nothing. The second pair reads
// the session row before it writes, as a handler that resolves the open session does. Read with
// go test -run TestHintSeqContention -v -count=N.
func TestHintSeqContentionOnCockroachDB(t *testing.T) {
	w := newHintTable(t)
	ctx := t.Context()
	// A statement only CockroachDB has: the contention of its serializable transactions is what is measured.
	var version string
	if err := w.h.pool.QueryRow(ctx, `SHOW CLUSTER SETTING version`).Scan(&version); err != nil {
		t.Fatalf("this test measures CockroachDB: %v", err)
	}
	const writers, perWriter = 5, 100
	// The test pool has one connection, which would make the writers take turns: a pool of their own over the same database.
	cfg := w.h.pool.Config().Copy()
	cfg.MaxConns = writers + 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	svc := &Service{pool: pool, queries: playdb.New(pool), hub: w.h.svc.hub}
	run := func(name string, tell, readsSession bool) {
		var attempts atomic.Int64
		var failures atomic.Int64
		var wg sync.WaitGroup
		before := w.seq(t)
		for id := range writers {
			wg.Go(func() {
				for range perWriter {
					err := svc.tx(ctx, w.sessionID, func(tx pgx.Tx, h *hints) error {
						attempts.Add(1)
						if readsSession { // most handlers read the open session first
							if _, err := svc.queries.WithTx(tx).GetHintSeq(ctx, w.sessionID); err != nil {
								return err
							}
						}
						if err := w.write(ctx, tx, id); err != nil {
							return err
						}
						if tell {
							h.tell(w.campaignID, w.event("write", true))
						}
						return nil
					})
					if err != nil {
						failures.Add(1)
						t.Errorf("%s: writer %d: a write failed after its retries: %v", name, id, err)
					}
				}
			})
		}
		wg.Wait()
		writes := int64(writers * perWriter)
		retries := attempts.Load() - writes
		want := before
		if tell {
			want += writes
		}
		if got := w.seq(t); got != want {
			t.Errorf("%s: hint_seq = %d, want %d", name, got, want)
		}
		t.Logf("%s: %d writes, %d failed, %d retries = %.1f per hundred writes", name, writes, failures.Load(), retries, float64(retries)*100/float64(writes))
		w.heard()
	}
	run("with the bump", true, false)
	run("control, no bump", false, false)
	run("with the bump, session row read first", true, true)
	run("control, no bump, session row read first", false, true)
}
