package play

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// A write that waits for the session's lock reads the characters after the lock, not before it (lockedCharacters): what changed in
// them while it waited is what it decides on. The tests here are the shape of TestCancelOfAnAskReadsTheOwnerUnderTheSessionLock for
// the ask (the giver's owner), the grant and the accept (the receiver in the reserve).

// lateHolder is a transaction starter that takes the session's row in a transaction of the test's, once, just before the first
// transaction of the call begins. A call through the RPC reads the session through the pool before its transaction (the plain read
// waits for the lock on CockroachDB, and the transaction then starts on a fresh snapshot): the holder taking the row after that read and
// before the transaction's own lock is what a write that loses the race to another write has, and it is made here without a clock.
type lateHolder struct {
	pool *pgxpool.Pool
	hold func()
	once sync.Once
}

func (l *lateHolder) Begin(ctx context.Context) (pgx.Tx, error) {
	return l.BeginTx(ctx, pgx.TxOptions{})
}

func (l *lateHolder) BeginTx(ctx context.Context, o pgx.TxOptions) (pgx.Tx, error) {
	l.once.Do(l.hold)
	return l.pool.BeginTx(ctx, o)
}

// whileTheSessionIsHeld holds the session's row in a transaction of its own, starts call and waits, on a condition and not on a clock,
// until call is stopped at the session's lock. Then change runs on a connection of its own (what it writes commits at once), and the
// transaction that held the session writes it and commits. It returns what call returned. call is a request through the RPC, which
// reads the session before it begins, so the row is held before it starts (a call that waits at that read starts its transaction after
// the holder's commit, and has nothing to test); a call that opens its transaction with the starter it is given has the row taken
// just before the transaction begins: pass early = false.
func (f *favorParty) whileTheSessionIsHeld(t *testing.T, early bool, call func(starter db.TxStarter) error, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	ctx := t.Context()
	side := dbtest.SideConnection(t, f.h.pool)
	holder, err := side.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = holder.Rollback(t.Context()) }()
	lock := func() {
		if _, err := holder.Exec(ctx, `SELECT id FROM game_sessions WHERE id = $1 FOR UPDATE`, f.sessionID); err != nil {
			t.Errorf("lock the session: %v", err)
		}
	}
	starter := &lateHolder{pool: f.h.pool, hold: lock}
	if early {
		lock()
		starter.hold = func() {}
	}
	done := make(chan error, 1)
	go func() { done <- call(starter) }()
	observer := dbtest.SideConnection(t, f.h.pool)
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		n, err := dbtest.WaitingForTable(ctx, observer, "game_sessions")
		if err != nil {
			t.Fatalf("look for the waiting call: %v", err)
		}
		if n > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the call ended (%v) while the session's row was held", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the call never waited for the session's row")
		}
	}
	change(ctx, side)
	if _, err := holder.Exec(ctx, `UPDATE game_sessions SET hint_seq = hint_seq + 1 WHERE id = $1`, f.sessionID); err != nil {
		t.Fatalf("write the session: %v", err)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	return <-done
}

// execAll runs the statements (the SQL and its arguments), in order, on the side connection.
func execAll(ctx context.Context, t *testing.T, side *pgxpool.Pool, stmts ...[2]any) {
	t.Helper()
	for _, s := range stmts {
		if _, err := side.Exec(ctx, s[0].(string), s[1].([]any)...); err != nil {
			t.Fatalf("change the characters: %v", err)
		}
	}
}

// An ask by Toren's owner that waited for the session while Toren changed hands is the old owner's no more: it is refused as any ask of
// a giver that is not the caller's, and nothing is written.
func TestAskReadsTheGiversOwnerAfterTheSessionLock(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	w := f.world(t)
	m := authz.Membership{CampaignID: f.campaignID, UserID: f.caio.id, Role: authz.RolePlayer}
	call := favorPassCall{m: m, sessionID: f.sessionID, from: f.toren.GetId(), to: f.pens.GetId(), key: newKey()}
	err := f.whileTheSessionIsHeld(t, false, func(starter db.TxStarter) error {
		return f.h.svc.txOn(t.Context(), starter, f.sessionID, func(tx pgx.Tx, h *hints) error {
			_, err := f.h.svc.askPassIn(t.Context(), tx, h, call)
			return err
		})
	}, func(ctx context.Context, side *pgxpool.Pool) {
		// Bia gives up Brisa (she may own one living character) and takes Toren over from Caio.
		execAll(ctx, t, side,
			[2]any{`UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, []any{f.bri.GetId()}},
			[2]any{`UPDATE characters SET player_user_id = $2 WHERE id = $1`, []any{f.toren.GetId(), f.bia.id}})
	})
	wantFavorBlocked(t, "the old owner's ask, which waited for the session", err, blockedNotTheHolder)
	w.seq++ // the write that held the session moved the number, and it is the only one
	f.wantSame(t, "the old owner's ask", w)
}

// A grant that waited for the session while its receiver was sent to the reserve is refused as a grant to a reserved character, and
// the Inspiration is not given.
func TestGrantReadsTheReceiverAfterTheSessionLock(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	w := f.world(t)
	granted := f.grantedEvents(t)
	g := favorGrant{m: authz.Membership{CampaignID: f.campaignID, UserID: f.master.id, Role: authz.RoleMaster}, sessionID: f.sessionID, characterID: f.pens.GetId(), key: newKey()}
	err := f.whileTheSessionIsHeld(t, false, func(starter db.TxStarter) error {
		return f.h.svc.txOn(t.Context(), starter, f.sessionID, func(tx pgx.Tx, h *hints) error { return f.h.svc.grantFavorIn(t.Context(), tx, h, g) })
	}, func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, []any{f.pens.GetId()}})
	})
	wantFavorBlocked(t, "the grant to a receiver sent to the reserve while it waited", err, blockedNotAPlayer)
	w.seq++ // the write that held the session moved the number, and it is the only one
	f.wantSame(t, "the grant", w)
	if got := f.grantedEvents(t); got != granted {
		t.Errorf("%d favor_granted events, want %d", got, granted)
	}
}

// An accept that waited for the session while the receiver was sent to the reserve is refused as an accept to a receiver that left the
// table: CANNOT_RECEIVE, the pass stays pending and the Inspiration stays with the giver.
func TestAcceptReadsTheReceiverAfterTheSessionLock(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	w := f.world(t)
	err := f.whileTheSessionIsHeld(t, true, func(db.TxStarter) error {
		_, err := f.answer(p.GetId(), true)
		return err
	}, func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, []any{f.pens.GetId()}})
	})
	wantFavorBlocked(t, "the accept, which waited for the session", err, blockedCannotReceive)
	w.seq++ // the write that held the session moved the number, and it is the only one
	f.wantSame(t, "the accept", w)
	if s := f.passStatus(t, p.GetId()); s != "pending" || !f.has(t, f.toren.GetId()) || f.has(t, f.pens.GetId()) {
		t.Errorf("pass %q, Toren has it %v, Pensantus %v, want it pending and unmoved", s, f.has(t, f.toren.GetId()), f.has(t, f.pens.GetId()))
	}
}
