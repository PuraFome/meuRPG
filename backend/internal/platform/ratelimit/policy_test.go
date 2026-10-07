package ratelimit

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestScaledMakesEveryRateMoreGenerous(t *testing.T) {
	t.Parallel()
	got := scaled(Config{
		PerClient: Rate{Burst: 20, Every: 6 * time.Second},
		Global:    Rate{Burst: 50, Every: time.Second},
	}, 10)
	if got.PerClient.Burst != 200 || got.PerClient.Every != 600*time.Millisecond ||
		got.Global.Burst != 500 || got.Global.Every != 100*time.Millisecond {
		t.Fatalf("scaled = %+v", got)
	}
	// Zero or negative means "no change", never a panic in New.
	if got := scaled(Config{PerClient: Rate{Burst: 3, Every: time.Second}, Global: Rate{Burst: 3, Every: time.Second}}, 0); got.PerClient.Burst != 3 {
		t.Fatalf("scaled by 0 = %+v", got)
	}
	NewPolicy(0.001) // tiny multipliers still make valid limiters
}

func TestMiddlewareLimitsTheAPIRoutesByIP(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := New(Config{
		PerClient:  Rate{Burst: 2, Every: 10 * time.Second},
		Global:     Rate{Burst: 100, Every: time.Second},
		MaxClients: 10,
		Now:        clock.Now,
	})
	var logs bytes.Buffer
	n := NewNotifier(slog.New(slog.NewTextHandler(&logs, nil)), "ip")
	handler := Middleware(l, false, n)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	do := func(path, remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil)
		req.RemoteAddr = remote
		// Not trusted off Cloud Run: it must not move the client to another bucket.
		req.Header.Set("X-Forwarded-For", "198.51.100.9")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for range 2 {
		if rec := do("/meurpg.campaigns.v1.CampaignService/ListMyCampaigns", "192.0.2.1:1000"); rec.Code != http.StatusNoContent {
			t.Fatalf("within the limit: status %d", rec.Code)
		}
	}
	rec := do("/uploads/images", "192.0.2.1:2000")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "10" {
		t.Fatalf("over the limit: status %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), `"reason":"RATE_LIMITED"`) || !strings.Contains(rec.Body.String(), `"code":"resource_exhausted"`) {
		t.Fatalf("body = %s", rec.Body)
	}
	if strings.Contains(logs.String(), "192.0.2.1") {
		t.Fatalf("the log line carries the client's IP: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "rate limit hit") {
		t.Fatalf("no WARN line: %q", logs.String())
	}
	// Another IP, the static app and the probes are not touched.
	if rec := do("/images/abc", "192.0.2.2:1"); rec.Code != http.StatusNoContent {
		t.Fatalf("another IP: status %d", rec.Code)
	}
	for _, path := range []string{"/", "/assets/app.js", "/healthz", "/auth/login"} {
		if rec := do(path, "192.0.2.1:3000"); rec.Code != http.StatusNoContent {
			t.Fatalf("%s: status %d, want it unlimited here", path, rec.Code)
		}
	}
	// The bucket refills.
	clock.Advance(10 * time.Second)
	if rec := do("/images/abc", "192.0.2.1:4000"); rec.Code != http.StatusNoContent {
		t.Fatalf("after the refill: status %d", rec.Code)
	}
}

func TestMiddlewareReadsTheForwardedIPOnCloudRun(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := New(Config{PerClient: Rate{Burst: 1, Every: time.Minute}, Global: Rate{Burst: 100, Every: time.Second}, MaxClients: 10, Now: clock.Now})
	handler := Middleware(l, true, NewNotifier(nil, "ip"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	do := func(xff string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/images/x", nil)
		req.RemoteAddr = "169.254.8.129:80" // Google's front end: the same for everybody
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if do("203.0.113.5") != http.StatusOK || do("203.0.113.6") != http.StatusOK {
		t.Fatal("two clients behind the front end share one bucket")
	}
	// A client that makes up the first entries stays in its own bucket (the last one is Google's).
	if do("1.1.1.1, 203.0.113.5") != http.StatusTooManyRequests {
		t.Fatal("a forged leading entry moved the client to a fresh bucket")
	}
}

func TestNotifierLogsOncePerInterval(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	n := NewNotifier(slog.New(slog.NewTextHandler(&logs, nil)), "rpc")
	n.now = clock.Now
	for range 5 {
		n.Hit(t.Context())
	}
	if got := strings.Count(logs.String(), "rate limit hit"); got != 1 {
		t.Fatalf("%d lines for 5 hits in a row, want 1", got)
	}
	clock.Advance(11 * time.Second)
	n.Hit(t.Context())
	if got := strings.Count(logs.String(), "rate limit hit"); got != 2 || !strings.Contains(logs.String(), "suppressed_since_last_line=4") {
		t.Fatalf("after the interval: %s", logs.String())
	}
}

func TestInterceptorLimitsSignedInUsersOnly(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	l := New(Config{PerClient: Rate{Burst: 2, Every: 3 * time.Second}, Global: Rate{Burst: 100, Every: time.Second}, MaxClients: 10, Now: clock.Now})
	type ctxKey struct{}
	who := func(ctx context.Context) (string, bool) {
		id, ok := ctx.Value(ctxKey{}).(string)
		return id, ok
	}
	next := Interceptor(l, who, NewNotifier(nil, "rpc")).WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})
	call := func(user string) error {
		ctx := t.Context()
		if user != "" {
			ctx = context.WithValue(ctx, ctxKey{}, user)
		}
		_, err := next(ctx, connect.NewRequest(&struct{}{}))
		return err
	}

	for range 10 {
		if err := call(""); err != nil {
			t.Fatalf("a call with no session is not this limiter's: %v", err)
		}
	}
	for range 2 {
		if err := call("alice"); err != nil {
			t.Fatalf("within the burst: %v", err)
		}
	}
	err := call("alice")
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok || connectErr.Code() != connect.CodeResourceExhausted || connectErr.Meta().Get("Retry-After") != "3" || !IsRateLimited(err) {
		t.Fatalf("over the limit: %v", err)
	}
	if err := call("bob"); err != nil {
		t.Fatalf("another user is not affected: %v", err)
	}
	clock.Advance(3 * time.Second)
	if err := call("alice"); err != nil {
		t.Fatalf("after the refill: %v", err)
	}
	if IsRateLimited(connect.NewError(connect.CodeResourceExhausted, errors.New("gallery full"))) {
		t.Fatal("a plain resource_exhausted is not a rate limit")
	}
}

func TestChainRunsInOrder(t *testing.T) {
	t.Parallel()
	var order []string
	mark := func(name string) connect.Interceptor {
		return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				order = append(order, name)
				return next(ctx, req)
			}
		})
	}
	fn := Chain(mark("a"), mark("b")).WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		order = append(order, "handler")
		return nil, nil
	})
	if _, err := fn(t.Context(), connect.NewRequest(&struct{}{})); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "a,b,handler" {
		t.Fatalf("order = %v", order)
	}
}
