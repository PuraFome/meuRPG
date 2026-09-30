package nostore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	systemv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/system"
)

// TestInterceptor serves a real Connect service behind the interceptor and
// checks both a success and an error from a later interceptor.
func TestInterceptor(t *testing.T) {
	t.Parallel()
	// failIfAsked fails the request when it carries the Fail header, the way
	// an authorization interceptor would.
	failIfAsked := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Header().Get("Fail") != "" {
				err := connect.NewError(connect.CodeUnauthenticated, errors.New("sign in"))
				err.Meta().Set("Cache-Control", "no-store") // already set: must not repeat
				return nil, err
			}
			return next(ctx, req)
		}
	})
	mux := http.NewServeMux()
	mux.Handle(systemv1connect.NewSystemServiceHandler(system.NewService("v", "c"),
		connect.WithInterceptors(Interceptor(), failIfAsked)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := systemv1connect.NewSystemServiceClient(server.Client(), server.URL)

	res, err := client.GetServerInfo(t.Context(), connect.NewRequest(&systemv1.GetServerInfoRequest{}))
	if err != nil {
		t.Fatalf("GetServerInfo() error = %v", err)
	}
	if got := res.Header().Values("Cache-Control"); !slices.Equal(got, []string{"no-store"}) {
		t.Errorf("success Cache-Control = %q, want no-store once", got)
	}

	req := connect.NewRequest(&systemv1.GetServerInfoRequest{})
	req.Header().Set("Fail", "yes")
	_, err = client.GetServerInfo(t.Context(), req)
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		t.Fatalf("GetServerInfo() error = %v, want a Connect error", err)
	}
	if got := ce.Meta().Values("Cache-Control"); !slices.Equal(got, []string{"no-store"}) {
		t.Errorf("error Cache-Control = %q, want no-store once", got)
	}
}
