package authz

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
)

// Pending members (RN-15, MR-024).
//
// An invite can require the master's approval. Whoever accepts such an
// invite becomes a pending member of the campaign: they create their
// character right away, and become a player only when the master approves
// it (CharacterService.ApproveCharacter). If the master rejects it, the
// pending membership is deleted.
//
// A pending member is not a member. RequireCampaignMember and
// RequireCampaignRole answer them `not_found`, exactly as they answer
// someone outside the campaign, so every call that exists today, and every
// call added later, keeps them out by default.
//
// The one exception is the allowance below: working on their one character
// while they wait. It is built from two locks, and both must open:
//
//  1. The handler asks for it, by calling RequireCampaignMemberOrPending
//     instead of RequireCampaignMember.
//  2. The call is in pendingMayCall, the whole list of what a pending
//     member may call. A handler outside the list that asks anyway still
//     answers a pending member `not_found` (and the mistake is logged).
//
// Inside an allowed call, the handler narrows what the pending member
// gets: they are treated as a player (their role is RolePlayer) who sees
// only their own pending character (package characters, access.go), or only
// the campaign's name (package campaigns, GetCampaign). The authorization
// matrices of both packages have a "pending" caller that checks every method.
//
// Changing this list changes RN-15: it needs the product decision, the
// matrices' rows, and docs/arquitetura.md ("Membro pendente").

// pendingMayCall lists every RPC that lets a pending member in, and why.
// ListMyCampaigns is not here because it needs no campaign check at all:
// it lists the caller's own memberships, pending ones included.
var pendingMayCall = map[string]string{
	campaignsv1connect.CampaignServiceGetCampaignProcedure:            "see the campaign's name while they wait",
	charactersv1connect.CharacterServiceCreateCharacterProcedure:      "create their one player character",
	charactersv1connect.CharacterServiceGetCharacterProcedure:         "read their pending character",
	charactersv1connect.CharacterServiceListCharactersProcedure:       "list their pending character",
	charactersv1connect.CharacterServiceUpdateCharacterProcedure:      "edit their pending character's sheet",
	charactersv1connect.CharacterServiceUpdateCharacterStoryProcedure: "edit their pending character's story",
	rulesv1connect.ContentServiceListContentProcedure:                 "read the rules content the character editor offers",
	rulesv1connect.ContentServiceGetSpellDetailsProcedure:             "read one spell's details, as the editor's spell list shows",
}

// RequireCampaignMemberOrPending is RequireCampaignMember for the calls a
// pending member may make too (pendingMayCall): a pending caller gets
// through, with Membership.Pending set and Role RolePlayer. Everyone else
// gets exactly what RequireCampaignMember gives them.
//
// The handler must then keep a pending member to what the allowance is for
// (see this file's top comment). For a call outside pendingMayCall, a
// pending member gets `not_found`: the list, not the handler, has the last
// word.
func RequireCampaignMemberOrPending(ctx context.Context, campaignID string) (Membership, error) {
	m, memo, err := lookUp(ctx, campaignID)
	if err != nil {
		return Membership{}, err
	}
	if !m.Pending {
		return m, nil
	}
	if _, ok := pendingMayCall[memo.procedure]; !ok {
		// A handler asked to let a pending member into a call that the
		// allowance does not cover: a programming error. Fail closed.
		memo.logger.ErrorContext(ctx, "authz: a pending member reached a call outside pendingMayCall", "procedure", memo.procedure)
		return Membership{}, errNotFound()
	}
	// campaign_members_only_players_pending already keeps a master from
	// being pending; saying it again here costs nothing.
	m.Role = RolePlayer
	return m, nil
}
