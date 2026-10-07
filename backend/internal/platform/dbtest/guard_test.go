package dbtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/testenv"
)

// recorder is a testing.TB that keeps what the guard reports instead of
// failing the test that checks the guard.
type recorder struct {
	testing.TB
	mu      sync.Mutex
	errors  []string
	cleanup []func()
}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recorder) Cleanup(f func()) { r.cleanup = append(r.cleanup, f) }

func (r *recorder) found() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errors...)
}

// TestGuardCatchesANestedAcquisition proves the guard every test pool carries
// (see guardPool): a read through the pool inside a db.InTx closure is
// reported with its stack, and a closure that uses only its transaction is not.
// The pool here has two connections, so the stack check is what is tested, not
// the one-connection deadlock (the second line of defense, which takes
// nestedAcquireTimeout). The nested read fails at once, canceled.
func TestGuardCatchesANestedAcquisition(t *testing.T) {
	rawURL := testenv.DatabaseURL(t)
	ctx := t.Context()

	rec := &recorder{TB: t}
	pool, err := db.NewPoolWith(ctx, rawURL, guardPool(rec, 2))
	if err != nil {
		t.Fatalf("NewPoolWith() error = %v", err)
	}
	defer pool.Close()
	if got := pool.Config().MaxConns; got != 2 {
		t.Fatalf("MaxConns = %d, want 2 (%s)", got, poolMaxConnsEnv)
	}

	// Only the transaction: nothing to report.
	if err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		var n int
		return tx.QueryRow(ctx, "SELECT 1").Scan(&n)
	}); err != nil {
		t.Fatalf("InTx() with only the transaction error = %v", err)
	}
	if got := rec.found(); len(got) != 0 {
		t.Fatalf("the guard reported a transaction that stays on its connection: %v", got)
	}

	// A read through the pool inside the closure: reported, and canceled.
	err = db.InTx(ctx, pool, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, "SELECT 1").Scan(&n); err != nil {
			return err
		}
		return pool.QueryRow(ctx, "SELECT 2").Scan(&n) // the second connection
	})
	if err == nil {
		t.Fatal("InTx() with a nested read succeeded, want the read canceled")
	}
	got := rec.found()
	if len(got) != 1 || !strings.Contains(got[0], "nested pool acquisition") || !strings.Contains(got[0], "TestGuardCatchesANestedAcquisition") {
		t.Fatalf("the guard reported %q, want one nested pool acquisition that names this test in its stack", got)
	}

	// After the test ends, a late request must not touch the finished test.
	for _, f := range rec.cleanup {
		f()
	}
	_ = db.InTx(context.Background(), pool, func(_ pgx.Tx) error {
		return pool.QueryRow(context.Background(), "SELECT 3").Scan(new(int))
	})
	if got := rec.found(); len(got) != 1 {
		t.Errorf("a request after the test ended was reported again: %v", got)
	}
}

// TestPoolsHaveOneConnection pins the default size of a test pool.
func TestPoolsHaveOneConnection(t *testing.T) {
	rawURL := testenv.DatabaseURL(t)
	pool, err := db.NewPoolWith(t.Context(), rawURL, guardPool(&recorder{TB: t}, 1))
	if err != nil {
		t.Fatalf("NewPoolWith() error = %v", err)
	}
	defer pool.Close()
	if got := pool.Config().MaxConns; got != 1 {
		t.Errorf("MaxConns = %d, want 1: the guard needs a pool that deadlocks on a nested acquisition", got)
	}
}
