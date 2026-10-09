package campaigns

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
)

// The two calls below are for package characters, which hands a reserved character to
// the player who opens its claim link (MR-049). They take no caller on purpose, like
// ActivatePendingMember: the checks (the link, RN-03) are ClaimCharacter's, and nothing
// else calls them.

// CampaignForClaim returns the campaign's name and whether userID is its master, inside
// tx, for the card a claim link shows (PreviewClaim). It changes nothing. A campaign that
// does not exist is pgx.ErrNoRows.
func (s *Service) CampaignForClaim(ctx context.Context, tx pgx.Tx, campaignID, userID string) (name string, master bool, err error) {
	q := s.queriesIn(tx)
	campaign, err := q.GetCampaign(ctx, campaignID)
	if err != nil {
		return "", false, err
	}
	member, err := q.GetMembership(ctx, campaignsdb.GetMembershipParams{CampaignID: campaignID, UserID: userID, Now: s.now()})
	switch {
	case err == nil:
		return campaign.Name, member.Role == string(authz.RoleMaster), nil
	case errors.Is(err, pgx.ErrNoRows):
		return campaign.Name, false, nil
	default:
		return "", false, fmt.Errorf("get membership: %w", err)
	}
}

// JoinAsPlayer makes userID an active player of the campaign, inside tx, with no step of
// approval: the master made the character they take (MR-049). Someone already in stays as
// they are, and a pending member (RN-15) becomes an ordinary one, which closes their join
// request. It returns the campaign's name.
func (s *Service) JoinAsPlayer(ctx context.Context, tx pgx.Tx, campaignID, userID string) (string, error) {
	q := s.queriesIn(tx)
	now := s.now()
	campaign, err := q.GetCampaign(ctx, campaignID)
	if err != nil {
		return "", fmt.Errorf("get campaign: %w", err)
	}
	member, err := q.GetMembership(ctx, campaignsdb.GetMembershipParams{CampaignID: campaignID, UserID: userID, Now: now})
	switch {
	case err == nil && member.Status == string(authz.StatusPending):
		if _, err := q.ActivatePendingMember(ctx, campaignsdb.ActivatePendingMemberParams{CampaignID: campaignID, UserID: userID, Now: now}); err != nil {
			return "", fmt.Errorf("activate pending member: %w", err)
		}
		return campaign.Name, nil
	case err == nil:
		return campaign.Name, nil // already a member (or the master)
	case !errors.Is(err, pgx.ErrNoRows):
		return "", fmt.Errorf("get membership: %w", err)
	}
	// A pending member past the deadline is no member, but the TTL job may not have
	// deleted the row yet: it goes before the new one.
	if _, err := q.DeleteExpiredPendingMember(ctx, campaignsdb.DeleteExpiredPendingMemberParams{CampaignID: campaignID, UserID: userID, Now: now}); err != nil {
		return "", fmt.Errorf("delete expired pending member: %w", err)
	}
	if _, err := q.InsertMember(ctx, campaignsdb.InsertMemberParams{
		CampaignID: campaignID, UserID: userID, Role: string(authz.RolePlayer), Status: string(authz.StatusActive),
	}); err != nil {
		return "", fmt.Errorf("insert player: %w", err)
	}
	return campaign.Name, nil
}
