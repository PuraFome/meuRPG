package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
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
