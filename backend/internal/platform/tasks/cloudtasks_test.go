package tasks

import (
	"context"
	"encoding/base64"
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
)

const (
	testQueue    = "projects/demo-project/locations/southamerica-east1/queues/demo-tasks"
	testTarget   = "https://demo-api-abc123-rj.a.run.app"
	testAccount  = "demo-api@demo-project.iam.gserviceaccount.com"
	secretCanary = "TOKENCANARY"
)

// fakeAPI stands in for the Cloud Tasks REST API and the metadata server.
type fakeAPI struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	tokens   int
	creates  []received
	statuses []int           // answers for the next creates, then 200
	names    map[string]bool // task names seen
}

type received struct {
	path, auth string
	task       map[string]any
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, names: map[string]bool{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == cloudTasksTokenPath {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "flavor", http.StatusForbidden)
			return
		}
		f.mu.Lock()
		f.tokens++
		n := f.tokens
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": secretCanary + "-" + string(rune('0'+n)), "expires_in": 3600})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Task map[string]any `json:"task"`
	}
	_ = json.Unmarshal(body, &req)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, received{path: r.URL.Path, auth: r.Header.Get("Authorization"), task: req.Task})
	if len(f.statuses) > 0 {
		st := f.statuses[0]
		f.statuses = f.statuses[1:]
		w.WriteHeader(st)
		_, _ = w.Write([]byte(`{"error":{"message":"` + secretCanary + ` in a body"}}`))
		return
	}
	if name, _ := req.Task["name"].(string); name != "" {
		if f.names[name] {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"status":"ALREADY_EXISTS"}}`))
			return
		}
		f.names[name] = true
	}
	_, _ = w.Write([]byte(`{}`))
}

func (f *fakeAPI) created() []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]received(nil), f.creates...)
}

func newTestRunner(t *testing.T) (*CloudTasks, *fakeAPI) {
	t.Helper()
	f := newFakeAPI(t)
	c, err := NewCloudTasks(Config{Queue: testQueue, TargetURL: testTarget, ServiceAccount: testAccount}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c.apiBase, c.metadataBase = f.server.URL, f.server.URL
	c.pause = time.Millisecond
	t.Cleanup(c.Cancel)
	return c, f
}

func noop(context.Context, json.RawMessage) error { return nil }

func TestEnqueueSendsTheTask(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("make-image", noop, Options{Deadline: 7 * time.Minute})

	at := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: map[string]string{"id": "abc"}, Name: "image-abc_1", At: at})
	if err != nil {
		t.Fatal(err)
	}
	got := f.created()
	if len(got) != 1 {
		t.Fatalf("creates = %d, want 1", len(got))
	}
	if want := "/v2/" + testQueue + "/tasks"; got[0].path != want {
		t.Errorf("path = %q, want %q", got[0].path, want)
	}
	if got[0].auth != "Bearer "+secretCanary+"-1" {
		t.Errorf("authorization = %q", got[0].auth)
	}
	task := got[0].task
	if task["name"] != testQueue+"/tasks/image-abc_1" {
		t.Errorf("name = %v", task["name"])
	}
	if task["dispatchDeadline"] != "420s" {
		t.Errorf("dispatchDeadline = %v", task["dispatchDeadline"])
	}
	if task["scheduleTime"] != at.Format(time.RFC3339) {
		t.Errorf("scheduleTime = %v", task["scheduleTime"])
	}
	req, _ := task["httpRequest"].(map[string]any)
	if req["url"] != testTarget+"/internal/tasks/make-image" || req["httpMethod"] != "POST" {
		t.Errorf("httpRequest = %v", req)
	}
	if h, _ := req["headers"].(map[string]any); h["Content-Type"] != "application/json" {
		t.Errorf("headers = %v", req["headers"])
	}
	raw, err := base64.StdEncoding.DecodeString(req["body"].(string))
	if err != nil || string(raw) != `{"id":"abc"}` {
		t.Errorf("body = %q, %v", raw, err)
	}
	oidc, _ := req["oidcToken"].(map[string]any)
	if oidc["serviceAccountEmail"] != testAccount || oidc["audience"] != testTarget {
		t.Errorf("oidcToken = %v", oidc)
	}
}

func TestEnqueueWithoutNameOrTimeLeavesThemOut(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}, At: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	task := f.created()[0].task
	if _, ok := task["name"]; ok {
		t.Error("an unnamed task has a name")
	}
	if _, ok := task["scheduleTime"]; ok {
		t.Error("a task due already has a scheduleTime")
	}
	if task["dispatchDeadline"] != "600s" {
		t.Errorf("default dispatchDeadline = %v", task["dispatchDeadline"])
	}
}

func TestDeadlineIsCappedAtWhatCloudTasksAllows(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("long", noop, Options{Deadline: 35 * time.Minute})
	c.Register("short", noop, Options{Deadline: 3 * time.Second})
	for _, k := range []Kind{"long", "short"} {
		if err := c.Enqueue(t.Context(), Job{Kind: k, Payload: struct{}{}}); err != nil {
			t.Fatal(err)
		}
	}
	got := f.created()
	if got[0].task["dispatchDeadline"] != "1800s" || got[1].task["dispatchDeadline"] != "15s" {
		t.Errorf("deadlines = %v, %v; want 1800s and 15s", got[0].task["dispatchDeadline"], got[1].task["dispatchDeadline"])
	}
	if e, _ := c.lookup("long"); e.opts.Deadline != MaxDeadline {
		t.Errorf("the handler's deadline = %v, want %v", e.opts.Deadline, MaxDeadline)
	}
}

func TestEnqueueRejectsBadNamesAndUnknownKinds(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	for _, name := range []string{"has space", "slash/inside", strings.Repeat("a", 501)} {
		if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}, Name: name}); err == nil {
			t.Errorf("name %.20q accepted", name)
		}
	}
	if err := c.Enqueue(t.Context(), Job{Kind: "nope", Payload: struct{}{}}); err == nil {
		t.Error("an unknown kind was accepted")
	}
	if n := len(f.created()); n != 0 {
		t.Errorf("%d calls for refused jobs", n)
	}
}

func TestEnqueueExistingNamedTaskIsErrExists(t *testing.T) {
	t.Parallel()
	c, _ := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	job := Job{Kind: "make-image", Payload: struct{}{}, Name: "same"}
	if err := c.Enqueue(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if err := c.Enqueue(t.Context(), job); !errors.Is(err, ErrExists) {
		t.Errorf("second enqueue = %v, want ErrExists", err)
	}
}

func TestEnqueueRetriesTransientErrorsWithoutLeakingBodies(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	f.statuses = []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}
	if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}}); err != nil {
		t.Fatalf("after two transient errors: %v", err)
	}
	if n := len(f.created()); n != 3 {
		t.Errorf("attempts = %d, want 3", n)
	}

	f.mu.Lock()
	f.creates = nil
	f.statuses = []int{503, 503, 503}
	f.mu.Unlock()
	err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}})
	if err == nil || strings.Contains(err.Error(), secretCanary) || !strings.Contains(err.Error(), "503") {
		t.Errorf("error after three 503 = %v", err)
	}

	f.mu.Lock()
	f.creates = nil
	f.statuses = []int{http.StatusForbidden}
	f.mu.Unlock()
	if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}}); err == nil || len(f.created()) != 1 {
		t.Errorf("a 403 must fail at once: err = %v, attempts = %d", err, len(f.created()))
	}
}

func TestAccessTokenIsCachedAndForgottenOn401(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	for range 3 {
		if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}}); err != nil {
			t.Fatal(err)
		}
	}
	if f.tokens != 1 {
		t.Errorf("tokens fetched = %d, want 1", f.tokens)
	}
	f.mu.Lock()
	f.statuses = []int{http.StatusUnauthorized}
	f.mu.Unlock()
	if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}}); err == nil {
		t.Error("a 401 was accepted")
	}
	if err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}}); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 2 {
		t.Errorf("tokens after a 401 = %d, want 2", f.tokens)
	}
}

func TestTokenFailureNeverLeaksTheTokenOrBody(t *testing.T) {
	t.Parallel()
	c, _ := newTestRunner(t)
	c.Register("make-image", noop, Options{})
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(secretCanary))
	}))
	defer meta.Close()
	c.metadataBase = meta.URL
	err := c.Enqueue(t.Context(), Job{Kind: "make-image", Payload: struct{}{}})
	if err == nil || strings.Contains(err.Error(), secretCanary) {
		t.Errorf("error = %v", err)
	}
}

func TestSlotNames(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000*600+123, 0)
	name, at := slot("sweep-things", 10*time.Minute, now)
	if name != "sweep-sweep-things-1001" || !at.Equal(time.Unix(1001*600, 0)) {
		t.Errorf("slot = %q at %v", name, at)
	}
	// Exactly on a boundary the next slot is the following one.
	name, _ = slot("x", 10*time.Minute, time.Unix(1000*600, 0))
	if name != "sweep-x-1001" {
		t.Errorf("on the boundary: %q", name)
	}
}

func TestStartEnqueuesTheNextSlotAndExistingIsFine(t *testing.T) {
	t.Parallel()
	c, f := newTestRunner(t)
	fixed := time.Unix(5000*600+30, 0)
	c.now = func() time.Time { return fixed }
	c.Every("expired", 10*time.Minute, noop, Options{})
	c.Start(t.Context())
	c.Start(t.Context()) // twice is once
	c.Wait(t.Context())
	got := f.created()
	if len(got) != 1 {
		t.Fatalf("creates = %d, want 1", len(got))
	}
	if got[0].task["name"] != testQueue+"/tasks/sweep-expired-5001" {
		t.Errorf("name = %v", got[0].task["name"])
	}
	if got[0].task["scheduleTime"] != time.Unix(5001*600, 0).UTC().Format(time.RFC3339) {
		t.Errorf("scheduleTime = %v", got[0].task["scheduleTime"])
	}

	// A second instance starting in the same slot gets ALREADY_EXISTS and carries on.
	if err := c.enqueueNext(t.Context(), "expired"); err != nil {
		t.Errorf("an existing slot is not an error: %v", err)
	}
}

func TestNewCloudTasksValidates(t *testing.T) {
	t.Parallel()
	good := Config{Queue: testQueue, TargetURL: testTarget, ServiceAccount: testAccount}
	if _, err := NewCloudTasks(good, nil); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Config){
		"no queue":        func(c *Config) { c.Queue = "" },
		"short queue":     func(c *Config) { c.Queue = "demo-tasks" },
		"http target":     func(c *Config) { c.TargetURL = "http://demo.example.com" },
		"target path":     func(c *Config) { c.TargetURL = testTarget + "/api" },
		"target query":    func(c *Config) { c.TargetURL = testTarget + "?x=1" },
		"not an e-mail":   func(c *Config) { c.ServiceAccount = "demo-api" },
		"no account":      func(c *Config) { c.ServiceAccount = "" },
		"target no host":  func(c *Config) { c.TargetURL = "https://" },
		"queue with tail": func(c *Config) { c.Queue = testQueue + "/tasks/x" },
	}
	for name, mutate := range cases {
		cfg := good
		mutate(&cfg)
		if _, err := NewCloudTasks(cfg, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	local := good
	local.TargetURL = "http://localhost:8080"
	if _, err := NewCloudTasks(local, nil); err != nil {
		t.Errorf("localhost over http: %v", err)
	}
}

func TestRegisterPanicsOnBadKindsAndDuplicates(t *testing.T) {
	t.Parallel()
	c, _ := newTestRunner(t)
	c.Register("ok-kind", noop, Options{})
	for name, f := range map[string]func(){
		"twice":     func() { c.Register("ok-kind", noop, Options{}) },
		"uppercase": func() { c.Register("Bad", noop, Options{}) },
		"slash":     func() { c.Register("a/b", noop, Options{}) },
		"interval":  func() { c.Every("fast", time.Millisecond, noop, Options{}) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			f()
		}()
	}
}
