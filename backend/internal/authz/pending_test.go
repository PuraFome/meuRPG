package authz

import (
	"maps"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
)

// TestRN15_PendingMemberOnlyGetsThroughTheAllowedCalls: a pending member
// passes RequireCampaignMemberOrPending only for the calls in
// pendingMayCall, as a player; for any other call, and for every other
// check, they are a stranger to the campaign.
func TestRN15_PendingMemberOnlyGetsThroughTheAllowedCalls(t *testing.T) {
	t.Parallel()
	source := newFakeSource()

	for procedure := range pendingMayCall {
		ctx := procedureContext(t, source, pending, procedure)
		m, err := RequireCampaignMemberOrPending(ctx, campaignA)
		if err != nil {
			t.Errorf("%s: RequireCampaignMemberOrPending() error = %v, want allowed", procedure, err)
			continue
		}
		if !m.Pending || m.Role != RolePlayer || m.UserID != pending || m.CampaignID != campaignA {
			t.Errorf("%s: membership = %+v, want a pending player", procedure, m)
		}
		// The other checks never let a pending member in, whatever the call.
		checkCode(t, procedure+": RequireCampaignMember", errOf(RequireCampaignMember(ctx, campaignA)), connect.CodeNotFound)
		checkCode(t, procedure+": RequireCampaignRole", errOf(RequireCampaignRole(ctx, campaignA, RolePlayer)), connect.CodeNotFound)
	}

	// Calls outside the list: a handler that asks for the allowance anyway
	// still keeps the pending member out.
	for _, procedure := range []string{
		campaignsv1connect.CampaignServiceListMembersProcedure,
		campaignsv1connect.CampaignServiceCreateInviteProcedure,
		charactersv1connect.CharacterServiceApproveCharacterProcedure,
		charactersv1connect.CharacterServiceMarkCharacterDeadProcedure,
		charactersv1connect.CharacterServiceGetMasterNotesProcedure,
		playv1connect.PlayServiceListGameSessionsProcedure,
		"", // no procedure at all
	} {
		ctx := procedureContext(t, source, pending, procedure)
		checkCode(t, procedure+": RequireCampaignMemberOrPending", errOf(RequireCampaignMemberOrPending(ctx, campaignA)), connect.CodeNotFound)
	}
}

// TestRequireCampaignMemberOrPendingForEveryoneElse: for anyone who is not
// pending, it is exactly RequireCampaignMember.
func TestRequireCampaignMemberOrPendingForEveryoneElse(t *testing.T) {
	t.Parallel()
	procedure := charactersv1connect.CharacterServiceGetCharacterProcedure
	tests := []struct {
		name string
		user string
		want connect.Code
		role Role
	}{
		{"master", master, 0, RoleMaster},
		{"player", player, 0, RolePlayer},
		{"non-member", outsider, connect.CodeNotFound, ""},
		{"anonymous", "", connect.CodeUnauthenticated, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := RequireCampaignMemberOrPending(procedureContext(t, newFakeSource(), tt.user, procedure), campaignA)
			checkCode(t, "RequireCampaignMemberOrPending", err, tt.want)
			if err == nil && (m.Pending || m.Role != tt.role) {
				t.Errorf("membership = %+v, want an active %s", m, tt.role)
			}
		})
	}
}

// TestRN15_PendingMemberLooksLikeAStranger: outside the allowance, a
// pending member gets the very same error as someone who is not in the
// campaign, so being pending reveals nothing more.
func TestRN15_PendingMemberLooksLikeAStranger(t *testing.T) {
	t.Parallel()
	source := newFakeSource()
	_, fromPending := RequireCampaignMember(requestContext(t, source, pending), campaignA)
	_, fromStranger := RequireCampaignMember(requestContext(t, source, outsider), campaignA)
	if fromPending == nil || fromStranger == nil || fromPending.Error() != fromStranger.Error() {
		t.Errorf("errors differ: %v vs %v", fromPending, fromStranger)
	}
}

// TestPendingMayCallIsTheAgreedList pins the allowance to the product
// decision (RN-15, MR-024, docs/architecture.md): a pending member may see
// the campaign's name and work on their one character, nothing else.
// Changing the list means changing this test, the matrices and the docs.
func TestPendingMayCallIsTheAgreedList(t *testing.T) {
	t.Parallel()
	want := []string{
		campaignsv1connect.CampaignServiceGetCampaignProcedure,
		campaignsv1connect.CampaignServiceGetTableRulesProcedure,
		charactersv1connect.CharacterServiceCreateCharacterProcedure,
		charactersv1connect.CharacterServiceGetCharacterProcedure,
		charactersv1connect.CharacterServiceListCharactersProcedure,
		charactersv1connect.CharacterServicePreviewCharacterProcedure,
		charactersv1connect.CharacterServicePreviewChoicesProcedure,
		charactersv1connect.CharacterServiceUpdateCharacterProcedure,
		charactersv1connect.CharacterServiceUpdateCharacterStoryProcedure,
		charactersv1connect.CharacterServiceGetAbilityRollsProcedure,
		charactersv1connect.CharacterServiceRollAbilityScoresProcedure,
		rulesv1connect.ContentServiceListContentProcedure,
		rulesv1connect.ContentServiceGetSpellDetailsProcedure,
	}
	slices.Sort(want)
	if got := slices.Sorted(maps.Keys(pendingMayCall)); !slices.Equal(got, want) {
		t.Errorf("pendingMayCall = %v, want %v", got, want)
	}
}

// errOf keeps only the error of a Require* call.
func errOf(_ Membership, err error) error { return err }
