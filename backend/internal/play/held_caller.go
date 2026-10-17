package play

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// The paths a held roll runs again (an attack, a cast, a save, a check, a search) ask who calls as any handler does.
// When the master answers the question of a player's roll the path must run as the player who rolled, or it would
// look for the master's character, or roll for the master, and the answer would settle another roll. answerHeld
// puts the roller's membership in the context after checking that the caller may answer, and these two
// stand in for authz.RequireCampaignMember and authz.RequireCampaignRole in those paths: with no such answer in
// the context they are exactly those.

// heldMember is authz.RequireCampaignMember for a path a held roll runs again.
func heldMember(ctx context.Context, campaignID string) (authz.Membership, error) {
	if m, ok := heldRollerOf(ctx, campaignID); ok {
		return m, nil
	}
	return authz.RequireCampaignMember(ctx, campaignID) //nolint:wrapcheck // the authz refusal, as the RPC gives it
}

// heldRole is authz.RequireCampaignRole for a path a held roll runs again.
func heldRole(ctx context.Context, campaignID string, role authz.Role) (authz.Membership, error) {
	if m, ok := heldRollerOf(ctx, campaignID); ok && m.Role == role {
		return m, nil
	}
	return authz.RequireCampaignRole(ctx, campaignID, role) //nolint:wrapcheck // the authz refusal, as the RPC gives it
}

// heldRollerOf is the roller the context says the path runs as, in the campaign.
func heldRollerOf(ctx context.Context, campaignID string) (authz.Membership, bool) {
	a, ok := heldReplyOf(ctx)
	if !ok || a.as == nil || a.as.CampaignID != campaignID {
		return authz.Membership{}, false
	}
	return *a.as, true
}
