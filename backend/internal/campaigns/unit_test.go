package campaigns

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/durationpb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
)

// These tests need no database, so they also run in `go test ./...`
// without MEURPG_TEST_DATABASE_URL.

func TestInviteState(t *testing.T) {
	t.Parallel()
	now := time.Now()
	revokedAt := now.Add(-time.Minute)
	tests := []struct {
		name   string
		invite campaignsdb.CampaignInvite
		want   campaignsv1.InviteState
	}{
		{"active", campaignsdb.CampaignInvite{MaxUses: 1, ExpiresAt: now.Add(time.Second)}, campaignsv1.InviteState_INVITE_STATE_ACTIVE},
		{"expired at expires_at", campaignsdb.CampaignInvite{MaxUses: 1, ExpiresAt: now}, campaignsv1.InviteState_INVITE_STATE_EXPIRED},
		{"used up", campaignsdb.CampaignInvite{MaxUses: 2, UseCount: 2, ExpiresAt: now.Add(time.Hour)}, campaignsv1.InviteState_INVITE_STATE_USED_UP},
		{"used up wins over expired", campaignsdb.CampaignInvite{MaxUses: 1, UseCount: 1, ExpiresAt: now}, campaignsv1.InviteState_INVITE_STATE_USED_UP},
		{"revoked wins over everything", campaignsdb.CampaignInvite{MaxUses: 1, UseCount: 1, ExpiresAt: now, RevokedAt: &revokedAt}, campaignsv1.InviteState_INVITE_STATE_REVOKED},
	}
	for _, tt := range tests {
		if got := inviteState(tt.invite, now); got != tt.want {
			t.Errorf("%s: inviteState() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInviteLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		maxUses      int32
		expiresIn    *durationpb.Duration
		wantUses     int32
		wantLifetime time.Duration // 0: want invalid_argument
	}{
		{"defaults: one use, 7 days", 0, nil, 1, 7 * 24 * time.Hour},
		{"most uses", MaxInviteUses, nil, MaxInviteUses, DefaultInviteLifetime},
		{"too many uses", MaxInviteUses + 1, nil, 0, 0},
		{"negative uses", -1, nil, 0, 0},
		{"shortest", 1, durationpb.New(MinInviteLifetime), 1, MinInviteLifetime},
		{"too short", 1, durationpb.New(MinInviteLifetime - time.Second), 0, 0},
		{"longest", 1, durationpb.New(MaxInviteLifetime), 1, MaxInviteLifetime},
		{"too long", 1, durationpb.New(MaxInviteLifetime + time.Second), 0, 0},
		{"zero", 1, durationpb.New(0), 0, 0},
		{"not a valid duration", 1, &durationpb.Duration{Seconds: 3600, Nanos: -1}, 0, 0},
	}
	for _, tt := range tests {
		uses, lifetime, err := inviteLimits(&campaignsv1.CreateInviteRequest{MaxUses: tt.maxUses, ExpiresIn: tt.expiresIn})
		if tt.wantLifetime == 0 {
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("%s: inviteLimits() error = %v, want invalid_argument", tt.name, err)
			}
			continue
		}
		if err != nil || uses != tt.wantUses || lifetime != tt.wantLifetime {
			t.Errorf("%s: inviteLimits() = %d, %v, %v; want %d, %v", tt.name, uses, lifetime, err, tt.wantUses, tt.wantLifetime)
		}
	}
}

// TestXpModes: every XP mode of the API maps to a value that the
// campaigns_xp_mode_valid CHECK accepts, and back.
func TestXpModes(t *testing.T) {
	t.Parallel()
	for number := range campaignsv1.XpMode_name {
		mode := campaignsv1.XpMode(number)
		if mode == campaignsv1.XpMode_XP_MODE_UNSPECIFIED {
			continue
		}
		dbValue, ok := xpModeToDB[mode]
		if !ok {
			t.Errorf("%v has no database value", mode)
		}
		if xpModeFromDB[dbValue] != mode {
			t.Errorf("%v -> %q does not map back", mode, dbValue)
		}
	}
	if len(xpModeFromDB) != 3 {
		t.Errorf("xpModeFromDB has %d values; the CHECK allows enemies, gold and milestones", len(xpModeFromDB))
	}
}

func TestNewValidatesItsConfig(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{Profiles: noProfiles{}}); err == nil {
		t.Error("New() without a Pool succeeded")
	}
	if _, err := New(Config{Pool: lazyPool(t)}); err == nil {
		t.Error("New() without Profiles succeeded")
	}
}

// TestEveryMethodNeedsASession calls every method signed out. The session
// check comes first, so no database is needed to see it refuse.
func TestEveryMethodNeedsASession(t *testing.T) {
	t.Parallel()
	svc, err := New(Config{Pool: lazyPool(t), Profiles: noProfiles{}, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	svc.Mount(mux.Handle, testSessions, connect.WithRequireConnectProtocolHeader())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := campaignsv1connect.NewCampaignServiceClient(server.Client(), server.URL)
	ctx := t.Context()
	id := "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"

	calls := map[string]error{}
	_, calls["CreateCampaign"] = c.CreateCampaign(ctx, connect.NewRequest(&campaignsv1.CreateCampaignRequest{Name: "Mirathel", XpMode: campaignsv1.XpMode_XP_MODE_GOLD}))
	_, calls["ListMyCampaigns"] = c.ListMyCampaigns(ctx, connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	_, calls["GetCampaign"] = c.GetCampaign(ctx, connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
	_, calls["ListMembers"] = c.ListMembers(ctx, connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: id}))
	_, calls["CreateInvite"] = c.CreateInvite(ctx, connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: id}))
	_, calls["ListInvites"] = c.ListInvites(ctx, connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: id}))
	_, calls["RevokeInvite"] = c.RevokeInvite(ctx, connect.NewRequest(&campaignsv1.RevokeInviteRequest{CampaignId: id, InviteId: id}))
	_, calls["AcceptInvite"] = c.AcceptInvite(ctx, connect.NewRequest(&campaignsv1.AcceptInviteRequest{Token: "x"}))

	methods := campaignsv1.File_meurpg_campaigns_v1_campaigns_proto.Services().ByName("CampaignService").Methods()
	if len(calls) != methods.Len() {
		t.Errorf("called %d methods, the service has %d", len(calls), methods.Len())
	}
	for name, err := range calls {
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("%s signed out: error = %v, want unauthenticated", name, err)
		}
		if ce, ok := errors.AsType[*connect.Error](err); !ok || !slices.Equal(ce.Meta().Values("Cache-Control"), []string{"no-store"}) {
			t.Errorf("%s: error response Cache-Control = %q, want no-store, once", name, ce.Meta().Values("Cache-Control"))
		}
	}
}

// lazyPool is a pool that never connects: pgxpool connects on first use,
// and these tests never get that far.
func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), "postgresql://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type noProfiles struct{}

func (noProfiles) DisplayNames(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("no profiles in this test")
}
