package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
)

// TraceKey is the key Cloud Logging reads to join a log line to a request's
// trace.
const TraceKey = "logging.googleapis.com/trace"

// fields are what the logger knows about the request in flight. It is a
// pointer shared by every context derived from the request's one, and not a
// plain value in the context, because the interesting facts arrive late and
// deeper in the call: the session interceptor learns the user inside the
// request, while the interceptor that writes the `rpc` line is outside it
// and could never see a value that was added to a child context.
type fields struct {
	mu sync.Mutex
	values
}

// values is what fields guards; it is copied out under the lock.
type values struct {
	requestID  string
	trace      string
	userID     string
	campaignID string
}

type fieldsKey struct{}

func fieldsFrom(ctx context.Context) *fields {
	f, _ := ctx.Value(fieldsKey{}).(*fields)
	return f
}

func (f *fields) snapshot() values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values
}

// WithRequest returns ctx with a new, empty set of request fields. The HTTP
// middleware calls it once per request. trace is the full
// `logging.googleapis.com/trace` value, or empty.
func WithRequest(ctx context.Context, requestID, trace string) context.Context {
	return context.WithValue(ctx, fieldsKey{}, &fields{requestID: requestID, trace: trace})
}

// NewRequestID returns 32 lowercase hex characters from crypto/rand. It is
// generated on the server for every request: an id sent by a client is never
// trusted, or one caller could write any text into another's log trail.
func NewRequestID() string {
	var b [16]byte
	// crypto/rand.Read never returns an error in Go 1.24 and later.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// TraceFromHeader turns Cloud Run's X-Cloud-Trace-Context header
// ("TRACE_ID/SPAN_ID;o=1") into the value of the trace key, or "" when there
// is no project or the header is not a 32-hex trace id.
func TraceFromHeader(project, header string) string {
	if project == "" || header == "" {
		return ""
	}
	id, _, _ := strings.Cut(header, "/")
	if len(id) != 32 {
		return ""
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return ""
		}
	}
	return "projects/" + project + "/traces/" + strings.ToLower(id)
}

// SetUserID records the signed-in user on the request's log fields. It does
// nothing when ctx has no request (a test, a background job).
func SetUserID(ctx context.Context, id string) {
	if f := fieldsFrom(ctx); f != nil {
		f.mu.Lock()
		f.userID = id
		f.mu.Unlock()
	}
}

// SetCampaignID records the campaign the request is about. Callers pass only
// IDs that were already parsed as UUIDs, so a client cannot write text into
// the logs through this key.
func SetCampaignID(ctx context.Context, id string) {
	if f := fieldsFrom(ctx); f != nil {
		f.mu.Lock()
		f.campaignID = id
		f.mu.Unlock()
	}
}

// RequestID is the request's id, or "".
func RequestID(ctx context.Context) string {
	return get(ctx, func(f values) string { return f.requestID })
}

// UserID is the signed-in user recorded for the request, or "".
func UserID(ctx context.Context) string { return get(ctx, func(f values) string { return f.userID }) }

// CampaignID is the campaign recorded for the request, or "".
func CampaignID(ctx context.Context) string {
	return get(ctx, func(f values) string { return f.campaignID })
}

func get(ctx context.Context, pick func(values) string) string {
	f := fieldsFrom(ctx)
	if f == nil {
		return ""
	}
	return pick(f.snapshot())
}

// Event logs a domain event at DEBUG: message "event", the dotted event name
// (`<area>.<verb_past>`, such as `combat.started`) and ids and closed enums
// only (never a name or a text, see the contract). Call it once the change
// has committed, after db.InTx returns: the transaction may run more than
// once on a retry, and the line must not.
func Event(ctx context.Context, l *slog.Logger, name string, attrs ...slog.Attr) {
	if l == nil || !l.Enabled(ctx, slog.LevelDebug) {
		return
	}
	all := make([]slog.Attr, 0, len(attrs)+1)
	all = append(all, slog.String("event", name))
	all = append(all, attrs...)
	l.LogAttrs(ctx, slog.LevelDebug, "event", all...)
}
