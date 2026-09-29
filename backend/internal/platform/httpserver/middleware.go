package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// logRequests logs one line per request with its method, path, protocol,
// status and duration.
//
// It never logs headers, query strings or bodies: they can carry session
// cookies, tokens and personal data. Probe requests (/healthz, /readyz) are
// logged at DEBUG, otherwise they would drown the useful lines.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			level = slog.LevelDebug
		}
		logger.LogAttrs(r.Context(), level, "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("proto", r.Proto),
			slog.Int("status", rec.statusCode()),
			// Milliseconds with microsecond precision: readable, and numeric
			// so Cloud Logging can filter on it (e.g. duration_ms > 500).
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
		)
	})
}

// statusRecorder wraps an http.ResponseWriter to remember the status code,
// which net/http does not expose once it has been written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		// Writing a body without calling WriteHeader first implies 200.
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Flush forwards to the underlying writer. Connect needs http.Flusher to
// stream responses, and it checks for it with a type assertion, so the
// wrapper must implement it explicitly.
func (r *statusRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	// Ignore the error: flushing is best effort, as with http.Flusher.
	_ = http.NewResponseController(r.ResponseWriter).Flush()
}

// Unwrap lets http.ResponseController reach the original writer, e.g. to
// set per-request deadlines or enable full-duplex HTTP/1.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) statusCode() int {
	if r.status == 0 {
		// The handler wrote nothing at all; net/http then sends 200.
		return http.StatusOK
	}
	return r.status
}
