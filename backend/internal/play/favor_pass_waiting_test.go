package play

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// A write that waits for the session's lock reads the characters after the lock (LockedSessionCharacters), so it decides on what changed in
// them while it waited: the giver's owner of an ask, the receiver of a grant, an accept, a refund.

// lateHolder is a transaction starter that takes the session's row in a transaction of the test's, once, just before the first
// transaction of the call begins. A write through the RPC that reads the session through the pool before its transaction waits there
// on CockroachDB, and its transaction then starts on a fresh snapshot: nothing to test. The holder that takes the row after that read
// and before the transaction's own lock is the race a write has with another write, made here without a clock.
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

// heldBeforeTheCall runs call, a request through the RPC that does not read the session before it begins its transaction, while the
// session's row is held by a transaction of the test's, and waits, on a condition and not on a clock, until call is stopped at the lock.
// Then change runs on a connection of its own (what it writes commits at once), and the transaction that held the session writes it and
// commits. It returns what call returned.
func (f *favorParty) heldBeforeTheCall(t *testing.T, call func() error, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	return f.held(t, true, func(db.TxStarter) error { return call() }, change)
}

// heldBeforeTheTransaction is heldBeforeTheCall for a call that opens its transaction with the starter it is given (the RPC reads the
// session first): the row is taken just before the transaction begins.
func (f *favorParty) heldBeforeTheTransaction(t *testing.T, call func(starter db.TxStarter) error, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	return f.held(t, false, call, change)
}

func (f *favorParty) held(t *testing.T, early bool, call func(starter db.TxStarter) error, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	ctx := t.Context()
	side := dbtest.SideConnection(t, f.h.pool)
	holder, err := side.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	done := make(chan error, 1)
	started, answered := false, false
	defer func() {
		_ = holder.Rollback(t.Context())
		if started && !answered { // a Fatal below: the call is let go by the rollback, and ends before the test does
			<-done
		}
	}()
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
	started = true
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
			answered = true
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
	answered = true
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
	err := f.heldBeforeTheTransaction(t, func(starter db.TxStarter) error {
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
	err := f.heldBeforeTheTransaction(t, func(starter db.TxStarter) error {
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
	err := f.heldBeforeTheCall(t, func() error {
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

// A give-back that waited for the session while the roller was sent to the reserve is refused as a give-back to a roller in the
// reserve: CANNOT_RECEIVE, the question stays open and nothing is given back.
func TestRefundReadsTheRollerAfterTheSessionLock(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	w := f.world(t)
	err := f.heldBeforeTheCall(t, func() error {
		_, err := f.answerRefund(f.master, p.GetId(), true, newKey())
		return err
	}, func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, []any{f.toren.GetId()}})
	})
	wantFavorBlocked(t, "the give-back, which waited for the session", err, blockedCannotReceive)
	w.seq++ // the write that held the session moved the number, and it is the only one
	f.wantSame(t, "the give-back", w)
	if status := f.promptStatus(t, p.GetId()); status != "open" {
		t.Errorf("the prompt is %q, want open", status)
	}
	if f.rows(t) != 0 {
		t.Errorf("character_favor has %d rows, want 0", f.rows(t))
	}
}

// toTheReserve is the statement that sends a character to the reserve, as the master does.
func toTheReserve(id string) [2]any {
	return [2]any{`UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, []any{id}}
}

// handedToBia changes the owner of a character to Bia: she gives up Brisa first (a player may own one living character).
func (f *favorParty) handedToBia(ctx context.Context, t *testing.T, side *pgxpool.Pool, id string) {
	t.Helper()
	execAll(ctx, t, side, toTheReserve(f.bri.GetId()), [2]any{`UPDATE characters SET player_user_id = $2 WHERE id = $1`, []any{id, f.bia.id}})
}

// closedMeanwhile is the statement of the master's answer (or the end of the session) closing a pass.
func closedMeanwhile(id, status string) [2]any {
	return [2]any{`UPDATE favor_passes SET status = $2, closed_at = now() WHERE id = $1`, []any{id, status}}
}

// askedByCaio is a pass of Toren, who has the Inspiration, to Pensantus, asked by Caio, and the world before the call that waits.
func (f *favorParty) askedByCaio(t *testing.T) *playv1.FavorPass {
	t.Helper()
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	return f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
}

// cancelWaiting runs Caio's cancel of the pass while the session's row is held and change runs; it returns what the cancel returned.
func (f *favorParty) cancelWaiting(t *testing.T, id string, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	return f.heldBeforeTheCall(t, func() error {
		_, err := f.cancelAs(f.caio, id, newKey())
		return err
	}, change)
}

// acceptWaiting is cancelWaiting for the master's accept.
func (f *favorParty) acceptWaiting(t *testing.T, id string, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	return f.heldBeforeTheCall(t, func() error {
		_, err := f.answer(id, true)
		return err
	}, change)
}

func TestCancelOfAGiverSentToTheReserveWhileItWaitedIsNotFoundAndTheAskStaysPending(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	w := f.world(t)
	err := f.cancelWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) { execAll(ctx, t, side, toTheReserve(f.toren.GetId())) })
	wantCode(t, "the cancel of a giver sent to the reserve", err, connect.CodeNotFound)
	w.seq++
	f.wantSame(t, "the cancel", w)
	if s := f.passStatus(t, p.GetId()); s != "pending" {
		t.Errorf("the ask is %q, want pending", s)
	}
}

func TestCancelOfAnAskTheMasterRefusedWhileItWaitedIsPassClosed(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	err := f.cancelWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, closedMeanwhile(p.GetId(), "refused"))
	})
	wantFavorBlocked(t, "the cancel of an ask refused meanwhile", err, blockedPassClosed)
	if s := f.passStatus(t, p.GetId()); s != "refused" || f.eventsOf(t, eventFavorPassCanceled) != 0 {
		t.Errorf("the ask is %q with %d cancel events, want refused and none", s, f.eventsOf(t, eventFavorPassCanceled))
	}
}

func TestCancelOfAnAskWhoseRowWentWhileItWaitedIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	err := f.cancelWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`DELETE FROM favor_passes WHERE id = $1`, []any{p.GetId()}})
	})
	wantCode(t, "the cancel of an ask that went", err, connect.CodeNotFound)
	if f.eventsOf(t, eventFavorPassCanceled) != 0 {
		t.Errorf("a cancel event was written")
	}
}

// askWaiting runs Caio's ask of Toren to Pensantus while the session's row is held and change runs.
func (f *favorParty) askWaiting(t *testing.T, change func(ctx context.Context, side *pgxpool.Pool)) error {
	t.Helper()
	call := favorPassCall{m: authz.Membership{CampaignID: f.campaignID, UserID: f.caio.id, Role: authz.RolePlayer}, sessionID: f.sessionID, from: f.toren.GetId(), to: f.pens.GetId(), key: newKey()}
	return f.heldBeforeTheTransaction(t, func(starter db.TxStarter) error {
		return f.h.svc.txOn(t.Context(), starter, f.sessionID, func(tx pgx.Tx, h *hints) error {
			_, err := f.h.svc.askPassIn(t.Context(), tx, h, call)
			return err
		})
	}, change)
}

func TestAskOfAGiverSentToTheReserveWhileItWaitedIsNotTheHolderAndWritesNothing(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	w := f.world(t)
	err := f.askWaiting(t, func(ctx context.Context, side *pgxpool.Pool) { execAll(ctx, t, side, toTheReserve(f.toren.GetId())) })
	wantFavorBlocked(t, "the ask of a giver sent to the reserve", err, blockedNotTheHolder)
	w.seq++
	f.wantSame(t, "the ask", w)
}

func TestAskToAReceiverSentToTheReserveWhileItWaitedIsNotAPlayerCharacterAndWritesNothing(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	w := f.world(t)
	err := f.askWaiting(t, func(ctx context.Context, side *pgxpool.Pool) { execAll(ctx, t, side, toTheReserve(f.pens.GetId())) })
	wantFavorBlocked(t, "the ask to a receiver sent to the reserve", err, blockedNotAPlayer)
	w.seq++
	f.wantSame(t, "the ask", w)
}

func TestGrantToAReceiverWhoChangedOwnerWhileItWaitedGoesThrough(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	g := favorGrant{m: authz.Membership{CampaignID: f.campaignID, UserID: f.master.id, Role: authz.RoleMaster}, sessionID: f.sessionID, characterID: f.pens.GetId(), key: newKey()}
	err := f.heldBeforeTheTransaction(t, func(starter db.TxStarter) error {
		return f.h.svc.txOn(t.Context(), starter, f.sessionID, func(tx pgx.Tx, h *hints) error { return f.h.svc.grantFavorIn(t.Context(), tx, h, g) })
	}, func(ctx context.Context, side *pgxpool.Pool) { f.handedToBia(ctx, t, side, f.pens.GetId()) })
	if err != nil {
		t.Fatalf("the grant error = %v, want none: it reads no owner", err)
	}
	if !f.has(t, f.pens.GetId()) {
		t.Error("Pensantus has no Inspiration, want it granted")
	}
}

func TestAcceptOfAGiverWhoChangedOwnerWhileItWaitedGoesThrough(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	err := f.acceptWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) { f.handedToBia(ctx, t, side, f.toren.GetId()) })
	if err != nil {
		t.Fatalf("the accept error = %v, want none", err)
	}
	if s := f.passStatus(t, p.GetId()); s != "accepted" || f.has(t, f.toren.GetId()) || !f.has(t, f.pens.GetId()) {
		t.Errorf("pass %q, Toren has it %v, Pensantus %v, want accepted and moved", s, f.has(t, f.toren.GetId()), f.has(t, f.pens.GetId()))
	}
}

func TestAcceptOfAGiverSentToTheReserveWhileItWaitedIsNotTheHolderAndNothingMoves(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	w := f.world(t)
	err := f.acceptWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) { execAll(ctx, t, side, toTheReserve(f.toren.GetId())) })
	wantFavorBlocked(t, "the accept of a giver sent to the reserve", err, blockedNotTheHolder)
	w.seq++
	f.wantSame(t, "the accept", w)
	if s := f.passStatus(t, p.GetId()); s != "pending" {
		t.Errorf("the ask is %q, want pending", s)
	}
}

func TestAcceptOfAnAskCanceledWhileItWaitedIsPassClosedAndNothingMoves(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	err := f.acceptWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, closedMeanwhile(p.GetId(), "canceled"))
	})
	wantFavorBlocked(t, "the accept of an ask canceled meanwhile", err, blockedPassClosed)
	if s := f.passStatus(t, p.GetId()); s != "canceled" || !f.has(t, f.toren.GetId()) || f.has(t, f.pens.GetId()) || f.eventsOf(t, eventFavorPassed) != 0 {
		t.Errorf("pass %q, Toren has it %v, Pensantus %v, want canceled and unmoved", s, f.has(t, f.toren.GetId()), f.has(t, f.pens.GetId()))
	}
}

func TestAcceptOfAnAskWhoseRowWentWhileItWaitedIsNotFoundAndNothingMoves(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	p := f.askedByCaio(t)
	err := f.acceptWaiting(t, p.GetId(), func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`DELETE FROM favor_passes WHERE id = $1`, []any{p.GetId()}})
	})
	wantCode(t, "the accept of an ask that went", err, connect.CodeNotFound)
	if !f.has(t, f.toren.GetId()) || f.has(t, f.pens.GetId()) || f.eventsOf(t, eventFavorPassed) != 0 {
		t.Errorf("Toren has it %v, Pensantus %v, want it unmoved", f.has(t, f.toren.GetId()), f.has(t, f.pens.GetId()))
	}
}

func TestRefundGiveBackToARollerWhoChangedOwnerWhileItWaitedGoesThrough(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	err := f.heldBeforeTheCall(t, func() error {
		_, err := f.answerRefund(f.master, p.GetId(), true, newKey())
		return err
	}, func(ctx context.Context, side *pgxpool.Pool) { f.handedToBia(ctx, t, side, f.toren.GetId()) })
	if err != nil {
		t.Fatalf("the give-back error = %v, want none", err)
	}
	if status := f.promptStatus(t, p.GetId()); status != "returned" || !f.has(t, f.toren.GetId()) {
		t.Errorf("the prompt is %q, Toren has it %v, want returned and given back", status, f.has(t, f.toren.GetId()))
	}
}

func TestRefundGiveBackWhoseQuestionWentWhileItWaitedIsNotFound(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	err := f.heldBeforeTheCall(t, func() error {
		_, err := f.answerRefund(f.master, p.GetId(), true, newKey())
		return err
	}, func(ctx context.Context, side *pgxpool.Pool) {
		execAll(ctx, t, side, [2]any{`DELETE FROM favor_refund_prompts WHERE id = $1`, []any{p.GetId()}})
	})
	wantCode(t, "the give-back of a question that went", err, connect.CodeNotFound)
	if f.rows(t) != 0 {
		t.Errorf("character_favor has %d rows, want 0", f.rows(t))
	}
}

// The check of an ask's pair holds the rows of both characters until the transaction ends: a write on either row, from another
// connection, waits for it (FOR UPDATE in LockSessionCharacters; on PostgreSQL a plain read sees what changed meanwhile, so only the wait proves it).
func TestLockPairHoldsBothCharactersRowsUntilTheTransactionEnds(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	ctx := t.Context()
	side := dbtest.SideConnection(t, f.h.pool)
	observer := dbtest.SideConnection(t, f.h.pool)
	ids := []string{f.toren.GetId(), f.pens.GetId()}
	done := make(chan error, len(ids))
	ended := 0
	err := f.h.svc.txOn(ctx, f.h.pool, f.sessionID, func(tx pgx.Tx, h *hints) error {
		if err := f.h.svc.lockPair(ctx, tx, f.campaignID, ids[0], ids[1]); err != nil {
			return err
		}
		for _, id := range ids {
			go func() {
				var err error
				for { // the isolation level may abort the write that waited: it is retried, as a client does
					_, err = side.Exec(ctx, `UPDATE characters SET updated_at = now() WHERE id = $1`, id)
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
						break
					}
				}
				done <- err
			}()
		}
		for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(5 * time.Millisecond) {
			n, err := dbtest.WaitingForTable(ctx, observer, "characters")
			if err != nil {
				return err
			}
			if n >= len(ids) {
				return nil
			}
			select {
			case err := <-done:
				ended++
				t.Errorf("a write on a locked character ended (%v) before the transaction did", err)
				return nil
			default:
			}
			if time.Now().After(deadline) {
				t.Error("the writes on the characters' rows did not both wait")
				return nil
			}
		}
	})
	if err != nil {
		t.Fatalf("the transaction error = %v", err)
	}
	for ; ended < len(ids); ended++ {
		if err := <-done; err != nil {
			t.Errorf("the write on a character's row after the transaction: %v", err)
		}
	}
}
