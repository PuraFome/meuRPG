package httpserver

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// Every kind of response carries the route-independent headers: a health
// probe, an API-like route, a 404 and the static app.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	static, ok, err := NewStatic(dir)
	if err != nil || !ok {
		t.Fatalf("NewStatic: ok=%v err=%v", ok, err)
	}
	srv := New(Config{Logger: slog.New(slog.DiscardHandler)})
	srv.Handle("GET /api-like", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	srv.Handle("GET /auth/thing", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Like the identity routes: a stricter value of a default header.
		w.Header().Set("Referrer-Policy", "no-referrer")
	}))
	srv.Handle("/", static)

	for _, tc := range []struct{ name, path, csp string }{
		{"health", "/healthz", apiCSP},
		{"api", "/api-like", apiCSP},
		{"auth", "/auth/thing", apiCSP},
		{"a missing API path", "/meurpg.nothing.v1.Nothing/Call", apiCSP},
		{"the app", "/campanhas", cspHeader(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))
			want := map[string]string{
				"X-Content-Type-Options":       "nosniff",
				"Content-Security-Policy":      tc.csp,
				"Permissions-Policy":           permissionsPolicy,
				"Cross-Origin-Opener-Policy":   "same-origin",
				"Cross-Origin-Resource-Policy": "same-origin",
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			wantRef := "same-origin"
			if tc.path == "/auth/thing" {
				wantRef = "no-referrer"
			}
			if got := rec.Header().Get("Referrer-Policy"); got != wantRef {
				t.Errorf("Referrer-Policy = %q, want %q", got, wantRef)
			}
			if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
				t.Errorf("HSTS on a plain HTTP request: %q", got)
			}
		})
	}
}

func TestHSTSOnlyOverHTTPS(t *testing.T) {
	t.Parallel()
	srv := New(Config{Logger: slog.New(slog.DiscardHandler)})

	for _, tc := range []struct {
		name, proto string
		want        bool
	}{
		{"behind Cloud Run's front end", "https", true},
		{"any case", "HTTPS", true},
		{"plain http", "http", false},
		{"no header", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			got := rec.Header().Get("Strict-Transport-Security")
			if tc.want && got != hstsValue {
				t.Errorf("HSTS = %q, want %q", got, hstsValue)
			}
			if !tc.want && got != "" {
				t.Errorf("HSTS = %q, want none", got)
			}
		})
	}

	// A direct TLS connection counts too.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/healthz", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Errorf("HSTS over TLS = %q, want %q", got, hstsValue)
	}
}

func TestLogRequestsCapsTheLoggedPath(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	handler := logRequests(logging.New(&logs, slog.LevelDebug), "", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	// Ends in a multi-byte character straddling the cap.
	long := "/" + strings.Repeat("a", maxLoggedPath-2) + "é" + strings.Repeat("b", 5000)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, long, nil))

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	got, _ := entry["path"].(string)
	if len(got) > maxLoggedPath+len("...[truncated]") || !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("logged path has %d bytes, %q...; want a capped path with a marker", len(got), got[:20])
	}
	if strings.Contains(got, "�") {
		t.Error("the cap split a UTF-8 character")
	}
	if capPath("/short") != "/short" {
		t.Error("a short path must not change")
	}
}
