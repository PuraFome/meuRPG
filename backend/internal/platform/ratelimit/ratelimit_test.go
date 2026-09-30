package ratelimit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestLimiter(clock *fakeClock) *Limiter {
	return New(Config{
		PerClient:  Rate{Burst: 3, Every: 10 * time.Second},
		Global:     Rate{Burst: 5, Every: time.Second},
		MaxClients: 4,
		Now:        clock.Now,
	})
}

func mustAllow(t *testing.T, l *Limiter, key string) {
	t.Helper()
	if ok, wait := l.Allow(key); !ok {
		t.Fatalf("Allow(%q) = false (retry after %v), want true", key, wait)
	}
}

func TestPerClientLimit(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := newTestLimiter(clock)

	for range 3 {
		mustAllow(t, l, "192.0.2.1")
	}
	ok, wait := l.Allow("192.0.2.1")
	if ok || wait != 10*time.Second {
		t.Fatalf("4th Allow = %v, retry after %v; want false, 10s", ok, wait)
	}
	// Another client is not affected.
	mustAllow(t, l, "192.0.2.2")

	// One token comes back every 10 seconds.
	clock.Advance(4 * time.Second)
	if ok, wait := l.Allow("192.0.2.1"); ok || wait != 6*time.Second {
		t.Errorf("Allow after 4s = %v, retry after %v; want false, 6s", ok, wait)
	}
	clock.Advance(6 * time.Second)
	mustAllow(t, l, "192.0.2.1")
}

func TestGlobalLimit(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := New(Config{
		PerClient:  Rate{Burst: 3, Every: 10 * time.Second},
		Global:     Rate{Burst: 5, Every: time.Second},
		MaxClients: 100,
		Now:        clock.Now,
	})

	for i := range 5 {
		mustAllow(t, l, fmt.Sprintf("192.0.2.%d", i))
	}
	ok, wait := l.Allow("192.0.2.99")
	if ok || wait != time.Second {
		t.Fatalf("6th client Allow = %v, retry after %v; want false, 1s", ok, wait)
	}
	// The global refusal did not cost the client its own budget: after one
	// second it still has all 3 tokens, and uses the one global token.
	clock.Advance(time.Second)
	mustAllow(t, l, "192.0.2.99")

	// A client over its own limit never spends global tokens.
	clock.Advance(5 * time.Second) // the global bucket is full again
	for range 2 {
		mustAllow(t, l, "192.0.2.99")
	}
	for range 10 {
		if ok, _ := l.Allow("192.0.2.99"); ok {
			t.Fatal("client over its limit was allowed")
		}
	}
	// 5 global tokens - 2 used by that client = 3 left for everybody else.
	for i := range 3 {
		mustAllow(t, l, fmt.Sprintf("198.51.100.%d", i))
	}
}

func TestClientTableIsBounded(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := newTestLimiter(clock) // MaxClients: 4

	for i := range 4 {
		mustAllow(t, l, fmt.Sprintf("192.0.2.%d", i))
	}
	if ok, wait := l.Allow("192.0.2.200"); ok || wait != 30*time.Second {
		t.Errorf("5th client with a full table: Allow = %v, retry after %v; want false, 30s", ok, wait)
	}

	// Once the known clients are idle long enough to be full again, they
	// are forgotten and new clients fit.
	clock.Advance(30 * time.Second)
	mustAllow(t, l, "192.0.2.200")
	l.mu.Lock()
	n := len(l.clients)
	l.mu.Unlock()
	if n != 1 {
		t.Errorf("clients tracked after the sweep = %d, want 1", n)
	}
}

func TestClientKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		cloudRun   bool
		want       string
	}{
		{
			name:       "locally, the connection's address",
			remoteAddr: "192.0.2.1:54321",
			want:       "192.0.2.1",
		},
		{
			name:       "locally, X-Forwarded-For is ignored: any client can send one",
			remoteAddr: "192.0.2.1:54321",
			xff:        []string{"203.0.113.9"},
			want:       "192.0.2.1",
		},
		{
			name:       "on Cloud Run, the entry Google's front end appended",
			remoteAddr: "169.254.1.1:1234",
			xff:        []string{"203.0.113.9"},
			cloudRun:   true,
			want:       "203.0.113.9",
		},
		{
			name:       "on Cloud Run, entries the client sent itself are ignored",
			remoteAddr: "169.254.1.1:1234",
			xff:        []string{"1.1.1.1, 2.2.2.2,203.0.113.9"},
			cloudRun:   true,
			want:       "203.0.113.9",
		},
		{
			name:       "on Cloud Run, several header lines are one list",
			remoteAddr: "169.254.1.1:1234",
			xff:        []string{"1.1.1.1", "203.0.113.9"},
			cloudRun:   true,
			want:       "203.0.113.9",
		},
		{
			name:       "on Cloud Run, no header falls back to the connection",
			remoteAddr: "169.254.1.1:1234",
			cloudRun:   true,
			want:       "169.254.1.1",
		},
		{
			name:       "on Cloud Run, garbage falls back to the connection",
			remoteAddr: "169.254.1.1:1234",
			xff:        []string{"1.1.1.1, not-an-ip"},
			cloudRun:   true,
			want:       "169.254.1.1",
		},
		{
			name:       "IPv6 clients are grouped by /64",
			remoteAddr: "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:443",
			want:       "2001:db8:1:2::/64",
		},
		{
			name:       "IPv6 on Cloud Run too",
			remoteAddr: "169.254.1.1:1234",
			xff:        []string{"2001:db8:1:2::99"},
			cloudRun:   true,
			want:       "2001:db8:1:2::/64",
		},
		{
			name:       "IPv4-mapped IPv6 is IPv4",
			remoteAddr: "[::ffff:192.0.2.1]:80",
			want:       "192.0.2.1",
		},
		{
			name:       "not an IP at all",
			remoteAddr: "@",
			want:       "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
			r.RemoteAddr = tt.remoteAddr
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := ClientKey(r, tt.cloudRun); got != tt.want {
				t.Errorf("ClientKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIdleClientsAreForgottenWithoutNewRequests(t *testing.T) {
	t.Parallel()
	// Real time here: the sweep runs on a timer.
	l := New(Config{
		PerClient:  Rate{Burst: 1, Every: 20 * time.Millisecond},
		Global:     Rate{Burst: 10, Every: time.Millisecond},
		MaxClients: 10,
	})
	mustAllow(t, l, "192.0.2.1")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		n, scheduled := len(l.clients), l.sweepScheduled
		l.mu.Unlock()
		if n == 0 && !scheduled {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the idle client was never forgotten, or the sweep timer kept running")
}

func TestNewRejectsAnEmptyRate(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("New() with an empty Rate did not panic")
		}
	}()
	New(Config{PerClient: Rate{Burst: 1, Every: time.Second}})
}
