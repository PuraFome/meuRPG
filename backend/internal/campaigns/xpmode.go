package campaigns

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// CampaignXPMode returns how the campaign levels (RN-09): by defeated enemies,
// by gold or by milestones. Package progression reads it through its own
// interface, to know which awards fit the campaign. authz.ErrNotMember for a
// campaign that does not exist.
func (s *Service) CampaignXPMode(ctx context.Context, campaignID string) (campaignsv1.XpMode, error) {
	campaign, err := s.queries.GetCampaign(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return campaignsv1.XpMode_XP_MODE_UNSPECIFIED, authz.ErrNotMember
	}
	if err != nil {
		return campaignsv1.XpMode_XP_MODE_UNSPECIFIED, fmt.Errorf("get campaign: %w", err)
	}
	return xpModeFromDB[campaign.XpMode], nil
}
