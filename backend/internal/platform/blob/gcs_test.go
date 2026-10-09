package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const fakeBucket = "meurpg-images-test"

// fakeCloud is an in-memory stand-in for the Cloud Storage JSON API and the
// metadata server, on one httptest server.
type fakeCloud struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	objects  map[string]fakeObject
	tokens   int           // tokens handed out
	lifetime int           // expires_in of the next token
	failNext int           // how many API calls answer 503 first
	apiCalls int           // API calls received
	reject   atomic.Int32  // API calls to answer 401
	delay    time.Duration // how long the API waits before answering
	paths    []string      // escaped paths the API received
}

type fakeObject struct {
	contentType string
	data        []byte
}

func newFakeCloud(t *testing.T) (*fakeCloud, *GCS) {
	t.Helper()
	f := &fakeCloud{t: t, objects: map[string]fakeObject{}, lifetime: 3600}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	g, err := NewGCS(fakeBucket)
	if err != nil {
		t.Fatal(err)
	}
	g.apiBase, g.metadataBase = f.server.URL, f.server.URL
	g.pause = time.Millisecond
	return f, g
}

func (f *fakeCloud) token(n int) string { return fmt.Sprintf("TOKENCANARY-%d", n) }

func (f *fakeCloud) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == gcsTokenPath {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing flavor", http.StatusForbidden)
			return
		}
		f.mu.Lock()
		f.tokens++
		n, life := f.tokens, f.lifetime
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": f.token(n), "expires_in": life, "token_type": "Bearer"})
		return
	}
	f.mu.Lock()
	f.apiCalls++
	f.paths = append(f.paths, r.URL.EscapedPath())
	fail := f.failNext > 0
	if fail {
		f.failNext--
	}
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer TOKENCANARY-") {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}
	if f.reject.Load() > 0 {
		f.reject.Add(-1)
		http.Error(w, "bad token", http.StatusUnauthorized)
		return
	}
	if fail {
		// A body that echoes the credential, to prove it never reaches an error.
		http.Error(w, "backend error for "+r.Header.Get("Authorization"), http.StatusServiceUnavailable)
		return
	}
	objectPrefix := "/storage/v1/b/" + fakeBucket + "/o/"
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/upload/storage/v1/b/"+fakeBucket+"/o":
		if r.URL.Query().Get("uploadType") != "media" {
			http.Error(w, "uploadType", http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[r.URL.Query().Get("name")] = fakeObject{r.Header.Get("Content-Type"), data}
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"name": r.URL.Query().Get("name")})
	case r.Method == http.MethodGet && r.URL.Path == "/storage/v1/b/"+fakeBucket+"/o":
		_, _ = io.WriteString(w, `{"kind":"storage#objects"}`)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, objectPrefix):
		f.mu.Lock()
		obj, ok := f.objects[strings.TrimPrefix(r.URL.Path, objectPrefix)]
		f.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", obj.contentType)
		w.Header().Set("X-Goog-Generation", strconv.Itoa(len(obj.data)))
		if g := r.URL.Query().Get("generation"); g != "" && g != strconv.Itoa(len(obj.data)) {
			http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(obj.data))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, objectPrefix):
		name := strings.TrimPrefix(r.URL.Path, objectPrefix)
		f.mu.Lock()
		_, ok := f.objects[name]
		delete(f.objects, name)
		f.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusTeapot)
	}
}

func TestGCSPutOpenDeleteRoundTrip(t *testing.T) {
	f, g := newFakeCloud(t)
	ctx := t.Context()
	key := "campaigns/abc-1/images/img_2.thumb"

	if err := g.Put(ctx, key, "image/jpeg", strings.NewReader("pixels")); err != nil {
		t.Fatal(err)
	}
	if got := f.objects[key]; got.contentType != "image/jpeg" || string(got.data) != "pixels" {
		t.Fatalf("stored %+v", got)
	}

	obj, err := g.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if obj.ContentType != "image/jpeg" || obj.Size != 6 {
		t.Fatalf("object %q, %d bytes", obj.ContentType, obj.Size)
	}
	got, err := io.ReadAll(obj.Content)
	if err != nil || string(got) != "pixels" {
		t.Fatalf("read %q, %v", got, err)
	}
	if err := obj.Close(); err != nil {
		t.Fatal(err)
	}

	// The key travels as one escaped path segment.
	for _, p := range f.paths {
		if strings.Contains(p, "/o/") && !strings.Contains(p, "campaigns%2Fabc-1%2Fimages%2Fimg_2.thumb") {
			t.Errorf("object path %q does not escape the key", p)
		}
	}

	if err := g.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open after delete: %v", err)
	}
}

func TestGCSPutReplacesAndLeavesTheCallersReaderOpen(t *testing.T) {
	f, g := newFakeCloud(t)
	closed := &closeSpy{Reader: strings.NewReader("second")}
	if err := g.Put(t.Context(), "a/b", "text/plain", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	if err := g.Put(t.Context(), "a/b", "text/plain", closed); err != nil {
		t.Fatal(err)
	}
	if string(f.objects["a/b"].data) != "second" {
		t.Fatalf("stored %q", f.objects["a/b"].data)
	}
	if closed.closed {
		t.Error("Put closed the reader it was given")
	}
}

type closeSpy struct {
	io.Reader
	closed bool
}

func (c *closeSpy) Close() error { c.closed = true; return nil }

func TestGCSDeleteOfAMissingObjectIsNotAnError(t *testing.T) {
	_, g := newFakeCloud(t)
	if err := g.Delete(t.Context(), "never/was"); err != nil {
		t.Fatal(err)
	}
}

func TestGCSRefusesInvalidKeysAndContentTypes(t *testing.T) {
	f, g := newFakeCloud(t)
	ctx := t.Context()
	for _, key := range []string{"", "../x", "/abs", "A", "a//b"} {
		if err := g.Put(ctx, key, "image/png", strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("put %q: %v", key, err)
		}
		if _, err := g.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("open %q: %v", key, err)
		}
		if err := g.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("delete %q: %v", key, err)
		}
	}
	if err := g.Put(ctx, "a", "image/png\nx", strings.NewReader("x")); err == nil {
		t.Error("put accepted a content type with a line break")
	}
	if f.apiCalls != 0 {
		t.Errorf("%d API calls for invalid input", f.apiCalls)
	}
}

func TestGCSOpenSeeksAndAnswersRanges(t *testing.T) {
	_, g := newFakeCloud(t)
	ctx := t.Context()
	if err := g.Put(ctx, "k", "image/png", strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	obj, err := g.Open(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = obj.Close() }()

	// What http.ServeContent does: learn the size, go back, then jump.
	if n, err := obj.Content.Seek(0, io.SeekEnd); err != nil || n != 10 {
		t.Fatalf("seek end: %d, %v", n, err)
	}
	if _, err := obj.Content.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 3)
	if _, err := io.ReadFull(obj.Content, head); err != nil || string(head) != "012" {
		t.Fatalf("head %q, %v", head, err)
	}
	if _, err := obj.Content.Seek(6, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(obj.Content)
	if err != nil || string(rest) != "6789" {
		t.Fatalf("tail %q, %v", rest, err)
	}
	if _, err := obj.Content.Seek(-1, io.SeekStart); err == nil {
		t.Error("a negative position was accepted")
	}

	// And through the real thing: a range request and a 304.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=2-4")
	obj2, err := g.Open(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = obj2.Close() }()
	http.ServeContent(rec, req, "", time.Time{}, obj2.Content)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "234" {
		t.Fatalf("range answered %d %q", rec.Code, rec.Body.String())
	}
}

func TestGCSOpenOfAMissingObjectIsErrNotFound(t *testing.T) {
	_, g := newFakeCloud(t)
	if _, err := g.Open(t.Context(), "no/such"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGCSReusesTheTokenAndRefreshesItBeforeItExpires(t *testing.T) {
	f, g := newFakeCloud(t)
	clock := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return clock }
	ctx := t.Context()

	for range 3 {
		if err := g.Delete(ctx, "a"); err != nil {
			t.Fatal(err)
		}
	}
	if f.tokens != 1 {
		t.Fatalf("%d tokens for 3 calls within the lifetime", f.tokens)
	}

	// 61 s before the end of the 3,600 s: still cached; 59 s before: renewed.
	clock = clock.Add(3600*time.Second - 61*time.Second)
	if err := g.Delete(ctx, "a"); err != nil || f.tokens != 1 {
		t.Fatalf("early: %d tokens, %v", f.tokens, err)
	}
	clock = clock.Add(2 * time.Second)
	if err := g.Delete(ctx, "a"); err != nil || f.tokens != 2 {
		t.Fatalf("after the margin: %d tokens, %v", f.tokens, err)
	}
}

func TestGCSForgetsATokenTheAPIRefused(t *testing.T) {
	f, g := newFakeCloud(t)
	f.reject.Store(1)
	err := g.Delete(t.Context(), "a")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("got %v", err)
	}
	if err := g.Delete(t.Context(), "a"); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 2 {
		t.Fatalf("%d tokens: the refused one was kept", f.tokens)
	}
}

func TestGCSRetriesAReadOnceOnA5xxButNotAWrite(t *testing.T) {
	f, g := newFakeCloud(t)
	ctx := t.Context()
	f.objects["k"] = fakeObject{"image/png", []byte("x")}

	f.failNext = 1
	obj, err := g.Open(ctx, "k")
	if err != nil {
		t.Fatalf("open after one 503: %v", err)
	}
	_ = obj.Close()

	f.failNext = 2
	if _, err := g.Open(ctx, "k"); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("open after two 503: %v", err)
	}

	before := f.apiCalls
	f.failNext = 1
	if err := g.Put(ctx, "k", "image/png", strings.NewReader("y")); err == nil {
		t.Fatal("a put hid a 503")
	}
	if f.apiCalls != before+1 {
		t.Errorf("put sent %d requests", f.apiCalls-before)
	}
	f.failNext = 1
	if err := g.Delete(ctx, "k"); err == nil {
		t.Fatal("a delete hid a 503")
	}
}

func TestGCSRetriesAReadOnceOnANetworkError(t *testing.T) {
	var calls atomic.Int32
	f, g := newFakeCloud(t)
	f.objects["k"] = fakeObject{"image/png", []byte("x")}
	inner := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != gcsTokenPath && calls.Add(1) == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close() // the connection dies with no answer
			return
		}
		inner.ServeHTTP(w, r)
	})
	obj, err := g.Open(t.Context(), "k")
	if err != nil {
		t.Fatal(err)
	}
	_ = obj.Close()
	if calls.Load() != 2 {
		t.Fatalf("%d requests", calls.Load())
	}
}

func TestGCSRetriesTheTokenReadOnceOnA5xx(t *testing.T) {
	var calls atomic.Int32
	f, g := newFakeCloud(t)
	inner := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gcsTokenPath && calls.Add(1) == 1 {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		inner.ServeHTTP(w, r)
	})
	if err := g.Delete(t.Context(), "a"); err != nil {
		t.Fatal(err)
	}
}

func TestGCSTimesOutOnASlowServer(t *testing.T) {
	f, g := newFakeCloud(t)
	f.delay = 5 * time.Second
	g.client.Transport.(*http.Transport).ResponseHeaderTimeout = 50 * time.Millisecond

	start := time.Now()
	err := g.Delete(t.Context(), "a")
	if err == nil {
		t.Fatal("a slow server was waited for")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("waited %v", elapsed)
	}
	if _, err := g.Open(t.Context(), "a"); err == nil {
		t.Fatal("open waited for a slow server")
	}
}

func TestGCSOpenDeadlineCoversTheBodyAndCloseReleasesIt(t *testing.T) {
	f, g := newFakeCloud(t)
	f.objects["k"] = fakeObject{"image/png", bytes.Repeat([]byte("x"), 1<<20)}
	g.openLifetime = 50 * time.Millisecond
	// A body that stalls: the handler sends the headers and half the data, then waits.
	stall := make(chan struct{})
	t.Cleanup(func() { close(stall) })
	inner := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gcsTokenPath {
			inner.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		select {
		case <-stall:
		case <-r.Context().Done():
		}
	})
	obj, err := g.Open(t.Context(), "k")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = obj.Close() }()
	if _, err := io.ReadAll(obj.Content); err == nil {
		t.Fatal("a stalled body was read to the end")
	}
}

func TestGCSNeverPutsTheTokenInAnErrorOrALog(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	f, g := newFakeCloud(t)
	ctx := t.Context()
	var errs []error

	f.failNext = 2 // the answer echoes the Authorization header
	_, err := g.Open(ctx, "k")
	errs = append(errs, err)
	f.failNext = 1
	errs = append(errs, g.Put(ctx, "k", "image/png", strings.NewReader("x")), g.Check(ctx))
	f.reject.Store(1)
	errs = append(errs, g.Delete(ctx, "k"))

	for _, e := range errs {
		if e == nil {
			continue
		}
		if strings.Contains(e.Error(), "TOKENCANARY") || strings.Contains(e.Error(), "Bearer") || strings.Contains(e.Error(), "k?") {
			t.Errorf("error leaks: %q", e)
		}
	}
	if strings.Contains(logs.String(), "TOKENCANARY") {
		t.Errorf("a log line carries the token: %q", logs.String())
	}

	// Positive control: the token is on the wire, and the canary works.
	if f.tokens == 0 {
		t.Fatal("no token was ever fetched")
	}
	if !strings.HasPrefix(f.token(1), "TOKENCANARY-") {
		t.Fatal("canary changed")
	}
}

func TestGCSErrorsDoNotCarryTheObjectNameOrBucket(t *testing.T) {
	f, g := newFakeCloud(t)
	f.server.Close()
	err := g.Delete(t.Context(), "campaigns/secret-id/images/x")
	if err == nil {
		t.Fatal("a closed server answered")
	}
	for _, s := range []string{"secret-id", fakeBucket} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q carries %q", err, s)
		}
	}
}

func TestGCSMetadataFailureIsAnError(t *testing.T) {
	f, g := newFakeCloud(t)
	inner := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gcsTokenPath {
			_, _ = io.WriteString(w, `{"access_token":""}`)
			return
		}
		inner.ServeHTTP(w, r)
	})
	if err := g.Check(t.Context()); err == nil {
		t.Fatal("a token-less answer was accepted")
	}
}

func TestGCSCheck(t *testing.T) {
	_, g := newFakeCloud(t)
	if err := g.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := g.Check(ctx); err == nil {
		t.Fatal("a canceled check passed")
	}
}

func TestNewGCSValidatesTheBucketName(t *testing.T) {
	for _, name := range []string{"", "ab", "Upper", "has space", "-lead", "trail-", "a/b"} {
		if _, err := NewGCS(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"abc", "meurpg-images", "my.bucket_1"} {
		if _, err := NewGCS(name); err != nil {
			t.Errorf("refused %q: %v", name, err)
		}
	}
}

func TestGCSProductionEndpointsAreTheRealOnes(t *testing.T) {
	g, err := NewGCS(fakeBucket)
	if err != nil {
		t.Fatal(err)
	}
	if g.apiBase != "https://storage.googleapis.com" || g.metadataBase != "http://metadata.google.internal" {
		t.Fatalf("endpoints %q, %q", g.apiBase, g.metadataBase)
	}
}

func TestGCSWorksAfterAFailedCheck(t *testing.T) {
	f, g := newFakeCloud(t)
	// The metadata server is down for the check (a hiccup at boot)...
	inner := f.server.Config.Handler
	var down atomic.Bool
	down.Store(true)
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gcsTokenPath && down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		inner.ServeHTTP(w, r)
	})
	if err := g.Check(t.Context()); err == nil {
		t.Fatal("the check passed with the metadata server down")
	}
	// ...and each later call is tried on its own, with no memory of the failure.
	down.Store(false)
	if err := g.Put(t.Context(), "k", "image/png", strings.NewReader("x")); err != nil {
		t.Fatalf("put after a failed check: %v", err)
	}
	obj, err := g.Open(t.Context(), "k")
	if err != nil {
		t.Fatalf("open after a failed check: %v", err)
	}
	_ = obj.Close()
}

func TestGCSRangeReadOfAReplacedObjectFailsInsteadOfMixingVersions(t *testing.T) {
	f, g := newFakeCloud(t)
	f.objects["k"] = fakeObject{"image/png", []byte("0123456789")}
	obj, err := g.Open(t.Context(), "k")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = obj.Close() }()

	// Another version (another generation) is stored after the open.
	f.mu.Lock()
	f.objects["k"] = fakeObject{"image/png", []byte("abcdefghij-longer")}
	f.mu.Unlock()
	if _, err := obj.Content.Seek(5, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if got, err := io.ReadAll(obj.Content); err == nil {
		t.Fatalf("read %q from another version", got)
	}
}
