package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/nostore"
)

// A handler that panics must answer `internal` (not reset the stream), leave a
// stack in the log and still get its `rpc` line, and the server must go on.
func TestAPanickingHandlerAnswersInternalAndTheServerLives(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := logging.New(&logs, slog.LevelDebug)

	mux := http.NewServeMux()
	mux.Handle("/test.Svc/Boom", connect.NewUnaryHandler("/test.Svc/Boom",
		func(context.Context, *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
			panic("the secret inside the panic")
		}, connectOptions(logger)...))
	mux.Handle("/test.Svc/Fine", connect.NewUnaryHandler("/test.Svc/Fine",
		func(context.Context, *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
			return connect.NewResponse(&campaignsv1.GetCampaignRequest{}), nil
		}, connectOptions(logger)...))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(logging.WithRequest(r.Context(), "0123456789abcdef0123456789abcdef", "")))
	}))
	t.Cleanup(srv.Close)

	call := func(name string) error {
		c := connect.NewClient[campaignsv1.GetCampaignRequest, campaignsv1.GetCampaignRequest](srv.Client(), srv.URL+"/test.Svc/"+name)
		_, err := c.CallUnary(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{}))
		return err
	}

	err := call("Boom")
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("a panicking handler answered %v, want internal", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("the panic value reached the client: %v", err)
	}
	if err := call("Fine"); err != nil {
		t.Errorf("the next call failed: %v", err)
	}

	out := logs.String()
	for _, want := range []string{"rpc handler panicked", "the secret inside the panic", `"stack"`, `"message":"rpc"`, `"code":"internal"`, "0123456789abcdef0123456789abcdef"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log misses %q:\n%s", want, out)
		}
	}
}

// Every answer of a module's RPC says `Cache-Control: no-store`, the failures
// too: a handler that returns a plain error (rpclog turns it into `internal`)
// or panics (the recover turns it into `internal`) is built outside the
// nostore interceptor's reach.
func TestEveryFailureOfAnRPCIsNoStore(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)
	handle := func(mux *http.ServeMux, name string, fn func() (*connect.Response[campaignsv1.GetCampaignRequest], error)) {
		opts := append(connectOptions(logger), connect.WithInterceptors(nostore.Interceptor()))
		mux.Handle("/test.Svc/"+name, connect.NewUnaryHandler("/test.Svc/"+name,
			func(context.Context, *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
				return fn()
			}, opts...))
	}
	mux := http.NewServeMux()
	handle(mux, "Plain", func() (*connect.Response[campaignsv1.GetCampaignRequest], error) {
		return nil, errors.New("pq: connection refused")
	})
	handle(mux, "Panic", func() (*connect.Response[campaignsv1.GetCampaignRequest], error) { panic("boom") })
	handle(mux, "Coded", func() (*connect.Response[campaignsv1.GetCampaignRequest], error) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such thing"))
	})
	handle(mux, "Fine", func() (*connect.Response[campaignsv1.GetCampaignRequest], error) {
		return connect.NewResponse(&campaignsv1.GetCampaignRequest{}), nil
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, name := range []string{"Fine", "Coded", "Plain", "Panic"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := connect.NewClient[campaignsv1.GetCampaignRequest, campaignsv1.GetCampaignRequest](srv.Client(), srv.URL+"/test.Svc/"+name)
			res, err := c.CallUnary(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{}))
			var got []string
			if err != nil {
				ce, ok := errors.AsType[*connect.Error](err)
				if !ok {
					t.Fatalf("%s error = %v, want a Connect error", name, err)
				}
				got = ce.Meta().Values("Cache-Control")
			} else {
				got = res.Header().Values("Cache-Control")
			}
			if !slices.Equal(got, []string{"no-store"}) {
				t.Errorf("%s Cache-Control = %q, want no-store once", name, got)
			}
		})
	}
}
