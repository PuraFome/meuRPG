package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// RequestIDHeader carries the request id back to the caller. The web app is
// served from the same origin as the API, so its fetches can read it without
// Access-Control-Expose-Headers (the CSP's connect-src is 'self' too).
const RequestIDHeader = "X-Request-Id"

// logRequests gives every request an id and logs one line per request with
// its method, path, protocol, status and duration.
//
// The id is generated here, never read from the client (a client could
// otherwise write any text into the logs, or pose as another request). It
// goes into the context, so every line logged for the request carries it,
// and into the X-Request-Id response header, so a person who sees a bug can
// quote it. traceProject is the Google Cloud project, set only on Cloud Run:
// with it, the Cloud Trace header is turned into the trace key that Cloud
// Logging uses to group a request's lines.
//
// It never logs headers, query strings or bodies: they can carry session
// cookies, tokens and personal data. Probe requests (/healthz, /readyz) are
// logged at DEBUG, otherwise they would drown the useful lines.
func logRequests(logger *slog.Logger, traceProject string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := logging.NewRequestID()
		ctx := logging.WithRequest(r.Context(), requestID, logging.TraceFromHeader(traceProject, r.Header.Get("X-Cloud-Trace-Context")))
		r = r.WithContext(ctx)
		// Set, not Add: whatever the client sent is overwritten by ours.
		w.Header().Set(RequestIDHeader, requestID)
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		switch {
		case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
			level = slog.LevelDebug
		case rec.statusCode() >= 500:
			level = slog.LevelWarn
		}
		// Logged after the handler ran, so user_id and campaign_id, which the
		// session and authz interceptors learn inside it, are on the line.
		logger.LogAttrs(ctx, level, "http request",
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
