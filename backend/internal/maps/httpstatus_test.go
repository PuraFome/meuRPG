package maps

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

// The Connect protocol's table of HTTP statuses
// (https://connectrpc.com/docs/protocol#error-codes): the plain HTTP routes
// answer with the same ones.
func TestHTTPStatusFollowsTheConnectProtocol(t *testing.T) {
	t.Parallel()
	for code, want := range map[connect.Code]int{
		connect.CodeCanceled:           499,
		connect.CodeUnknown:            http.StatusInternalServerError,
		connect.CodeInvalidArgument:    http.StatusBadRequest,
		connect.CodeDeadlineExceeded:   http.StatusGatewayTimeout,
		connect.CodeNotFound:           http.StatusNotFound,
		connect.CodeAlreadyExists:      http.StatusConflict,
		connect.CodePermissionDenied:   http.StatusForbidden,
		connect.CodeResourceExhausted:  http.StatusTooManyRequests,
		connect.CodeFailedPrecondition: http.StatusBadRequest,
		connect.CodeAborted:            http.StatusConflict,
		connect.CodeOutOfRange:         http.StatusBadRequest,
		connect.CodeUnimplemented:      http.StatusNotImplemented,
		connect.CodeInternal:           http.StatusInternalServerError,
		connect.CodeUnavailable:        http.StatusServiceUnavailable,
		connect.CodeDataLoss:           http.StatusInternalServerError,
		connect.CodeUnauthenticated:    http.StatusUnauthorized,
	} {
		if got := httpStatus(code); got != want {
			t.Errorf("httpStatus(%s) = %d, want %d", code, got, want)
		}
	}
}

// A context error and a Connect error come out with their own codes.
func TestWriteErrorStatuses(t *testing.T) {
	t.Parallel()
	s := &Service{logger: slog.New(slog.DiscardHandler)}
	for name, tc := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"deadline":         {fmt.Errorf("render: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "deadline_exceeded"},
		"canceled":         {context.Canceled, 499, "canceled"},
		"connect internal": {connect.NewError(connect.CodeInternal, errors.New("boom")), http.StatusInternalServerError, "internal"},
		"connect timeout":  {connect.NewError(connect.CodeDeadlineExceeded, errors.New("slow")), http.StatusGatewayTimeout, "deadline_exceeded"},
		"anything else":    {errors.New("surprise"), http.StatusServiceUnavailable, "unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			s.writeError(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), tc.err)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"`+tc.code+`"`) {
				t.Errorf("status %d body %s, want %d %s", rec.Code, rec.Body.String(), tc.status, tc.code)
			}
		})
	}
}

// If-None-Match is a list of entity tags (RFC 9110, 13.1.2).
func TestETagMatches(t *testing.T) {
	t.Parallel()
	const etag = `"t7"`
	for name, tc := range map[string]struct {
		values []string
		want   bool
	}{
		"exact":              {[]string{`"t7"`}, true},
		"weak validator":     {[]string{`W/"t7"`}, true},
		"in a list":          {[]string{`"t1", "t7", "t9"`}, true},
		"weak in a list":     {[]string{`"t1",W/"t7"`}, true},
		"on a second line":   {[]string{`"t1"`, `"t7"`}, true},
		"star":               {[]string{`*`}, true},
		"another tag":        {[]string{`"t8"`}, false},
		"a prefix is not it": {[]string{`"t70"`}, false},
		"unquoted":           {[]string{`t7`}, false},
		"empty":              {[]string{``}, false},
		"no header":          {nil, false},
	} {
		if got := etagMatches(tc.values, etag); got != tc.want {
			t.Errorf("%s: etagMatches(%q) = %v, want %v", name, tc.values, got, tc.want)
		}
	}
}
