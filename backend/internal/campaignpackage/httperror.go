package campaignpackage

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/platform/rpcerr"
)

// httpError is the answer of a plain route: a status and the small JSON the
// image routes use, {"code", "reason", "message"}. The message is for
// developers; the app has its own text for each reason.
type httpError struct {
	status     int
	code       connect.Code
	reason     string
	message    string
	retryAfter int
}

func (e *httpError) Error() string { return e.message }

func notFound() *httpError {
	return &httpError{status: http.StatusNotFound, code: connect.CodeNotFound, message: "not found"}
}

// writeError answers a route with err: an *httpError as it is, a Connect error
// by its code, and anything else as an unavailable or internal error that
// carries no text of the cause.
func (s *Service) writeError(w http.ResponseWriter, r *http.Request, err error) {
	he, ok := errors.AsType[*httpError](err)
	if !ok {
		if ce, isConnect := errors.AsType[*connect.Error](err); isConnect {
			he = &httpError{status: connectStatus(ce.Code()), code: ce.Code(), message: ce.Message()}
		} else {
			ce := rpcerr.FromDB(r.Context(), s.logger, "campaignpackage", "serve a package route", err)
			cerr, _ := errors.AsType[*connect.Error](ce)
			he = &httpError{status: connectStatus(cerr.Code()), code: cerr.Code(), message: cerr.Message()}
		}
	}
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "no-store")
	if he.retryAfter > 0 {
		header.Set("Retry-After", strconv.Itoa(he.retryAfter))
	}
	w.WriteHeader(he.status)
	body, jerr := json.Marshal(map[string]string{"code": he.code.String(), "reason": he.reason, "message": he.message})
	if jerr != nil {
		s.logger.LogAttrs(r.Context(), slog.LevelError, "campaignpackage: cannot encode an error", slog.Any("error", jerr))
		return
	}
	_, _ = w.Write(body) //nolint:gosec // G705: JSON from json.Marshal, sent as application/json with nosniff
}

// connectStatus is the HTTP status of a Connect code, as the Connect protocol
// table has it for the codes this service answers with.
func connectStatus(c connect.Code) int {
	switch c {
	case connect.CodeInvalidArgument, connect.CodeOutOfRange:
		return http.StatusBadRequest
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeNotFound:
		return http.StatusNotFound
	case connect.CodeAlreadyExists, connect.CodeAborted:
		return http.StatusConflict
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case connect.CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	case connect.CodeCanceled:
		return 499
	case connect.CodeUnavailable:
		return http.StatusServiceUnavailable
	case connect.CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case connect.CodeUnimplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
