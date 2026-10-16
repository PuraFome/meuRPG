package blob

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// gcsDoOutcome is what one call of do gives: the status of the response it
// hands back, or its exact error, and the API requests it sent.
type gcsDoOutcome struct {
	status   int
	err      string
	apiCalls int
}

// gcsDoCall calls do on a list of objects (a read the fake answers 200) and
// records the outcome.
func gcsDoCall(ctx context.Context, f *fakeCloud, g *GCS, method string, retry bool) gcsDoOutcome {
	before := f.apiCalls
	resp, err := g.do(ctx, "check", method, g.bucketURL()+"/o", "", "", nil, retry) //nolint:bodyclose // closed below by drainAndClose
	out := gcsDoOutcome{}
	if err != nil {
		out.err = err.Error()
	} else {
		out.status = resp.StatusCode
		drainAndClose(resp.Body)
	}
	f.mu.Lock()
	out.apiCalls = f.apiCalls - before
	f.mu.Unlock()
	return out
}

// TestGCSDoCharacterization freezes every way out of do: the answer handed
// back as it is (a 5xx without retry, the second 5xx with retry, a 401 that
// drops the token), the retry after one 5xx, a context that ends during the
// pause, and a request that cannot be built.
func TestGCSDoCharacterization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		failNext int
		reject   int32
		method   string
		retry    bool
		pause    time.Duration
		delay    time.Duration
		timeout  time.Duration
		want     gcsDoOutcome
		tokens   int // tokens fetched by the end, including the one the next call takes
	}{
		{name: "ok", method: http.MethodGet, retry: true, want: gcsDoOutcome{status: 200, apiCalls: 1}, tokens: 1},
		{name: "5xx without retry is handed back", failNext: 1, method: http.MethodGet, want: gcsDoOutcome{status: 503, apiCalls: 1}, tokens: 1},
		{name: "5xx then ok with retry", failNext: 1, method: http.MethodGet, retry: true, want: gcsDoOutcome{status: 200, apiCalls: 2}, tokens: 1},
		{name: "second 5xx is handed back", failNext: 2, method: http.MethodGet, retry: true, want: gcsDoOutcome{status: 503, apiCalls: 2}, tokens: 1},
		{name: "401 drops the token", reject: 1, method: http.MethodGet, retry: true, want: gcsDoOutcome{status: 401, apiCalls: 1}, tokens: 2},
		{
			name: "context ends during the pause", failNext: 1, method: http.MethodGet, retry: true,
			pause: time.Hour, timeout: 200 * time.Millisecond,
			want:   gcsDoOutcome{err: "blob: cloud storage check: context deadline exceeded", apiCalls: 1},
			tokens: 1,
		},
		{
			name: "context ends during the request is not retried", method: http.MethodGet, retry: true,
			delay: 5 * time.Second, timeout: 200 * time.Millisecond,
			want:   gcsDoOutcome{err: "blob: cloud storage check: context deadline exceeded", apiCalls: 1},
			tokens: 1,
		},
		{
			name: "a request that cannot be built", method: "BAD METHOD", retry: true,
			want:   gcsDoOutcome{err: "blob: cloud storage check: cannot build the request"},
			tokens: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, g := newFakeCloud(t)
			f.failNext = tt.failNext
			f.reject.Store(tt.reject)
			f.delay = tt.delay
			if tt.pause > 0 {
				g.pause = tt.pause
			}
			ctx := t.Context()
			if tt.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}
			got := gcsDoCall(ctx, f, g, tt.method, tt.retry)
			if got != tt.want {
				t.Errorf("do() = %+v, want %+v", got, tt.want)
			}
			f.mu.Lock()
			f.delay = 0
			f.mu.Unlock()
			// A second call shows whether the token was kept or dropped.
			_ = gcsDoCall(t.Context(), f, g, http.MethodGet, false)
			f.mu.Lock()
			tokens := f.tokens
			f.mu.Unlock()
			if tokens != tt.tokens {
				t.Errorf("tokens fetched = %d, want %d", tokens, tt.tokens)
			}
		})
	}
}

// TestGCSDoNetworkErrors freezes do on a connection that dies: the error of
// the last attempt, after one retry for a read, and none for a write.
func TestGCSDoNetworkErrors(t *testing.T) {
	t.Parallel()
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprint("retry ", retry), func(t *testing.T) {
			t.Parallel()
			f, g := newFakeCloud(t)
			var calls int
			inner := f.server.Config.Handler
			f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == gcsTokenPath {
					inner.ServeHTTP(w, r)
					return
				}
				f.mu.Lock()
				calls++
				f.mu.Unlock()
				hj, _ := w.(http.Hijacker)
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
			})
			got := gcsDoCall(t.Context(), f, g, http.MethodGet, retry)
			want := "blob: cloud storage check: EOF"
			if got.err != want || got.status != 0 {
				t.Errorf("do() = %+v, want the error %q", got, want)
			}
			wantCalls := 1
			if retry {
				wantCalls = 2
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if calls != wantCalls {
				t.Errorf("%d requests, want %d", calls, wantCalls)
			}
		})
	}
}
