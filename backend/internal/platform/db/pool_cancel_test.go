package db

import (
	"context"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestACanceledCallLeavesNoDeadlineOnThePool: calls whose contexts are canceled at random moments, mid-query
// or before it, never make a later call with a live context fail with a timeout (issue 710).
func TestACanceledCallLeavesNoDeadlineOnThePool(t *testing.T) {
	t.Parallel()
	fake := newFakePG(t, 2*time.Millisecond)
	pool, err := NewPool(t.Context(), fake.URL())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	var bad sync.Map
	var wg sync.WaitGroup
	for w := range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 7)) //nolint:gosec // seeded test jitter
			for i := range 400 {
				if i%100 == 99 {
					time.Sleep(1100 * time.Millisecond) // idle long enough for the pool to ping on acquire
				}
				// A call that is canceled at a random moment...
				ctx, cancel := context.WithCancel(context.Background())
				switch r.IntN(3) {
				case 0:
					go func(d time.Duration) { time.Sleep(d); cancel() }(time.Duration(r.IntN(12000)) * time.Microsecond)
				case 1:
					ctx, cancel = context.WithTimeout(context.Background(), time.Duration(r.IntN(4000))*time.Microsecond)
				default:
					cancel() // canceled before the call
				}
				var n int
				_ = pool.QueryRow(ctx, "SELECT 1").Scan(&n)
				cancel()
				// ...and a call that is not, which must work.
				var m int
				if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&m); err != nil {
					bad.Store(err.Error(), true)
				}
			}
		}()
	}
	wg.Wait()
	bad.Range(func(k, _ any) bool {
		t.Errorf("a call with a live context failed: %v", k)
		return true
	})
}

// TestACanceledSlowWriteLeavesNoStaleErrorOnTheConnection: a write that takes longer than 15 ms starts pgx's
// background reader; a call canceled around then must not leave its read error for the next call on the
// connection (issue 710).
func TestACanceledSlowWriteLeavesNoStaleErrorOnTheConnection(t *testing.T) {
	t.Parallel()
	fake := newFakePG(t, time.Millisecond)
	fake.readDelay = 20 * time.Millisecond
	pool, err := NewPoolWith(t.Context(), fake.URL(), func(c *pgxpool.Config) { c.MaxConns = 4 })
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	big := strings.Repeat("x", 4<<20)

	var bad sync.Map
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 9)) //nolint:gosec // seeded test jitter
			for range 60 {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10+r.IntN(60))*time.Millisecond)
				var s string
				_ = pool.QueryRow(ctx, "SELECT $1::text", big).Scan(&s)
				cancel()
				var m int
				if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&m); err != nil {
					bad.Store(err.Error(), true)
				}
			}
		}()
	}
	wg.Wait()
	bad.Range(func(k, _ any) bool {
		t.Errorf("a call with a live context failed: %v", k)
		return true
	})
}

// slowWriteConn is a connection whose writes return 16 ms late, longer than pgx's write-read deadlock timer
// (15 ms), the way a write does when the machine is loaded and the goroutine waits to run again.
type slowWriteConn struct {
	net.Conn
	d time.Duration
}

func (c slowWriteConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	time.Sleep(c.d)
	return n, err
}

// TestACallCanceledAsItEndsLeavesNoStaleErrorOnTheConnection: pgx starts a background reader when a write
// takes 15 ms, and the reader may be waiting for more when the call ends; a context canceled just then makes
// that read fail, and the error must not wait on the connection for the next call (issue 710).
func TestACallCanceledAsItEndsLeavesNoStaleErrorOnTheConnection(t *testing.T) {
	t.Parallel()
	fake := newFakePG(t, 0)
	pool, err := NewPoolWith(t.Context(), fake.URL(), func(c *pgxpool.Config) {
		c.MaxConns = 4
		dial := c.ConnConfig.DialFunc
		c.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dial(ctx, network, addr)
			return slowWriteConn{conn, 16 * time.Millisecond}, err
		}
	})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	var bad sync.Map
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 11)) //nolint:gosec // seeded test jitter
			for range 250 {
				ctx, cancel := context.WithTimeout(context.Background(), 16*time.Millisecond+time.Duration(r.IntN(2500))*time.Microsecond)
				var n int
				_ = pool.QueryRow(ctx, "SELECT 1").Scan(&n)
				cancel()
				var m int
				if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&m); err != nil {
					bad.Store(err.Error(), true)
				}
			}
		}()
	}
	wg.Wait()
	bad.Range(func(k, _ any) bool {
		t.Errorf("a call with a live context failed: %v", k)
		return true
	})
}
