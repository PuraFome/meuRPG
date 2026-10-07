package httpserver

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
			slog.String("path", capPath(r.URL.Path)),
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

// maxLoggedPath is the longest request path written to the log. The path is
// chosen by the client (up to the 1 MiB header limit), so without a cap one
// request could write a megabyte into the log.
const maxLoggedPath = 256

// capPath shortens a path longer than maxLoggedPath bytes and marks the cut,
// never splitting a UTF-8 character in two.
func capPath(p string) string {
	if len(p) <= maxLoggedPath {
		return p
	}
	cut := maxLoggedPath
	for cut > 0 && !utf8.RuneStart(p[cut]) {
		cut--
	}
	return p[:cut] + "...[truncated]"
}

// apiCSP is the policy of every response that is not the Angular app: the
// API's JSON, redirects, images and errors never need to load or run
// anything, so an attacker who gets one rendered as a page gets nothing.
// The static handler replaces it with the app's own policy (cspHeader).
const apiCSP = "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// permissionsPolicy denies the browser features the app never uses. The app
// uses the clipboard (writeText, which is not restricted by default for the
// page itself) and nothing else on this list.
const permissionsPolicy = "accelerometer=(), autoplay=(), bluetooth=(), camera=(), display-capture=(), " +
	"geolocation=(), gyroscope=(), hid=(), magnetometer=(), microphone=(), midi=(), payment=(), " +
	"serial=(), usb=(), xr-spatial-tracking=()"

// hstsValue is two years, with subdomains (OWASP: at least one year).
const hstsValue = "max-age=63072000; includeSubDomains"

// securityHeaders sets, on every response, the headers that do not depend on
// the route: nosniff, a no-referrer-leaks policy, a locked-down CSP, the
// Permissions-Policy, and the cross-origin isolation pair (COOP and CORP).
// A handler that needs something different (the static handler's CSP, the
// auth routes' "no-referrer") overwrites the header with Set, which works
// because this runs before the handler.
//
// HSTS goes only on requests that arrived over HTTPS. On Cloud Run, Google's
// front end terminates TLS and sets X-Forwarded-Proto: https, and the
// container cannot be reached any other way, so a client cannot forge it
// there; anywhere else (http://localhost) a forged header only earns an HSTS
// header on a plain-HTTP response, which browsers ignore.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", apiCSP)
		h.Set("Permissions-Policy", permissionsPolicy)
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", hstsValue)
		}
		next.ServeHTTP(w, r)
	})
}
