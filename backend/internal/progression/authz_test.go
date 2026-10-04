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
	player2 := h.newUser("Segundo")
	markCampaign := h.newCampaign(master, "Valdris", milestones, player, player2)
	h.joinPending(master, xpCampaign, pending)
	h.joinPending(master, markCampaign, pending)
	xpPC := player.pc(t, xpCampaign, "Pensantus")
	markPC := player.pc(t, markCampaign, "Pensantus")
	markPC2 := player2.pc(t, markCampaign, "Toren")
	// The master's one call to each milestone method needs a milestone to act
	// on: one to edit, one to move, one to remove, one to mark reached and
	// one that is reached already, to give to Toren.
	forUpdate, forMove, forRemove, forMark, forGive := master.plan(t, markCampaign, "A"), master.plan(t, markCampaign, "B"),
		master.plan(t, markCampaign, "C"), master.plan(t, markCampaign, "D"), master.plan(t, markCampaign, "E")
	master.reach(t, markCampaign, forGive.GetId(), markPC.GetId())

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
		{"ListMilestones", func(ctx context.Context, c client) error {
			_, err := c.ListMilestones(ctx, connect.NewRequest(&progressionv1.ListMilestonesRequest{CampaignId: markCampaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"AddMilestone", func(ctx context.Context, c client) error {
			_, err := c.AddMilestone(ctx, connect.NewRequest(&progressionv1.AddMilestoneRequest{CampaignId: markCampaign, Text: "Teste"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"UpdateMilestone", func(ctx context.Context, c client) error {
			_, err := c.UpdateMilestone(ctx, connect.NewRequest(&progressionv1.UpdateMilestoneRequest{CampaignId: markCampaign, MilestoneId: forUpdate.GetId(), Text: "Novo"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"MoveMilestone", func(ctx context.Context, c client) error {
			_, err := c.MoveMilestone(ctx, connect.NewRequest(&progressionv1.MoveMilestoneRequest{
				CampaignId: markCampaign, MilestoneId: forMove.GetId(), Direction: progressionv1.MilestoneDirection_MILESTONE_DIRECTION_UP,
			}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"RemoveMilestone", func(ctx context.Context, c client) error {
			_, err := c.RemoveMilestone(ctx, connect.NewRequest(&progressionv1.RemoveMilestoneRequest{CampaignId: markCampaign, MilestoneId: forRemove.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"MarkMilestoneReached", func(ctx context.Context, c client) error {
			_, err := c.MarkMilestoneReached(ctx, connect.NewRequest(&progressionv1.MarkMilestoneReachedRequest{
				CampaignId: markCampaign, MilestoneId: forMark.GetId(), CharacterIds: []string{markPC.GetId()}, IdempotencyKey: newKey(),
			}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"GiveMilestoneTo", func(ctx context.Context, c client) error {
			_, err := c.GiveMilestoneTo(ctx, connect.NewRequest(&progressionv1.GiveMilestoneToRequest{
				CampaignId: markCampaign, MilestoneId: forGive.GetId(), CharacterIds: []string{markPC2.GetId()}, IdempotencyKey: newKey(),
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
		{"ListTreasuresToConvert", func(ctx context.Context, c client) error {
			_, err := c.ListTreasuresToConvert(ctx, connect.NewRequest(&progressionv1.ListTreasuresToConvertRequest{CampaignId: xpCampaign}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
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
