package reqlimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	systemv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1"
)

// The server cannot attach a typed detail to the read error of Connect, so the
// web (isSizeRefused in web/src/app/core/connect/connect-errors.ts) tells a
// body over its size limit from a handler's own quota refusal, which is also
// `resource_exhausted`, by two wordings of the message:
//
//   - "http.MaxBytesReader limit": the body is over its read limit (Middleware);
//   - "larger than configured max": the message is over the Backstop (Connect).
//
// This test pins both against a real server. If a Connect upgrade changes a
// wording, change isSizeRefused and its spec with it.
func TestSizeRefusalWordingTheWebReads(t *testing.T) {
	const procedure = "/meurpg.test.v1.Echo/Say"
	// No procedure is read above the Backstop in production; this one is, so that the
	// body reaches Connect's own check (at the Backstop the reader refuses first).
	withRaised(t, "/meurpg.test.v1.Echo/Big", 2*Backstop)
	echo := func(_ context.Context, req *connect.Request[systemv1.GetServerInfoResponse]) (*connect.Response[systemv1.GetServerInfoResponse], error) {
		return connect.NewResponse(req.Msg), nil
	}
	mux := http.NewServeMux()
	mux.Handle(procedure, connect.NewUnaryHandler(procedure, echo, connect.WithReadMaxBytes(Backstop)))
	mux.Handle("/meurpg.test.v1.Echo/Big", connect.NewUnaryHandler("/meurpg.test.v1.Echo/Big", echo, connect.WithReadMaxBytes(Backstop)))
	srv := httptest.NewServer(Middleware(mux))
	t.Cleanup(srv.Close)
	call := func(path string, n int) *connect.Error {
		client := connect.NewClient[systemv1.GetServerInfoResponse, systemv1.GetServerInfoResponse](http.DefaultClient, srv.URL+path)
		_, err := client.CallUnary(t.Context(), connect.NewRequest(&systemv1.GetServerInfoResponse{Version: text(n)}))
		ce, ok := errors.AsType[*connect.Error](err)
		if !ok {
			t.Fatalf("%s with %d bytes: %v, want a *connect.Error", path, n, err)
		}
		return ce
	}
	check := func(name string, ce *connect.Error, wording string) {
		t.Helper()
		if ce.Code() != connect.CodeResourceExhausted {
			t.Errorf("%s: code = %v, want resource_exhausted", name, ce.Code())
		}
		if got := ce.Meta().Get("Retry-After"); got != "" {
			t.Errorf("%s: Retry-After = %q, want none (it would read as a rate limit)", name, got)
		}
		if len(ce.Details()) != 0 {
			t.Errorf("%s: has %d typed details, want none", name, len(ce.Details()))
		}
		if !strings.Contains(ce.Message(), wording) {
			t.Errorf("%s: message = %q, want it to contain %q", name, ce.Message(), wording)
		}
	}

	check("over the read limit", call(procedure, DefaultReadLimit+1), "http.MaxBytesReader limit")
	check("over the backstop", call("/meurpg.test.v1.Echo/Big", Backstop+1), "larger than configured max")
}
