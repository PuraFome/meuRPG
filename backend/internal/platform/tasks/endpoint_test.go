package tasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeIssuer serves a JWKS and mints tokens, standing in for Google.
type fakeIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIssuer{key: key, signer: signer}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

type tokenOpts struct {
	issuer, audience, email string
	unverified              bool
	expires                 time.Duration
}

func (f *fakeIssuer) mint(t *testing.T, o tokenOpts) string {
	t.Helper()
	if o.issuer == "" {
		o.issuer = f.server.URL
	}
	if o.audience == "" {
		o.audience = testTarget
	}
	if o.email == "" {
		o.email = testAccount
	}
	if o.expires == 0 {
		o.expires = time.Hour
	}
	now := time.Now()
	claims := struct {
		jwt.Claims
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}{
		Claims:   jwt.Claims{Issuer: o.issuer, Subject: "1234", Audience: jwt.Audience{o.audience}, IssuedAt: jwt.NewNumericDate(now.Add(-2 * time.Hour)), Expiry: jwt.NewNumericDate(now.Add(o.expires))},
		Email:    o.email,
		Verified: !o.unverified,
	}
	raw, err := jwt.Signed(f.signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// endpointFixture is a runner whose OIDC check trusts the fake issuer.
type endpointFixture struct {
	c      *CloudTasks
	api    *fakeAPI
	issuer *fakeIssuer
	logs   *syncBuffer
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newEndpointFixture(t *testing.T) *endpointFixture {
	t.Helper()
	c, api := newTestRunner(t)
	issuer := newFakeIssuer(t)
	logs := &syncBuffer{}
	c.logger = slog.New(slog.NewTextHandler(logs, nil))
	c.check = newTokenChecker(issuer.server.URL, issuer.server.URL, testTarget, testAccount, nil, time.Now)
	return &endpointFixture{c: c, api: api, issuer: issuer, logs: logs}
}

func (e *endpointFixture) post(t *testing.T, method, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	e.c.Handler().ServeHTTP(rec, req)
	return rec
}

func TestEndpointRefusesWhatIsNotAGoodToken(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	var ran bool
	e.c.Register("work", func(context.Context, json.RawMessage) error { ran = true; return nil }, Options{})

	other := newFakeIssuer(t) // another key: a forged signature
	cases := map[string]struct{ method, auth string }{
		"no header":        {http.MethodPost, ""},
		"not a bearer":     {http.MethodPost, "Basic abc"},
		"garbage":          {http.MethodPost, "Bearer not.a.jwt"},
		"wrong audience":   {http.MethodPost, "Bearer " + e.issuer.mint(t, tokenOpts{audience: "https://other.example.com"})},
		"wrong email":      {http.MethodPost, "Bearer " + e.issuer.mint(t, tokenOpts{email: "someone@demo-project.iam.gserviceaccount.com"})},
		"unverified email": {http.MethodPost, "Bearer " + e.issuer.mint(t, tokenOpts{unverified: true})},
		"expired":          {http.MethodPost, "Bearer " + e.issuer.mint(t, tokenOpts{expires: -time.Minute})},
		"wrong issuer":     {http.MethodPost, "Bearer " + e.issuer.mint(t, tokenOpts{issuer: "https://evil.example.com"})},
		"forged signature": {http.MethodPost, "Bearer " + other.mint(t, tokenOpts{issuer: e.issuer.server.URL})},
		"wrong method":     {http.MethodGet, "Bearer " + e.issuer.mint(t, tokenOpts{})},
		"put":              {http.MethodPut, "Bearer " + e.issuer.mint(t, tokenOpts{})},
	}
	for name, tc := range cases {
		rec := e.post(t, tc.method, EndpointPrefix+"work", tc.auth, "{}")
		if rec.Code != http.StatusUnauthorized || rec.Body.Len() != 0 {
			t.Errorf("%s: status %d, body %q; want 401 and no body", name, rec.Code, rec.Body.String())
		}
	}
	if ran {
		t.Error("a handler ran for a refused call")
	}
	if strings.Contains(e.logs.String(), "Bearer") || strings.Contains(e.logs.String(), "eyJ") {
		t.Errorf("a token reached the log: %s", e.logs.String())
	}
}

func TestEndpointUnknownKindAndBadBody(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	e.c.Register("work", noop, Options{})
	auth := "Bearer " + e.issuer.mint(t, tokenOpts{})
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"nope", auth, "{}"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown kind = %d, want 404", rec.Code)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"nope", "", "{}"); rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown kind without a token = %d, want 401", rec.Code)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"work", auth, "{not json"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json = %d, want 400", rec.Code)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"work", auth, strings.Repeat("1", maxPayloadBytes+1)); rec.Code != http.StatusBadRequest {
		t.Errorf("huge body = %d, want 400", rec.Code)
	}
}

func TestEndpointMapsTheResult(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	var got json.RawMessage
	var deadline time.Time
	e.c.Register("ok", func(ctx context.Context, p json.RawMessage) error {
		got = p
		deadline, _ = ctx.Deadline()
		return nil
	}, Options{Deadline: 2 * time.Minute})
	e.c.Register("permanent", func(context.Context, json.RawMessage) error { return Permanent(errors.New("bad id")) }, Options{})
	e.c.Register("plain", func(context.Context, json.RawMessage) error { return errors.New("db is down") }, Options{})
	e.c.Register("panics", func(context.Context, json.RawMessage) error { panic("boom") }, Options{})
	auth := "Bearer " + e.issuer.mint(t, tokenOpts{})

	start := time.Now()
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"ok", auth, `{"id":"abc"}`); rec.Code != http.StatusOK {
		t.Errorf("nil = %d", rec.Code)
	}
	if string(got) != `{"id":"abc"}` {
		t.Errorf("payload = %s", got)
	}
	if d := deadline.Sub(start); d < time.Minute || d > 2*time.Minute+time.Second {
		t.Errorf("deadline of the context = %v from now, want about 2m", d)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"ok", auth, ""); rec.Code != http.StatusOK || string(got) != "{}" {
		t.Errorf("empty body: %d, payload %s", rec.Code, got)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"permanent", auth, "{}"); rec.Code != http.StatusOK {
		t.Errorf("Permanent = %d, want 200", rec.Code)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"plain", auth, "{}"); rec.Code != http.StatusInternalServerError {
		t.Errorf("error = %d, want 500", rec.Code)
	}
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"panics", auth, "{}"); rec.Code != http.StatusInternalServerError {
		t.Errorf("panic = %d, want 500", rec.Code)
	}
	if !strings.Contains(e.logs.String(), "bad id") || !strings.Contains(e.logs.String(), "db is down") {
		t.Errorf("failures are not logged: %s", e.logs.String())
	}
}

func TestEndpointRunsTheHandlerWithTheRequestContext(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	started := make(chan struct{})
	e.c.Register("slow", func(ctx context.Context, _ json.RawMessage) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, Options{Deadline: time.Minute})
	auth := "Bearer " + e.issuer.mint(t, tokenOpts{})
	srv := httptest.NewServer(e.c.Handler())
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+EndpointPrefix+"slow", strings.NewReader("{}"))
	req.Header.Set("Authorization", auth)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	// While it runs, Wait does not return; the caller going away ends it.
	waitCtx, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	begin := time.Now()
	e.c.Wait(waitCtx)
	if time.Since(begin) < 40*time.Millisecond {
		t.Error("Wait returned while a handler ran")
	}
	cancel()
	<-done
	e.c.Wait(t.Context())
}

func TestCancelEndsRunningHandlers(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	started := make(chan struct{})
	e.c.Register("slow", func(ctx context.Context, _ json.RawMessage) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, Options{})
	auth := "Bearer " + e.issuer.mint(t, tokenOpts{})
	result := make(chan int, 1)
	go func() { result <- e.post(t, http.MethodPost, EndpointPrefix+"slow", auth, "{}").Code }()
	<-started
	e.c.Cancel()
	select {
	case code := <-result:
		if code != http.StatusInternalServerError {
			t.Errorf("a cancelled handler = %d, want 500", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel did not end the handler")
	}
	e.c.Wait(t.Context())
}

func TestScheduledDeliveryEnqueuesTheNextSlotBeforeRunning(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	e.c.now = func() time.Time { return time.Unix(7000*600+5, 0) }
	var order []string
	e.c.Every("expired", 10*time.Minute, func(context.Context, json.RawMessage) error {
		order = append(order, "handler:"+string(rune('0'+len(e.api.created())))) // creates seen when it ran
		return errors.New("fails")
	}, Options{})
	auth := "Bearer " + e.issuer.mint(t, tokenOpts{})

	rec := e.post(t, http.MethodPost, EndpointPrefix+"expired", auth, "{}")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("a failing sweep = %d, want 500 (a retry)", rec.Code)
	}
	if len(order) != 1 || order[0] != "handler:1" {
		t.Errorf("the next slot must be enqueued before the handler: %v", order)
	}
	got := e.api.created()
	if len(got) != 1 || got[0].task["name"] != testQueue+"/tasks/sweep-expired-7001" {
		t.Fatalf("creates = %+v", got)
	}

	// A retry, or another instance, in the same slot: ALREADY_EXISTS is success and the handler runs.
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"expired", auth, "{}"); rec.Code != http.StatusInternalServerError || len(order) != 2 {
		t.Errorf("second delivery: %d, handler runs %d", rec.Code, len(order))
	}

	// If the next slot cannot be enqueued the delivery fails without running the handler.
	e.api.mu.Lock()
	e.api.statuses = []int{403}
	e.api.mu.Unlock()
	e.c.now = func() time.Time { return time.Unix(7001*600+5, 0) }
	if rec := e.post(t, http.MethodPost, EndpointPrefix+"expired", auth, "{}"); rec.Code != http.StatusInternalServerError || len(order) != 2 {
		t.Errorf("enqueue refused: %d, handler runs %d", rec.Code, len(order))
	}
}

func TestMountServesUnderThePrefix(t *testing.T) {
	t.Parallel()
	e := newEndpointFixture(t)
	e.c.Register("work", noop, Options{})
	mux := http.NewServeMux()
	e.c.Mount(mux.Handle)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, EndpointPrefix+"work", io.NopCloser(strings.NewReader("{}")))
	req.Header.Set("Authorization", "Bearer "+e.issuer.mint(t, tokenOpts{}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("through the mux = %d", rec.Code)
	}
	// The catch-all does not take the prefix's paths.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, EndpointPrefix+"work", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET through the mux = %d, want 401 (not the catch-all's)", rec.Code)
	}
}
