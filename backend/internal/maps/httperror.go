package maps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
)

// Why an upload was refused: the "reason" of an error body, which the app
// turns into its own message. The GalleryService comment in
// proto/meurpg/maps/v1/gallery.proto lists them with their HTTP statuses.
const (
	ReasonUnsupportedType  = "UNSUPPORTED_TYPE"
	ReasonTooLarge         = "TOO_LARGE"
	ReasonDimensions       = "DIMENSIONS"
	ReasonCorrupt          = "CORRUPT"
	ReasonMalformedRequest = "MALFORMED_REQUEST"
	ReasonQuota            = "QUOTA"
)

// errorBody is the JSON body of every error of the HTTP routes: the Connect
// error code (such as "invalid_argument"), the reason, when there is one,
// and an English message for developers and logs. It is not Connect's own
// error format on purpose: these routes are plain HTTP.
type errorBody struct {
	Code    string `json:"code"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message"`
}

// httpError is an error of the HTTP routes that knows how to answer.
type httpError struct {
	status  int
	code    connect.Code
	reason  string
	message string
	// retryAfter, when set, is the Retry-After header in seconds (a 503 the client may retry).
	retryAfter int
}

func (e *httpError) Error() string { return e.message }

// invalid is a 400 invalid_argument with a reason.
func invalid(reason, message string) *httpError {
	return &httpError{status: http.StatusBadRequest, code: connect.CodeInvalidArgument, reason: reason, message: message}
}

func errTooLarge() *httpError {
	return &httpError{
		status: http.StatusRequestEntityTooLarge, code: connect.CodeInvalidArgument,
		reason: ReasonTooLarge, message: "the image is larger than 10 MiB",
	}
}

func errQuota() *httpError {
	return &httpError{
		status: http.StatusTooManyRequests, code: connect.CodeResourceExhausted,
		reason: ReasonQuota, message: "the campaign's gallery is full",
	}
}

// httpStatus is the HTTP status the Connect protocol gives each code
// (https://connectrpc.com/docs/protocol#error-codes), for the codes these
// routes answer with.
func httpStatus(code connect.Code) int {
	switch code {
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition:
		return http.StatusBadRequest
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeNotFound:
		return http.StatusNotFound
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case connect.CodeAborted:
		return http.StatusConflict
	default:
		return http.StatusServiceUnavailable
	}
}

// writeError answers err as an errorBody. err is an *httpError, a Connect
// error (from authz, identity or dbError, whose code is kept), or anything
// else, which is logged and answered as unavailable.
func (s *Service) writeError(w http.ResponseWriter, r *http.Request, err error) {
	he, ok := errors.AsType[*httpError](err)
	switch {
	case ok:
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		// The client went away; nobody reads this answer.
		he = &httpError{status: http.StatusServiceUnavailable, code: connect.CodeUnavailable, message: "the request was canceled"}
	default:
		if ce, isConnect := errors.AsType[*connect.Error](err); isConnect {
			he = &httpError{status: httpStatus(ce.Code()), code: ce.Code(), message: ce.Message()}
		} else {
			s.logger.ErrorContext(r.Context(), "maps: unexpected error", "error", err)
			he = &httpError{status: http.StatusServiceUnavailable, code: connect.CodeUnavailable, message: "something went wrong, please try again"}
		}
	}
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	if he.retryAfter > 0 {
		header.Set("Retry-After", strconv.Itoa(he.retryAfter))
	}
	w.WriteHeader(he.status)
	_ = json.NewEncoder(w).Encode(errorBody{Code: he.code.String(), Reason: he.reason, Message: he.message})
}
