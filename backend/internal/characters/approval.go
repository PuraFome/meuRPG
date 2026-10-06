package characters

import (
	"context"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// Invites with approval (RN-15, MR-024).
//
// A pending member's character starts 'pending'. The master settles it with
// one of the two calls below, and each changes the character and the
// player's membership in ONE transaction, so there is never a moment where
// one has changed and the other has not:
//
//	ApproveCharacter: character 'pending' -> 'active' (a draft, RN-01)
//	                  membership 'pending' -> 'active' (a player now)
//	RejectCharacter:  character deleted, with its story
//	                  pending membership deleted (a new invite is needed)
//
// campaign_members belongs to package campaigns, which does its part
// through PendingMembers inside this transaction.
//
// Both lock the character first (FOR UPDATE, in visibleForUpdate), so an
// approval and a rejection that race run one after the other: the second
// finds the character already active (reject: failed_precondition) or
// already gone (approve: not_found). TestRN15_ApproveAndRejectRace checks it.

// ApproveCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) ApproveCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.ApproveCharacterRequest],
) (*connect.Response[charactersv1.ApproveCharacterResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}

	content, err := s.contentFor(ctx, nil, m.CampaignID) // before the write: a failure after the commit would make the client retry it
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		row, err = visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		if row.Kind != kindPlayer {
			return invalidArgument(fieldErr("character_id", "is an NPC: only player characters wait for approval"))
		}
		if row.Status != statusPending {
			return nil // already part of the campaign: nothing to change
		}
		row, err = q.ApproveCharacter(ctx, charactersdb.ApproveCharacterParams{CampaignID: m.CampaignID, ID: id})
		if err != nil {
			return wrap("approve character", err)
		}
		// A player who deleted their account while waiting has no
		// membership left: the character alone stays, as in RN-16.
		if row.PlayerUserID != nil {
			if err := s.members.ActivatePendingMember(ctx, tx, m.CampaignID, *row.PlayerUserID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "approve a character", err)
	}
	c, err := s.character(ctx, content, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.ApproveCharacterResponse{Character: c}), nil
}

// RejectCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) RejectCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.RejectCharacterRequest],
) (*connect.Response[charactersv1.RejectCharacterResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		current, err := visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		if current.Kind != kindPlayer {
			return invalidArgument(fieldErr("character_id", "is an NPC: only player characters wait for approval"))
		}
		// Only a character that never became part of the campaign may be
		// deleted: an approved one changes status, never row (RN-03).
		if current.Status != statusPending {
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_PENDING, current.ID)
		}
		if _, err := q.DeletePendingCharacter(ctx, charactersdb.DeletePendingCharacterParams{CampaignID: m.CampaignID, ID: id}); err != nil {
			return wrap("delete pending character", err)
		}
		if current.PlayerUserID != nil {
			if err := s.members.DeletePendingMember(ctx, tx, m.CampaignID, *current.PlayerUserID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "reject a character", err)
	}
	return connect.NewResponse(&charactersv1.RejectCharacterResponse{}), nil
}

// ApprovePendingCharacter approves userID's character waiting for approval
// in campaignID, if there is one, inside tx. Without one, nothing changes.
//
// Package campaigns calls it when an ordinary invite promotes a pending
// member (RN-15, Q25): an invite without approval counts as the master's
// approval. It does what ApproveCharacter does to the character (the same
// UPDATE, by player instead of by ID); the membership is the caller's to
// settle, because campaign_members is its table. It takes no caller on
// purpose, like LockSheets: the check was made by whoever calls it. Nothing
// else calls it.
func (s *Service) ApprovePendingCharacter(ctx context.Context, tx pgx.Tx, campaignID, userID string) error {
	_, err := s.queries.WithTx(tx).ApprovePendingCharacterOfPlayer(ctx, charactersdb.ApprovePendingCharacterOfPlayerParams{
		CampaignID: campaignID, PlayerUserID: &userID,
	})
	if err != nil {
		return wrap("approve pending character", err)
	}
	return nil
}
