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
)

func TestLogRequests(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := logRequests(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		"msg":    "http request",
		"method": "POST",
		"path":   "/some/rpc",
		"status": float64(http.StatusTeapot),
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
