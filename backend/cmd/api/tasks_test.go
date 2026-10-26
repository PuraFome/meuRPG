package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/httpserver"
	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tasks"
)

func TestNewTaskRunnerIsInProcessWithoutTasksConfig(t *testing.T) {
	t.Parallel()
	r, endpoint, err := newTaskRunner(t.Context(), slog.New(slog.DiscardHandler), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(*tasks.InProcess); !ok || endpoint != nil {
		t.Errorf("runner %T, endpoint %v; want InProcess and no endpoint", r, endpoint)
	}
}

func TestNewTaskRunnerIsCloudTasksWhenConfigured(t *testing.T) {
	t.Parallel()
	r, endpoint, err := newTaskRunner(t.Context(), slog.New(slog.DiscardHandler), config.Config{Tasks: config.Tasks{
		Queue:          "projects/demo-project/locations/southamerica-east1/queues/demo-tasks",
		TargetURL:      "https://demo-api.run.app",
		ServiceAccount: "demo-api@demo-project.iam.gserviceaccount.com",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(*tasks.CloudTasks); !ok || endpoint == nil {
		t.Errorf("runner %T, endpoint %v; want CloudTasks and an endpoint", r, endpoint)
	}
	taskCancel(r)()
}

// The endpoint sits in the server with the per-IP limit and the web app's catch-all: the limit
// does not apply to /internal/tasks/ (Cloud Tasks calls from Google's addresses, many at a
// time), and the catch-all does not answer for it.
func TestTaskEndpointIsNotRateLimitedNorTakenByTheCatchAll(t *testing.T) {
	t.Parallel()
	limiter := ratelimit.New(ratelimit.Config{
		PerClient:  ratelimit.Rate{Burst: 1, Every: time.Hour},
		Global:     ratelimit.Rate{Burst: 1, Every: time.Hour},
		MaxClients: 10,
	})
	srv := httpserver.New(httpserver.Config{
		Logger: slog.New(slog.DiscardHandler),
		Limit:  ratelimit.Middleware(limiter, 0, ratelimit.NewNotifier(nil, "test")),
	})
	r, endpoint, err := newTaskRunner(t.Context(), slog.New(slog.DiscardHandler), config.Config{Tasks: config.Tasks{
		Queue:          "projects/demo-project/locations/southamerica-east1/queues/demo-tasks",
		TargetURL:      "https://demo-api.run.app",
		ServiceAccount: "demo-api@demo-project.iam.gserviceaccount.com",
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer taskCancel(r)()
	srv.Handle(tasks.EndpointPrefix, endpoint)
	srv.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))

	do := func(method, path string) int {
		req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader("{}"))
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	// The limit is real: the second API call is refused.
	if first, second := do(http.MethodPost, "/meurpg.x.v1.Y/Z"), do(http.MethodPost, "/meurpg.x.v1.Y/Z"); first != http.StatusTeapot || second != http.StatusTooManyRequests {
		t.Fatalf("API calls: %d then %d; the test's limit is not working", first, second)
	}
	for range 5 {
		if code := do(http.MethodPost, tasks.EndpointPrefix+"whatever"); code != http.StatusUnauthorized {
			t.Fatalf("a call to the task endpoint without a token = %d, want 401 every time", code)
		}
	}
	if code := do(http.MethodGet, tasks.EndpointPrefix+"whatever"); code != http.StatusUnauthorized {
		t.Errorf("GET = %d, want 401", code)
	}
}
