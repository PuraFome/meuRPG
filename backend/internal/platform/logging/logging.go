// Package logging builds the structured logger used by every binary.
//
// Logs are JSON lines on stdout. Cloud Run forwards stdout to Cloud Logging,
// which parses each JSON line into a structured entry, so fields such as
// "status" or "duration" become searchable without any extra agent. The
// field contract (names, types, what is never logged) is in
// docs/arquitetura.md, "Os logs".
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// ServiceName is the value of the `service` key on every line of the API.
const ServiceName = "meurpg-api"

// Option changes how New builds the logger.
type Option func(*options)

type options struct {
	service string
	version string
}

// WithVersion sets the `version` key (the build version the server reports in
// SystemService.GetServerInfo). Without it, the key is left out.
func WithVersion(v string) Option { return func(o *options) { o.version = v } }

// WithService overrides the `service` key, for binaries that are not the API
// (such as the migration job).
func WithService(s string) Option { return func(o *options) { o.service = s } }

// New returns a JSON logger that writes to w and drops records below level.
//
// Two keys are renamed to the names Cloud Logging understands, so the log
// viewer shows the right severity and message text:
//
//	level -> severity (and WARN -> WARNING)
//	msg   -> message
//
// Every line also carries `service` and `version`, and, when the call passes
// a request context (logger.InfoContext(ctx, ...)), the request's
// `request_id`, `user_id` and `campaign_id`. Attributes whose key is in the
// deny list (see deniedKeys) are dropped at any depth.
func New(w io.Writer, level slog.Level, opts ...Option) *slog.Logger {
	o := options{service: ServiceName}
	for _, opt := range opts {
		opt(&o)
	}
	inner := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttr,
	})
	var static []slog.Attr
	static = append(static, slog.String("service", o.service))
	if o.version != "" {
		static = append(static, slog.String("version", o.version))
	}
	return slog.New(&contextHandler{Handler: inner.WithAttrs(static)})
}

// replaceAttr renames slog's built-in keys to Cloud Logging's and applies
// the deny list. slog calls it for every leaf attribute, with the names of
// the groups it sits in, so the guard works at any depth.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if isDenied(a.Key) {
		return slog.Attr{} // an empty attribute is not written
	}
	if a.Value.Kind() == slog.KindAny {
		a.Value = slog.AnyValue(scrub(a.Value.Any()))
	}

	// Only top-level attributes can be the built-in ones.
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
		if level, ok := a.Value.Any().(slog.Level); ok && level == slog.LevelWarn {
			a.Value = slog.StringValue("WARNING")
		}
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}

// deniedKeys are attribute names that may hold secrets, personal data or
// free text a person typed (privacy and RN-10). A call site that uses one by
// mistake loses the attribute instead of leaking it. Keys are compared in
// lower case.
var deniedKeys = map[string]bool{
	"email":         true,
	"token":         true,
	"password":      true,
	"secret":        true,
	"cookie":        true,
	"authorization": true,
	"prompt":        true,
	"body":          true,
	"text":          true,
	"name":          true,
	"display_name":  true,
}

func isDenied(key string) bool { return deniedKeys[strings.ToLower(key)] }

// scrub removes denied keys from a map or slice logged with slog.Any, which
// the JSON handler would otherwise marshal untouched (ReplaceAttr only sees
// slog's own groups, not the insides of a Go value). Values it does not
// understand pass through: structs are the caller's responsibility, and the
// code review rule is "ids and closed enums only".
func scrub(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if !isDenied(k) {
				out[k] = scrub(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = scrub(val)
		}
		return out
	}
	return v
}

// contextHandler adds the request's fields to every record logged with a
// context. slog's own handlers ignore the context; this is where it is read.
type contextHandler struct {
	slog.Handler
}

// Handle implements slog.Handler.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if f := fieldsFrom(ctx); f != nil {
		snap := f.snapshot()
		// A call site may pass the same key itself (an event names its
		// campaign explicitly, because it can run after the request). JSON
		// with a duplicate key is ambiguous, so the explicit one wins.
		have := map[string]bool{}
		r.Attrs(func(a slog.Attr) bool {
			have[a.Key] = true
			return true
		})
		add := func(key, value string) {
			if value != "" && !have[key] {
				r.AddAttrs(slog.String(key, value))
			}
		}
		add("request_id", snap.requestID)
		add("user_id", snap.userID)
		add("campaign_id", snap.campaignID)
		add(TraceKey, snap.trace)
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler, keeping the wrapper.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler, keeping the wrapper.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
