package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/traffic"
)

// movingClock is a time the test moves, read by the handler goroutines.
type movingClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *movingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *movingClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// The options every handler gets note each unary call, so the idle limit (instanceIdleLimit) is measured from
// the last call the instance received.
func TestEveryUnaryCallIsNotedForTheIdleLimit(t *testing.T) {
	t.Parallel()
	clock := &movingClock{now: time.Unix(1_000_000, 0)}
	meter := traffic.New(instanceIdleLimit, clock.Now)
	mux := http.NewServeMux()
	mux.Handle("/test.Svc/Fine", connect.NewUnaryHandler("/test.Svc/Fine",
		func(context.Context, *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
			return connect.NewResponse(&campaignsv1.GetCampaignRequest{}), nil
		}, connectOptions(slog.New(slog.DiscardHandler), meter)...))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	clock.advance(2 * instanceIdleLimit)
	if meter.Idle() {
		t.Fatal("the instance is idle before any call")
	}
	c := connect.NewClient[campaignsv1.GetCampaignRequest, campaignsv1.GetCampaignRequest](srv.Client(), srv.URL+"/test.Svc/Fine")
	if _, err := c.CallUnary(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{})); err != nil {
		t.Fatal(err)
	}
	if meter.Idle() {
		t.Fatal("the instance is idle right after a call")
	}
	clock.advance(instanceIdleLimit)
	if !meter.Idle() {
		t.Fatal("the instance is not idle once the limit passed since its last call")
	}
}

// Every unary procedure of every service, mounted the way run() does, is a call the meter notes. A service that
// forgets the shared Connect options would make an active instance look idle and cut its streams.
func TestEveryUnaryProcedureIsNotedForTheIdleLimit(t *testing.T) {
	clock := &movingClock{now: time.Unix(1_000_000, 0)}
	meter := traffic.New(instanceIdleLimit, clock.Now)
	handler := buildHandlerSetWith(t, meter)

	// The first call makes the instance one that has received calls.
	anonymousCode(t, handler, "/meurpg.system.v1.SystemService/GetServerInfo", false)
	n := 0
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(file.Package()), "meurpg.") {
			return true
		}
		for i := range file.Services().Len() {
			service := file.Services().Get(i)
			for j := range service.Methods().Len() {
				method := service.Methods().Get(j)
				if method.IsStreamingServer() {
					continue // a stream is what the idle instance keeps open
				}
				procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
				clock.advance(2 * instanceIdleLimit)
				if !meter.Idle() {
					t.Fatalf("before %s: the instance is not idle after %v without a call", procedure, 2*instanceIdleLimit)
				}
				anonymousCode(t, handler, procedure, false)
				if meter.Idle() {
					t.Errorf("a call to %s was not noted: an instance that receives it looks idle", procedure)
				}
				n++
			}
		}
		return true
	})
	if n < 100 {
		t.Errorf("only %d unary procedures found; the services are no longer all linked in", n)
	}
}
