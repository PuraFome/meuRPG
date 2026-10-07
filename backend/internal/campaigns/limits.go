package campaigns

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

// DefaultMaxCampaignsPerUser is how many campaigns one account may be master
// of (RN-30). A table is one master and up to six players, so a handful is
// plenty; the limit stops one account from filling the database, and the
// gallery quota of each, with campaigns nobody plays.
const DefaultMaxCampaignsPerUser = 10

// canCreate reports whether the caller may create campaigns at all (RN-30).
// With no allow-list, anyone may. With one, only a user with a verified
// e-mail on the list may: the provider vouches for the address, so it cannot
// be claimed by someone else. It reads through the pool, so it must run
// outside a transaction.
func (s *Service) canCreate(ctx context.Context, userID string) (bool, error) {
	if len(s.creators) == 0 {
		return true, nil
	}
	emails, err := s.profiles.VerifiedEmails(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("read the caller's verified e-mails: %w", err)
	}
	return slices.ContainsFunc(emails, func(e string) bool {
		return slices.Contains(s.creators, strings.ToLower(e))
	}), nil
}

// errCreationRefused is CreateCampaign's refusal (RN-30), with the
// CampaignCreationRefused detail the app shows its message from.
func errCreationRefused(reason campaignsv1.CampaignCreationRefusedReason, maxCampaigns int) error {
	var err *connect.Error
	switch reason {
	case campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_LIMIT_REACHED:
		err = connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("you are already the master of %d campaigns, the most one account may have", maxCampaigns))
	default:
		err = connect.NewError(connect.CodePermissionDenied, errors.New("this server does not let this account create campaigns"))
	}
	detail, detailErr := connect.NewErrorDetail(&campaignsv1.CampaignCreationRefused{Reason: reason, MaxCampaigns: int32(maxCampaigns)}) //nolint:gosec // G115: the config caps it at 1000
	if detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}
