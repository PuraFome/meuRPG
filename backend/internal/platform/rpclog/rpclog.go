// Package rpclog logs every Connect RPC and live stream: one `rpc` line per
// call, and `stream opened` / `stream closed` lines for streams. It is a
// Connect interceptor.
//
// Mount it first (cmd/api puts it in the options shared by every service),
// so it is the outermost interceptor: it sees the final error, whichever
// interceptor or handler made it, and the user and campaign that the inner
// interceptors recorded on the request's log fields (platform/logging).
//
// It logs ids and codes, never messages: no request or response body, no
// header. See docs/architecture.md#logs.
package rpclog

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// maxErrorLen caps the error text on a line. Handler messages are short
// fixed sentences; the cap is for the odd one that echoes input.
const maxErrorLen = 500

// Interceptor returns the logging interceptor. A nil logger means
// slog.Default().
func Interceptor(logger *slog.Logger) connect.Interceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &interceptor{logger: logger}
}

type interceptor struct {
	logger *slog.Logger
}

// campaignGetter is implemented by the generated request messages that name
// a campaign (most of them). Reading it here is what lets one interceptor
// stand in for a line in every handler.
type campaignGetter interface {
	GetCampaignId() string
}

// noteCampaign puts the request's campaign on the log fields. Only a valid
// UUID is recorded, in its canonical form: the id comes from the client, and
// anything else could be text written into the logs.
func noteCampaign(ctx context.Context, msg any) {
	g, ok := msg.(campaignGetter)
	if !ok {
		return
	}
	if id, err := uuid.Parse(g.GetCampaignId()); err == nil {
		logging.SetCampaignID(ctx, id.String())
	}
}

// WrapUnary implements connect.Interceptor.
func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		start := time.Now()
		noteCampaign(ctx, req.Any())
		res, err := next(ctx, req)
		err = i.finish(ctx, req.Spec().Procedure, false, start, err)
		return res, err
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		procedure := conn.Spec().Procedure
		wrapped := &streamConn{StreamingHandlerConn: conn, onFirstMessage: func(msg any) {
			noteCampaign(ctx, msg)
			i.logger.LogAttrs(ctx, slog.LevelDebug, "stream opened", slog.String("procedure", procedure))
		}}
		err := next(ctx, wrapped)
		wrapped.open() // a handler that never read a message still gets its pair of lines

		closeReason := closeReasonOf(ctx, err, wrapped.sendFailed.Load())
		i.logger.LogAttrs(ctx, slog.LevelDebug, "stream closed",
			slog.String("procedure", procedure),
			durationAttr(start),
			slog.Int64("messages", wrapped.sent.Load()),
			slog.String("close_reason", closeReason),
		)
		return i.finish(ctx, procedure, true, start, err)
	}
}

// streamConn counts what the handler sends and notices the first message
// it reads, which is when the request (and its campaign) is known.
type streamConn struct {
	connect.StreamingHandlerConn
	onFirstMessage func(msg any)
	opened         atomic.Bool
	sent           atomic.Int64
	sendFailed     atomic.Bool
}

func (c *streamConn) open() {
	if c.opened.CompareAndSwap(false, true) {
		c.onFirstMessage(nil)
	}
}

// Receive implements connect.StreamingHandlerConn.
func (c *streamConn) Receive(msg any) error {
	err := c.StreamingHandlerConn.Receive(msg)
	if err == nil && c.opened.CompareAndSwap(false, true) {
		c.onFirstMessage(msg)
	}
	return err
}

// Send implements connect.StreamingHandlerConn.
func (c *streamConn) Send(msg any) error {
	err := c.StreamingHandlerConn.Send(msg)
	if err != nil {
		c.sendFailed.Store(true)
	} else {
		c.sent.Add(1)
	}
	return err
}

// closeReasonOf says why a stream ended, from what the server can see. A
// stream the handler ended by returning nil while the client was still there
// (session over, maximum lifetime, shutdown: it can't tell which) is the
// server's doing.
func closeReasonOf(ctx context.Context, err error, sendFailed bool) string {
	switch {
	case sendFailed:
		return "client_gone"
	case err == nil && ctx.Err() != nil:
		return "client_gone"
	case err == nil:
		return "server_ended"
	}
	switch codeOf(err) {
	case connect.CodeCanceled:
		return "client_gone"
	case connect.CodeUnauthenticated, connect.CodeNotFound, connect.CodePermissionDenied:
		return "access_lost"
	}
	return "error"
}

// finish logs the `rpc` line for a finished call and returns the error to
// send to the client, which is err itself except for one case: an error that
// is not a *connect.Error. Connect would send its text to the client with the
// code `unknown`; it is the server's own text (a driver message, a file
// path), so it stays in the log and the client gets a fixed `internal`.
func (i *interceptor) finish(ctx context.Context, procedure string, stream bool, start time.Time, err error) error {
	if err == nil {
		i.logger.LogAttrs(ctx, slog.LevelDebug, "rpc",
			slog.String("procedure", procedure), slog.String("code", "ok"), durationAttr(start), slog.Bool("stream", stream))
		return nil
	}

	out := err
	connectErr, isConnect := errors.AsType[*connect.Error](err)
	code := codeOf(err)
	if !isConnect && code == connect.CodeUnknown {
		connectErr = connect.NewError(connect.CodeInternal, errors.New("internal error"))
		out, code = connectErr, connect.CodeInternal
	}
	attrs := []slog.Attr{
		slog.String("procedure", procedure),
		slog.String("code", code.String()),
		durationAttr(start),
		slog.Bool("stream", stream),
		slog.String("error", errorText(err)),
	}
	if connectErr != nil {
		if reason := reasonOf(connectErr); reason != "" {
			attrs = append(attrs, slog.String("reason", reason))
		}
	}
	i.logger.LogAttrs(ctx, levelOf(code), "rpc", attrs...)
	return out
}

// codeOf is connect.CodeOf, which also understands a bare context error
// (Connect itself sends those as canceled and deadline_exceeded).
func codeOf(err error) connect.Code {
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		return ce.Code() // the code the service chose wins over what it wraps
	}
	switch {
	case errors.Is(err, context.Canceled):
		return connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return connect.CodeDeadlineExceeded
	}
	return connect.CodeOf(err)
}

func durationAttr(start time.Time) slog.Attr {
	return slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000)
}

// levelOf is DEBUG for the outcomes the caller causes (a refused or invalid
// request is the app working as designed) and ERROR for the ones that mean
// the server failed.
func levelOf(code connect.Code) slog.Level {
	switch code {
	case connect.CodeInvalidArgument, connect.CodeNotFound, connect.CodePermissionDenied,
		connect.CodeFailedPrecondition, connect.CodeAlreadyExists, connect.CodeAborted,
		connect.CodeUnauthenticated, connect.CodeResourceExhausted, connect.CodeOutOfRange,
		connect.CodeCanceled:
		return slog.LevelDebug
	}
	return slog.LevelError
}

// errorText is the server-side text of err, capped. For a Connect error that
// is its message (the part the client also gets); for a wrapped error, the
// whole chain, which is where the real cause is.
func errorText(err error) string {
	text := err.Error()
	if ce, ok := err.(*connect.Error); ok { //nolint:errorlint // only a bare *connect.Error has the short form
		text = ce.Message()
	}
	if len(text) > maxErrorLen {
		text = strings.ToValidUTF8(text[:maxErrorLen], "") + "..."
	}
	return text
}

// reasonOf finds the typed reason the error carries: services attach a
// detail message (XPBlocked, CharacterBlocked...) with an enum field whose
// name has "reason" in it. It returns that enum's name, or "". Reading the
// field by reflection means a new detail type is logged with no change here.
func reasonOf(err *connect.Error) string {
	for _, detail := range err.Details() {
		msg, vErr := detail.Value()
		if vErr != nil {
			continue
		}
		if reason := reasonField(msg); reason != "" {
			return reason
		}
	}
	return ""
}

func reasonField(msg proto.Message) string {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for idx := range fields.Len() {
		fd := fields.Get(idx)
		if fd.Kind() != protoreflect.EnumKind || fd.IsList() || fd.IsMap() || !strings.Contains(string(fd.Name()), "reason") {
			continue
		}
		if !m.Has(fd) {
			continue // the zero value is "unspecified"
		}
		if v := fd.Enum().Values().ByNumber(m.Get(fd).Enum()); v != nil {
			return string(v.Name())
		}
	}
	return ""
}
