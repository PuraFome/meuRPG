package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// These tests exercise the retry loop without a database: fakeConn hands out
// a fakeTx that records the SQL the helper sends. The integration tests in
// db_integration_test.go cover the same behavior against real CockroachDB.

// fakeTx implements pgx.Tx. Embedding the interface satisfies the methods we
// don't override; calling one of those panics, which is what we want in a
// test because it means the helper did something unexpected.
type fakeTx struct {
	pgx.Tx

	statements []string
	committed  bool
	rolledBack bool
}

func (f *fakeTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.statements = append(f.statements, sql)
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

type fakeConn struct{ tx *fakeTx }

func (c *fakeConn) Begin(context.Context) (pgx.Tx, error) { return c.tx, nil }

func (c *fakeConn) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) { return c.tx, nil }

// errSerializationFailure is the error pgx returns when CockroachDB aborts a
// transaction that must be retried.
var errSerializationFailure = &pgconn.PgError{Code: "40001", Message: "restart transaction"}

func TestInTxRetriesSerializationFailures(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{tx: &fakeTx{}}
	attempts := 0

	err := InTx(t.Context(), conn, func(pgx.Tx) error {
		attempts++
		if attempts < 3 {
			// Wrapped on purpose: the helper must look through %w.
			return fmt.Errorf("update character: %w", errSerializationFailure)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx() error = %v, want nil", err)
	}

	if attempts != 3 {
		t.Errorf("fn ran %d times, want 3", attempts)
	}
	if !conn.tx.committed || conn.tx.rolledBack {
		t.Errorf("committed = %v, rolledBack = %v; want committed only", conn.tx.committed, conn.tx.rolledBack)
	}
	// Each retry starts over; the last statement makes the work durable.
	if last := conn.tx.statements[len(conn.tx.statements)-1]; last != "RELEASE SAVEPOINT cockroach_restart" {
		t.Errorf("last statement = %q, want the savepoint release", last)
	}
}

func TestInTxDoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{tx: &fakeTx{}}
	errNotFound := errors.New("character not found")
	attempts := 0

	err := InTx(t.Context(), conn, func(pgx.Tx) error {
		attempts++
		return errNotFound
	})

	if !errors.Is(err, errNotFound) {
		t.Fatalf("InTx() error = %v, want %v", err, errNotFound)
	}
	if attempts != 1 {
		t.Errorf("fn ran %d times, want 1", attempts)
	}
	if !conn.tx.rolledBack || conn.tx.committed {
		t.Errorf("committed = %v, rolledBack = %v; want rolled back only", conn.tx.committed, conn.tx.rolledBack)
	}
}

func TestInTxGivesUpAfterRetryLimit(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{tx: &fakeTx{}}
	policy := &crdb.LimitBackoffRetryPolicy{RetryLimit: 2} // no delay, keeps the test fast
	attempts := 0

	err := inTx(t.Context(), conn, policy, func(pgx.Tx) error {
		attempts++
		return errSerializationFailure
	})

	if _, ok := errors.AsType[*crdb.MaxRetriesExceededError](err); !ok {
		t.Fatalf("InTx() error = %v, want *crdb.MaxRetriesExceededError", err)
	}
	if !errors.Is(err, errSerializationFailure) {
		t.Errorf("InTx() error = %v, want it to wrap the original 40001", err)
	}
	if attempts != 3 { // the first try + 2 retries
		t.Errorf("fn ran %d times, want 3", attempts)
	}
	if !conn.tx.rolledBack {
		t.Error("transaction was not rolled back")
	}
}

func TestDefaultRetryPolicyBacksOff(t *testing.T) {
	t.Parallel()

	next := defaultRetryPolicy.NewRetry()
	var delays []int64
	for {
		delay, err := next(errSerializationFailure)
		if err != nil {
			break
		}
		delays = append(delays, delay.Milliseconds())
	}

	want := []int64{10, 20, 40, 80, 160, 320, 500, 500}
	if !slices.Equal(delays, want) {
		t.Errorf("retry delays (ms) = %v, want %v", delays, want)
	}
}
