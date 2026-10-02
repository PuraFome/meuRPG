package campaigns

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
)

// allowed marks a call that must succeed in the authorization matrix.
const allowed connect.Code = 0

// TestAuthorizationMatrix calls every CampaignService method as each kind of
// caller and checks who gets in (ADR-0011):
//
//	anonymous   unauthenticated, always
//	non-member  not_found for anything about the campaign, so its existence
//	            does not leak
//	player      permission_denied for what only the master may do
//	pending     a pending member (RN-15, MR-024): not_found, like a
//	            non-member, except GetCampaign (only the name) and what
//	            any signed-in user may do
func TestAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	pending := h.newUser("Pendente")
	campaign := master.createCampaign(t, "Mirathel")
	id := campaign.GetId()
	invite, token := master.createInvite(t, id, MaxInviteUses, 0)
	player.join(t, token)
	_, approvalToken := master.createApprovalInvite(t, id)
	pending.join(t, approvalToken)

	type client = campaignsv1connect.CampaignServiceClient
	methods := []struct {
		name string
		call func(ctx context.Context, c client) error
		// The expected code for master, player, non-member, anonymous,
		// pending member.
		want [5]connect.Code
	}{
		{"CreateCampaign", func(ctx context.Context, c client) error {
			_, err := c.CreateCampaign(ctx, connect.NewRequest(&campaignsv1.CreateCampaignRequest{Name: "Outra", XpMode: campaignsv1.XpMode_XP_MODE_GOLD}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, connect.CodeUnauthenticated, allowed}},

		{"ListMyCampaigns", func(ctx context.Context, c client) error {
			_, err := c.ListMyCampaigns(ctx, connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, connect.CodeUnauthenticated, allowed}},

		{"GetCampaign", func(ctx context.Context, c client) error {
			_, err := c.GetCampaign(ctx, connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},

		{"ListMembers", func(ctx context.Context, c client) error {
			_, err := c.ListMembers(ctx, connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: id}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"CreateInvite", func(ctx context.Context, c client) error {
			_, err := c.CreateInvite(ctx, connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: id}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"ListInvites", func(ctx context.Context, c client) error {
			_, err := c.ListInvites(ctx, connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: id}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"RevokeInvite", func(ctx context.Context, c client) error {
			// A fresh invite each time, so the master's call really revokes.
			fresh, _ := master.createInvite(t, id, 0, 0)
			_, err := c.RevokeInvite(ctx, connect.NewRequest(&campaignsv1.RevokeInviteRequest{CampaignId: id, InviteId: fresh.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"ListPendingMembers", func(ctx context.Context, c client) error {
			_, err := c.ListPendingMembers(ctx, connect.NewRequest(&campaignsv1.ListPendingMembersRequest{CampaignId: id}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"RemovePendingMember", func(ctx context.Context, c client) error {
			// Nobody with this ID is a member, so the master's call gets
			// not_found for the user: it passed the authorization check.
			// The others are turned away before the user is looked at. The
			// removals themselves are in TestQ24_*.
			_, err := c.RemovePendingMember(ctx, connect.NewRequest(&campaignsv1.RemovePendingMemberRequest{CampaignId: id, UserId: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"}))
			return err
		}, [5]connect.Code{connect.CodeNotFound, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// Any signed-in user may accept an invite: that is how a non-member
		// becomes one. This row runs last, because it makes the non-member a
		// member.
		{"AcceptInvite", func(ctx context.Context, c client) error {
			_, err := c.AcceptInvite(ctx, connect.NewRequest(&campaignsv1.AcceptInviteRequest{Token: token}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, connect.CodeUnauthenticated, allowed}},
	}

	// Every method of the service must be in the table, so a new RPC cannot
	// ship without its authorization test.
	service := campaignsv1.File_meurpg_campaigns_v1_campaigns_proto.Services().ByName("CampaignService")
	covered := map[string]bool{}
	for _, m := range methods {
		covered[m.name] = true
	}
	for i := range service.Methods().Len() {
		if name := string(service.Methods().Get(i).Name()); !covered[name] {
			t.Errorf("CampaignService.%s is missing from the authorization matrix", name)
		}
	}

	callers := []struct {
		name   string
		client client
	}{
		{"master", master.api},
		{"player", player.api},
		{"non-member", h.newUser("Outra pessoa").api},
		{"anonymous", h.anonymous()},
		{"pending", pending.api},
	}
	// Sequential on purpose: the AcceptInvite row changes who is a member.
	for _, m := range methods {
		for i, caller := range callers {
			t.Run(m.name+"/"+caller.name, func(t *testing.T) {
				err := m.call(t.Context(), caller.client)
				want := m.want[i]
				switch {
				case want == allowed && err != nil:
					t.Errorf("error = %v, want allowed", err)
				case want != allowed && connect.CodeOf(err) != want:
					t.Errorf("error = %v, want %v", err, want)
				}
			})
		}
	}

	if n := h.useCount(invite.GetId()); n != 3 {
		t.Errorf("use_count = %d, want 3: the player's join, the non-member's, and the pending member's promotion (Q25)", n)
	}
	if got := h.memberStatus(id, pending.id); got != "active" {
		t.Errorf("pending member's status after the matrix = %q, want active: an invite without approval promoted them (Q25)", got)
	}
}

// TestNonMembersCannotTellCampaignsApart: for a non-member, a real campaign,
// a deleted one, a made-up ID and garbage all get the same answer.
func TestNonMembersCannotTellCampaignsApart(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	outsider := h.newUser("Outra pessoa")
	existing := master.createCampaign(t, "Mirathel")
	deleted := master.createCampaign(t, "Apagada")
	if _, err := h.pool.Exec(t.Context(), "DELETE FROM campaigns WHERE id = $1", deleted.GetId()); err != nil {
		t.Fatalf("delete campaign: %v", err)
	}

	var messages []string
	for _, id := range []string{existing.GetId(), deleted.GetId(), "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff", "mirathel"} {
		_, err := outsider.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
		wantCode(t, "GetCampaign("+id+")", err, connect.CodeNotFound)
		messages = append(messages, err.Error())
	}
	for _, msg := range messages[1:] {
		if msg != messages[0] {
			t.Errorf("error messages differ: %q", messages)
			break
		}
	}

	// The master sees their campaign under any spelling of its ID.
	_, err := master.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: strings.ToUpper(existing.GetId())}))
	if err != nil {
		t.Errorf("GetCampaign(upper-case ID) error = %v", err)
	}
}

// TestResponsesAreNotCached: every answer describes the caller's campaigns,
// so none may be stored by a browser or a proxy.
func TestResponsesAreNotCached(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")

	res, err := master.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil {
		t.Fatalf("ListMyCampaigns() error = %v", err)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("ListMyCampaigns Cache-Control = %q, want no-store", got)
	}

	_, err = h.anonymous().ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok || ce.Meta().Get("Cache-Control") != "no-store" {
		t.Errorf("error response %v lacks Cache-Control: no-store", err)
	}

	// ListMyCampaigns may be called with GET (its request is empty).
	get := campaignsv1connect.NewCampaignServiceClient(h.server.Client(), h.server.URL, connect.WithHTTPGet())
	if _, err := get.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("ListMyCampaigns over GET error = %v, want unauthenticated (it reached the handler)", err)
	}
}
