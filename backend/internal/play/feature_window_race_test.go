package play

import (
	"strconv"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The 30 s auto-pass of the master's screen answers "Deixar passar" with a key made from the window's id (docs/architecture/play/
// reactions.md "The automatic pass"), so it can reach the server at the same time as the player's "Usar": one answer wins and the
// other is refused, and nothing is spent twice. These tests need the database.

// autoPassKey is the key of the master's clock (web core/combat/reaction-autopass.ts): the window's id with every hex digit flipped
// but the version and the variant ones.
func autoPassKey(windowID string) string {
	out := []byte(windowID)
	for i, c := range out {
		if i == 14 || i == 19 {
			continue
		}
		if v, err := strconv.ParseUint(string(c), 16, 8); err == nil {
			out[i] = strconv.FormatUint(15-v, 16)[0]
		}
	}
	return string(out)
}

// autoPass is the answer the master's clock sends.
func autoPass(windowID string) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		passAnswer(r)
		r.IdempotencyKey = autoPassKey(windowID)
	}
}

// settled checks what the two answers left: the uses and the reaction of Pensantus, and the lines of the use in the log.
func (m *featureTable) settled(t *testing.T, e *playv1.Encounter, used bool) {
	t.Helper()
	wantLeft, wantLines := int32(2), 0
	if used {
		wantLeft, wantLines = 1, 1
	}
	if got := m.row(t, m.ana, m.get(t, m.ana), "Pensantus", "Halo").GetUseLeft(); got != wantLeft {
		t.Errorf("Halo's uses left = %d, want %d", got, wantLeft)
	}
	if got := m.pensantus(t).GetReactionUsed(); got != used {
		t.Errorf("Pensantus's reaction used = %v, want %v", got, used)
	}
	if lines := logLinesOf(t, m.armed, m.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != wantLines {
		t.Errorf("the log has %d FEATURE_USED lines, want %d", len(lines), wantLines)
	}
	if m.windowOf(t, m.master, kindFeature) != nil {
		t.Errorf("the window stands after the answers")
	}
}

// The player's "Usar" came first: the auto-pass that follows is refused and spends nothing more.
func TestTheAutoPassAfterAUseOfTheFeatureWindowIsRefused(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, w := m.haloWindow(t)
	m.mustAnswer(t, m.ana, e, w.GetId(), useAnswer(0))
	_, err := m.answerReaction(t, m.master, e, w.GetId(), autoPass(w.GetId()))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the auto-pass after the use: error = %v, want failed precondition", err)
	}
	m.settled(t, e, true)
}

// The auto-pass came first: the player's "Usar" is refused and nothing is spent.
func TestAUseOfTheFeatureWindowAfterTheAutoPassIsRefused(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, w := m.haloWindow(t)
	m.mustAnswer(t, m.master, e, w.GetId(), autoPass(w.GetId()))
	_, err := m.answerReaction(t, m.ana, e, w.GetId(), useAnswer(0))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the use after the auto-pass: error = %v, want failed precondition", err)
	}
	m.settled(t, e, false)
}

// Both at once: exactly one answer wins, whichever it is, and what it spent is spent once.
func TestTheAutoPassAndAUseOfTheFeatureWindowRace(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 2)
	m := haloTable(t)
	e, w := m.haloWindow(t)
	var errs [2]error
	var wg sync.WaitGroup
	wg.Go(func() { _, errs[0] = m.answerReaction(t, m.ana, e, w.GetId(), useAnswer(0)) })
	wg.Go(func() { _, errs[1] = m.answerReaction(t, m.master, e, w.GetId(), autoPass(w.GetId())) })
	wg.Wait()
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("the loser: error = %v, want failed precondition", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d of two answers won, want exactly one (%v)", won, errs)
	}
	m.settled(t, e, errs[0] == nil)
}

// The combat ends while the window waits: the window goes with it, quietly. Nothing is held, spent or written, and nobody waits.
func TestTheEndOfTheCombatDiscardsTheFeatureWindow(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, w := m.haloWindow(t)
	m.endEncounter(t, e)
	var rows int
	if err := m.h.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM reaction_windows) + (SELECT count(*) FROM reaction_holds)`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("rows left = %d, %v; want none", rows, err)
	}
	for name, u := range map[string]*user{"Ana": m.ana, "the master": m.master} {
		if got := m.get(t, u).GetReactionWait(); got != nil {
			t.Errorf("%s still reads a wait after the combat ended: %v", name, got)
		}
	}
	for _, answer := range []func(*playv1.AnswerReactionRequest){useAnswer(0), passAnswer} {
		if _, err := m.answerReaction(t, m.ana, e, w.GetId(), answer); err == nil {
			t.Error("a window of a finished combat was answered")
		}
	}
	if m.pensantus(t).GetReactionUsed() {
		t.Errorf("Pensantus's reaction is spent by a combat that ended")
	}
	if lines := logLinesOf(t, m.armed, m.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED); len(lines) != 0 {
		t.Errorf("the log has %v, want no FEATURE_USED", lines)
	}
}
