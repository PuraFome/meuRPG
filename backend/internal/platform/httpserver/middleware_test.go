package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

func TestLogRequests(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := logging.New(&logs, slog.LevelDebug)
	handler := logRequests(logger, "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/some/rpc?token=secret-query", strings.NewReader("secret-body"))
	req.Header.Set("Authorization", "Bearer secret-header")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("expected one JSON log line, got %q: %v", logs.String(), err)
	}
	want := map[string]any{
		"message": "http request",
		"method":  "POST",
		"path":    "/some/rpc",
		"status":  float64(http.StatusTeapot),
	}
	for key, value := range want {
		if entry[key] != value {
			t.Errorf("log[%q] = %v, want %v", key, entry[key], value)
		}
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Errorf("log[duration_ms] = %v, want a number", entry["duration_ms"])
	}
	if strings.Contains(logs.String(), "secret") {
		t.Errorf("log line leaks a header, query string or body: %s", logs.String())
	}
}

func TestStatusRecorderKeepsFlusher(t *testing.T) {
	t.Parallel()

	// Connect streams only if the writer it receives is an http.Flusher.
	var w http.ResponseWriter = &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("statusRecorder does not implement http.Flusher")
	}
	flusher.Flush()

	if _, err := io.WriteString(w, "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := w.(*statusRecorder).statusCode(); got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
}

func TestLogRequestsSetsRequestID(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := logging.New(&logs, slog.LevelDebug)
	var seen string
	handler := logRequests(logger, "proj", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = logging.RequestID(r.Context())
		// What the session interceptor does deeper in the request.
		logging.SetUserID(r.Context(), "u-1")
		logging.SetCampaignID(r.Context(), "c-1")
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "client-chosen-id") // must be ignored
	req.Header.Set("X-Cloud-Trace-Context", "105445aa7843bc8bf206b12000100000/1;o=1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get(RequestIDHeader)
	if len(got) != 32 || got == "client-chosen-id" {
		t.Fatalf("X-Request-Id = %q, want a generated 32-hex id", got)
	}
	if seen != got {
		t.Errorf("context id %q differs from the header %q", seen, got)
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("log line: %v", err)
	}
	want := map[string]any{
		"request_id":                   got,
		"user_id":                      "u-1",
		"campaign_id":                  "c-1",
		"logging.googleapis.com/trace": "projects/proj/traces/105445aa7843bc8bf206b12000100000",
		"severity":                     "INFO",
	}
	for k, v := range want {
		if entry[k] != v {
			t.Errorf("log[%q] = %v, want %v", k, entry[k], v)
		}
	}
}

func TestLogRequestsWarnsOnServerErrors(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	handler := logRequests(logging.New(&logs, slog.LevelInfo), "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
	if !strings.Contains(logs.String(), `"severity":"WARNING"`) {
		t.Errorf("a 5xx must log at WARNING: %s", logs.String())
	}
}
