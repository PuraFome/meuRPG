package characters

import (
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/authz"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// refuseFeatChange refuses a sheet whose feats differ from the stored ones when the caller is
// not the campaign's master (MR-025). A player's feats come from the guided level-up and from
// the table's choices, never from the sheet editor: only the master adds or removes a feat
// there. stored is nil when the sheet is being created (a player's new sheet has no feat). The
// order of the keys does not matter. It answers `permission_denied`, as the other master-only
// writes of this service do.
func refuseFeatChange(m authz.Membership, stored, sheet *charactersv1.FullSheet) error {
	if isMaster(m) {
		return nil
	}
	before, after := slices.Clone(stored.GetFeatKeys()), slices.Clone(sheet.GetFeatKeys())
	slices.Sort(before)
	slices.Sort(after)
	if slices.Equal(before, after) {
		return nil
	}
	return errPermission("only the campaign's master adds or removes the feats of a sheet: a player's feats come from the guided level-up")
}
