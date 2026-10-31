package db

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	fake := newFakePG(t)
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
			for i := range 250 {
				// 16 ms plus 0 to 2.5 ms, spread over the calls and the workers.
				spread := time.Duration((w*251+i)*997%2500) * time.Microsecond
				ctx, cancel := context.WithTimeout(context.Background(), 16*time.Millisecond+spread)
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
