package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTryAcquireRacersNeverFail: runs that send the lock statement together
// all get an answer. One wins and the rest hear "held", even when the database
// aborts a racer with 40001.
func TestTryAcquireRacersNeverFail(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	if err := createLockTable(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	const racers = 12
	var ready, start sync.WaitGroup
	start.Add(1)
	var won atomic.Int32
	var wg sync.WaitGroup
	for i := range racers {
		ready.Add(1)
		wg.Go(func() {
			ready.Done()
			start.Wait()
			got, err := tryAcquire(t.Context(), db, fmt.Sprintf("racer-%d", i), time.Minute)
			if err != nil {
				t.Errorf("tryAcquire() error = %v", err)
				return
			}
			if got {
				won.Add(1)
			}
		})
	}
	ready.Wait()
	start.Done()
	wg.Wait()
	if won.Load() != 1 {
		t.Errorf("%d racers won the lock, want 1", won.Load())
	}
}

// TestReleaseAndRenewAmongPollingWaiters: while other runs poll for the lock
// as fast as they can, the holder's renewals and its release still work: its
// lease is never lost to a 40001 and its release leaves no row to wait out.
func TestReleaseAndRenewAmongPollingWaiters(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	var logs syncBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, release, err := AcquireLock(t.Context(), db, LockOptions{TTL: 600 * time.Millisecond, Poll: time.Millisecond, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := tryAcquire(t.Context(), db, fmt.Sprintf("racer-%d", i), time.Minute); err != nil {
					t.Errorf("tryAcquire() error = %v", err)
					return
				}
			}
		})
	}
	// Several heartbeats go by while the waiters hammer the row.
	time.Sleep(1500 * time.Millisecond)
	if ctx.Err() != nil {
		t.Errorf("the lease was lost while renewing: %v", context.Cause(ctx))
	}
	release()
	close(stop)
	wg.Wait()
	if got := logs.String(); got != "" {
		t.Errorf("the holder logged warnings:\n%s", got)
	}
}

// syncBuffer is a log sink the heartbeat and the test write to together.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
