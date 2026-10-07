package httpserver

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// startServer runs srv on a random local port and returns its base URL, a
// function that triggers shutdown (like SIGTERM would) and a channel that
// receives Serve's return value.
func startServer(t *testing.T, srv *Server) (baseURL string, stop context.CancelFunc, done <-chan error) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ctx, ln) }()
	t.Cleanup(cancel)

	return "http://" + ln.Addr().String(), cancel, errc
}

// protoMajor makes a GET request and returns the HTTP major version the
// response came back with (1 or 2).
func protoMajor(t *testing.T, client *http.Client, url string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", url, resp.StatusCode)
	}
	return resp.ProtoMajor
}

func TestServeSpeaksHTTP1AndH2C(t *testing.T) {
	t.Parallel()

	baseURL, stop, done := startServer(t, New(Config{Logger: discardLogger()}))

	// A plain client uses HTTP/1.1 over clear text.
	if got := protoMajor(t, &http.Client{}, baseURL+"/healthz"); got != 1 {
		t.Errorf("default client got HTTP/%d, want HTTP/1", got)
	}

	// A client that only speaks unencrypted HTTP/2 ("prior knowledge"), like
	// Cloud Run's front end with end-to-end HTTP/2 enabled.
	var h2c http.Protocols
	h2c.SetUnencryptedHTTP2(true)
	h2cClient := &http.Client{Transport: &http.Transport{Protocols: &h2c}}
	if got := protoMajor(t, h2cClient, baseURL+"/healthz"); got != 2 {
		t.Errorf("h2c client got HTTP/%d, want HTTP/2", got)
	}
	h2cClient.CloseIdleConnections()

	stop()
	if err := <-done; err != nil {
		t.Errorf("Serve() = %v, want nil after a clean shutdown", err)
	}
}

func TestServeWaitsForInFlightRequests(t *testing.T) {
	t.Parallel()

	srv := New(Config{Logger: discardLogger()})
	entered := make(chan struct{})
	release := make(chan struct{})
	srv.Handle("GET /slow", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	baseURL, stop, done := startServer(t, srv)

	// Start a slow request and wait until the handler is running.
	statusc := make(chan int, 1)
	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/slow", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			statusc <- 0
			return
		}
		_ = resp.Body.Close()
		statusc <- resp.StatusCode
	}()
	<-entered

	// "SIGTERM": readiness must flip while the request is still running.
	stop()
	waitFor(t, srv.draining.Load)

	select {
	case err := <-done:
		t.Fatalf("Serve() returned %v before the in-flight request finished", err)
	default:
	}

	close(release)
	if status := <-statusc; status != http.StatusOK {
		t.Errorf("in-flight request status = %d, want 200", status)
	}
	if err := <-done; err != nil {
		t.Errorf("Serve() = %v, want nil", err)
	}
}

func TestServeGivesUpAfterShutdownTimeout(t *testing.T) {
	t.Parallel()

	srv := New(Config{Logger: discardLogger(), ShutdownTimeout: 50 * time.Millisecond})
	entered := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	srv.Handle("GET /stuck", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	}))
	baseURL, stop, done := startServer(t, srv)

	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/stuck", nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered

	stop()
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Serve() = %v, want a deadline exceeded error", err)
	}
}

// TestOnShutdownEndsLongLivedHandlers: a live stream never finishes on its
// own, so OnShutdown ends it when the shutdown starts, and the shutdown does
// not wait for its deadline (8 seconds by default).
func TestOnShutdownEndsLongLivedHandlers(t *testing.T) {
	t.Parallel()

	srv := New(Config{Logger: discardLogger(), ShutdownTimeout: 30 * time.Second})
	entered := make(chan struct{})
	shutdown := make(chan struct{})
	srv.OnShutdown(func() { close(shutdown) })
	srv.Handle("GET /stream", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = http.NewResponseController(w).Flush() // best effort, as in a stream
		close(entered)
		<-shutdown
	}))
	baseURL, stop, done := startServer(t, srv)

	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/stream", nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()
	<-entered

	start := time.Now()
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the shutdown waited for the long-lived handler")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("shutdown took %v, want it prompt", elapsed)
	}
}

// waitFor polls cond for up to a second. Used for state that changes on
// another goroutine without a channel to wait on.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 1s")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCrossOriginProtection(t *testing.T) {
	t.Parallel()

	srv := New(Config{Logger: discardLogger()})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv.Handle("POST /rpc", ok)
	srv.Handle("GET /page", ok)

	tests := []struct {
		name       string
		method     string
		path       string
		header     map[string]string
		wantStatus int
	}{
		{"same-origin POST from a browser", http.MethodPost, "/rpc", map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"POST from a non-browser client (no Sec-Fetch-Site, no Origin)", http.MethodPost, "/rpc", nil, http.StatusOK},
		{"cross-site POST from a browser", http.MethodPost, "/rpc", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"same-site but cross-origin POST", http.MethodPost, "/rpc", map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"POST whose Origin is another host (older browser)", http.MethodPost, "/rpc", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"cross-site GET passes (safe method)", http.MethodGet, "/page", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "http://meurpg.example"+tt.path, nil)
			for k, v := range tt.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// The Limit option wraps the routes inside the request log and the security
// headers, so a refused request is logged like any other and carries them.
func TestLimitRunsInsideTheLogAndHeaders(t *testing.T) {
	t.Parallel()
	var limited int
	srv := New(Config{Logger: discardLogger(), Limit: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/limited" {
				limited++
				http.Error(w, "slow down", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/limited", nil))
	if rec.Code != http.StatusTooManyRequests || limited != 1 {
		t.Fatalf("status %d, limited %d", rec.Code, limited)
	}
	if rec.Header().Get(RequestIDHeader) == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("the refusal lacks the request id or the security headers: %v", rec.Header())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status %d", rec.Code)
	}
}
