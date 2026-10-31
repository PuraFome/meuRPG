package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/traffic"
)

// The options every handler gets note each unary call, so the idle limit (instanceIdleLimit) is measured from
// the last call the instance received.
func TestEveryUnaryCallIsNotedForTheIdleLimit(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000_000, 0)
	meter := traffic.New(instanceIdleLimit, func() time.Time { return now })
	mux := http.NewServeMux()
	mux.Handle("/test.Svc/Fine", connect.NewUnaryHandler("/test.Svc/Fine",
		func(context.Context, *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
			return connect.NewResponse(&campaignsv1.GetCampaignRequest{}), nil
		}, connectOptions(slog.New(slog.DiscardHandler), meter)...))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	now = now.Add(2 * instanceIdleLimit)
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
	now = now.Add(instanceIdleLimit)
	if !meter.Idle() {
		t.Fatal("the instance is not idle once the limit passed since its last call")
	}
}
