package campaigns

import (
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/secret"
)

func TestAcceptInviteIsIdempotentForMembers(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	campaign := master.createCampaign(t, "Mirathel")
	invite, token := master.createInvite(t, campaign.GetId(), 0, 0)
	player.join(t, token)

	// A double click, or the player opening the link again later: same
	// answer, and no second use is spent.
	res, err := player.accept(t, token)
	if err != nil || !res.GetAlreadyMember() || res.GetCampaign().GetMyRole() != campaignsv1.Role_ROLE_PLAYER {
		t.Errorf("second AcceptInvite() = %v, %v; want already_member as a player", res, err)
	}
	// The master opening their own link stays the master.
	res, err = master.accept(t, token)
	if err != nil || !res.GetAlreadyMember() || res.GetCampaign().GetMyRole() != campaignsv1.Role_ROLE_MASTER {
		t.Errorf("master's AcceptInvite() = %v, %v; want already_member as the master", res, err)
	}
	if n := h.useCount(invite.GetId()); n != 1 {
		t.Errorf("use_count = %d, want 1", n)
	}

	// Members get the campaign back even after the invite stops working:
	// they can see it anyway, so the answer reveals nothing.
	h.clock.Advance(DefaultInviteLifetime)
	if res, err := player.accept(t, token); err != nil || !res.GetAlreadyMember() {
		t.Errorf("AcceptInvite() by a member after expiry = %v, %v; want already_member", res, err)
	}
}

func TestInviteMaxUses(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := master.createCampaign(t, "Mirathel")
	invite, token := master.createInvite(t, campaign.GetId(), 3, 0)

	for range 3 {
		h.newUser("").join(t, token)
	}
	_, err := h.newUser("").accept(t, token)
	if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_USED_UP {
		t.Errorf("fourth AcceptInvite() state = %v, want USED_UP", got)
	}
	if n := len(h.memberRoles(campaign.GetId())); n != 4 {
		t.Errorf("members = %d, want 4 (the master and 3 players)", n)
	}

	list, err := master.api.ListInvites(t.Context(), connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: campaign.GetId()}))
	if err != nil {
		t.Fatalf("ListInvites() error = %v", err)
	}
	if got := list.Msg.GetInvites()[0]; got.GetId() != invite.GetId() || got.GetUseCount() != 3 ||
		got.GetState() != campaignsv1.InviteState_INVITE_STATE_USED_UP {
		t.Errorf("ListInvites() = %v, want the invite used up, 3 of 3", got)
	}
}

// TestInviteReplay: a single-use link that someone else already used tells
// the next person so ("already used"), which is how a player learns that
// their link leaked (ADR-0009).
func TestInviteReplay(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := master.createCampaign(t, "Mirathel")
	_, token := master.createInvite(t, campaign.GetId(), 0, 0)

	h.newUser("Quem pegou o link").join(t, token)
	_, err := h.newUser("Jogador de verdade").accept(t, token)
	if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_USED_UP {
		t.Errorf("AcceptInvite() with a used link: state %v, want USED_UP", got)
	}
	if msg := connectMessage(err); !strings.Contains(msg, "already used") {
		t.Errorf("message %q does not say the invite was already used", msg)
	}
}

// TestAcceptInviteRaceForTheLastUse: many people accept the same invite at
// the same moment. Exactly max_uses of them get in; the others are told it
// is used up. CockroachDB runs the transactions serializably, and the
// invite row is locked (FOR UPDATE), so two of them can never both take the
// last use.
func TestAcceptInviteRaceForTheLastUse(t *testing.T) {
	t.Parallel()
	for _, maxUses := range []int32{1, 3} {
		t.Run(strings.Repeat("I", int(maxUses)), func(t *testing.T) {
			t.Parallel()
			dbtest.PoolSize(t, 14) // the racers must overlap: one connection would run them one by one
			h := newHarness(t)
			master := h.newUser("Mestre")
			campaign := master.createCampaign(t, "Mirathel")
			invite, token := master.createInvite(t, campaign.GetId(), maxUses, 0)

			const racers = 10
			users := make([]*user, racers)
			for i := range users {
				users[i] = h.newUser("")
			}
			errs := make([]error, racers)
			start := dbtest.NewBarrier(racers)
			var wg sync.WaitGroup
			for i, u := range users {
				wg.Go(func() {
					start.Wait()
					_, errs[i] = u.accept(t, token)
				})
			}
			wg.Wait()

			joined := 0
			for i, err := range errs {
				switch {
				case err == nil:
					joined++
				case connect.CodeOf(err) == connect.CodeFailedPrecondition:
					if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_USED_UP {
						t.Errorf("racer %d: state %v, want USED_UP", i, got)
					}
				default:
					t.Errorf("racer %d: AcceptInvite() error = %v", i, err)
				}
			}
			if joined != int(maxUses) {
				t.Errorf("%d people joined, want exactly %d", joined, maxUses)
			}
			if n := len(h.memberRoles(campaign.GetId())); n != int(maxUses)+1 {
				t.Errorf("members = %d, want %d", n, maxUses+1)
			}
			if n := h.useCount(invite.GetId()); n != int(maxUses) {
				t.Errorf("use_count = %d, want %d", n, maxUses)
			}
		})
	}
}

func TestInviteExpiresAfterItsLifetime(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := master.createCampaign(t, "Mirathel")
	invite, token := master.createInvite(t, campaign.GetId(), 2, time.Hour)
	if got, want := invite.GetExpiresAt().AsTime(), h.clock.Now().Add(time.Hour); !got.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", got, want)
	}

	h.clock.Advance(time.Hour - time.Second)
	h.newUser("").join(t, token)

	h.clock.Advance(time.Second)
	_, err := h.newUser("").accept(t, token)
	if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_EXPIRED {
		t.Errorf("AcceptInvite() at expires_at: state %v, want EXPIRED", got)
	}
}

func TestAcceptInviteWithBadTokens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	player := h.newUser("Jogador")
	neverIssued, _ := secret.New()

	_, err := player.accept(t, "")
	wantCode(t, "AcceptInvite(empty)", err, connect.CodeInvalidArgument)
	for name, token := range map[string]string{
		"malformed":    "not-a-token",
		"padded":       neverIssued + "=",
		"never issued": neverIssued,
	} {
		_, err := player.accept(t, token)
		wantCode(t, "AcceptInvite("+name+")", err, connect.CodeNotFound)
	}
}

func TestRevokeInviteOnlyWithinTheCampaign(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	mirathel := master.createCampaign(t, "Mirathel")
	other := master.createCampaign(t, "Outra")
	invite, _ := master.createInvite(t, other.GetId(), 0, 0)

	// The master of both campaigns cannot revoke Outra's invite through
	// Mirathel: invite IDs only count inside their campaign.
	for _, inviteID := range []string{invite.GetId(), "not-a-uuid", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"} {
		_, err := master.api.RevokeInvite(t.Context(), connect.NewRequest(&campaignsv1.RevokeInviteRequest{
			CampaignId: mirathel.GetId(),
			InviteId:   inviteID,
		}))
		wantCode(t, "RevokeInvite("+inviteID+")", err, connect.CodeNotFound)
	}
	var revoked bool
	if err := h.pool.QueryRow(t.Context(), "SELECT revoked_at IS NOT NULL FROM campaign_invites WHERE id = $1", invite.GetId()).Scan(&revoked); err != nil || revoked {
		t.Errorf("the other campaign's invite revoked = %v, %v; want it untouched", revoked, err)
	}
}
