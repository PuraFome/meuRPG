package progression

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1/progressionv1connect"
)

const allowed connect.Code = 0

// TestAuthorizationMatrix calls every ProgressionService method as each kind
// of caller (ADR-0011): only the master gives, marks and undoes; any member
// reads the history and everyone's XP (question 50); a non-member and a
// pending member (RN-15) get not_found; anonymous, unauthenticated.
func TestAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, pending := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Pendente")
	// Two campaigns with the same people: one counts XP, one marks milestones.
	xpCampaign := h.newCampaign(master, "Mirathel", enemies, player)
	markCampaign := h.newCampaign(master, "Valdris", milestones, player)
	h.joinPending(master, xpCampaign, pending)
	h.joinPending(master, markCampaign, pending)
	xpPC := player.pc(t, xpCampaign, "Pensantus")
	markPC := player.pc(t, markCampaign, "Pensantus")

	type client = progressionv1connect.ProgressionServiceClient
	rows := []struct {
		name string
		call func(ctx context.Context, c client) error
		// master, player, non-member, pending member, anonymous
		want [5]connect.Code
	}{
		// The master's call is allowed once; the others never get to it.
		{"AwardXP", func(ctx context.Context, c client) error {
			_, err := c.AwardXP(ctx, connect.NewRequest(&progressionv1.AwardXPRequest{
				CampaignId: xpCampaign, Mode: progressionv1.XPAwardMode_XP_AWARD_MODE_MANUAL, Amount: 10, Reason: "Teste",
				CharacterIds: []string{xpPC.GetId()}, IdempotencyKey: newKey(),
			}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"MarkMilestone", func(ctx context.Context, c client) error {
			_, err := c.MarkMilestone(ctx, connect.NewRequest(&progressionv1.MarkMilestoneRequest{
				CampaignId: markCampaign, Reason: "Teste", CharacterIds: []string{markPC.GetId()}, IdempotencyKey: newKey(),
			}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		// The award above is the one to undo.
		{"UndoLastXPAward", func(ctx context.Context, c client) error {
			_, err := c.UndoLastXPAward(ctx, connect.NewRequest(&progressionv1.UndoLastXPAwardRequest{CampaignId: xpCampaign, IdempotencyKey: newKey()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"ListXPAwards", func(ctx context.Context, c client) error {
			_, err := c.ListXPAwards(ctx, connect.NewRequest(&progressionv1.ListXPAwardsRequest{CampaignId: xpCampaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"GetCampaignExperience", func(ctx context.Context, c client) error {
			_, err := c.GetCampaignExperience(ctx, connect.NewRequest(&progressionv1.GetCampaignExperienceRequest{CampaignId: xpCampaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
	}

	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := progressionv1.File_meurpg_progression_v1_progression_proto.Services().ByName("ProgressionService").Methods()
	for i := range methods.Len() {
		if name := string(methods.Get(i).Name()); !covered[name] {
			t.Errorf("ProgressionService.%s is missing from the authorization matrix", name)
		}
	}

	callers := []struct {
		name   string
		client client
	}{
		{"master", master.xp},
		{"player", player.xp},
		{"non-member", h.newUser("De fora").xp},
		{"pending member", pending.xp},
		{"anonymous", h.anonymous().xp},
	}
	for _, r := range rows {
		for i, caller := range callers {
			t.Run(r.name+"/"+caller.name, func(t *testing.T) {
				err := r.call(t.Context(), caller.client)
				switch want := r.want[i]; {
				case want == allowed && err != nil:
					t.Errorf("error = %v, want allowed", err)
				case want != allowed && connect.CodeOf(err) != want:
					t.Errorf("error = %v, want %v", err, want)
				}
			})
		}
	}
}
