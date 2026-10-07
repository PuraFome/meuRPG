package migrations

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// serializeUpTo is how many migrations the two runs of TestMigrateUpRunsSerialize
// share: a few are enough to show that the second run waits and then finds
// nothing to do, and each one costs seconds of DDL.
const serializeUpTo = 8

// TestMigrateUpRunsSerialize: two `migrate up` runs started together, as two
// deploys at once, never overlap. Each takes the lock, runs the first real migrations
// and releases it; the second finds nothing left to apply.
func TestMigrateUpRunsSerialize(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)

	var inside, overlaps atomic.Int32
	var applied atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			ctx, release, err := AcquireLock(t.Context(), db, LockOptions{TTL: 6 * time.Second, Poll: 50 * time.Millisecond})
			if err != nil {
				t.Errorf("AcquireLock() error = %v", err)
				return
			}
			defer release()
			if inside.Add(1) > 1 {
				overlaps.Add(1)
			}
			defer inside.Add(-1)
			provider, err := NewProvider(db)
			if err != nil {
				t.Errorf("NewProvider() error = %v", err)
				return
			}
			results, err := provider.UpTo(ctx, serializeUpTo)
			if err != nil {
				t.Errorf("Up() error = %v", err)
				return
			}
			applied.Add(int32(len(results))) //nolint:gosec // G115: a few hundred migrations
		})
	}
	wg.Wait()

	if overlaps.Load() != 0 {
		t.Errorf("%d runs held the lock while another one did", overlaps.Load())
	}
	if got, want := int(applied.Load()), serializeUpTo; got != want {
		t.Errorf("the two runs applied %d migrations in all, want %d (each once)", got, want)
	}
	var rows int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM migration_lock`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("migration_lock has %d rows after both runs, want 0 (released)", rows)
	}
}

// TestLockTimesOutWithAClearError: while a live run holds the lock, a second one
// waits for its Wait and then says so.
func TestLockTimesOutWithAClearError(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	_, release, err := AcquireLock(t.Context(), db, LockOptions{TTL: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, _, err = AcquireLock(t.Context(), db, LockOptions{TTL: 30 * time.Second, Wait: 300 * time.Millisecond, Poll: 50 * time.Millisecond})
	if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("second AcquireLock() error = %v, want ErrLockTimeout", err)
	}
}

// TestLockOfACrashedRunExpires: a run that died leaves its row; once the lease
// has passed, the next run takes over without anyone cleaning up by hand.
func TestLockOfACrashedRunExpires(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	// A first run that never heartbeats nor releases: a short lease, no release call.
	if err := createLockTable(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if got, err := tryAcquire(t.Context(), db, "crashed", 300*time.Millisecond); err != nil || !got {
		t.Fatalf("tryAcquire(crashed) = %v, %v; want true", got, err)
	}
	if got, _ := tryAcquire(t.Context(), db, "second", time.Minute); got {
		t.Fatal("a live lease was taken over")
	}

	_, release, err := AcquireLock(t.Context(), db, LockOptions{TTL: time.Minute, Wait: 10 * time.Second, Poll: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("AcquireLock() after the lease expired error = %v", err)
	}
	release()
}

// TestLostLeaseStopsTheRun: when another holder has the row, the heartbeat finds
// out and cancels the run's context, so goose stops between statements.
func TestLostLeaseStopsTheRun(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx, release, err := AcquireLock(t.Context(), db, LockOptions{TTL: 600 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.ExecContext(t.Context(), `UPDATE migration_lock SET holder = 'someone else'`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
		if cause := context.Cause(ctx); cause == nil || errors.Is(cause, context.Canceled) {
			t.Errorf("cause = %v, want the lost-lease reason", cause)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run was not stopped after losing the lease")
	}
}
