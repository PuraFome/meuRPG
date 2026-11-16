package characters

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/sessiontell"
)

// OptionSwap is one option of a table feature that a level-up gave up for another.
type OptionSwap struct {
	// FeatureKey is the table feature whose option pool the swap was made in (the group of the swap).
	FeatureKey string
	// GivenUp is the option the character no longer has, and Taken the option of the same pool it took in its place.
	GivenUp, Taken string
	// ClassKey is the class that gained the level.
	ClassKey string
	// ToLevel is the character's total level after the level-up, the number the level-up's own record
	// (ListLevelUps, `to_level`) holds.
	ToLevel int
}

// OptionSwaps is what another module hears when a level-up swaps the options of a table feature
// (LevelUpChoices.swap_feature_options). cmd/api may connect one with SetOptionSwaps; without it the
// level-up works the same and nobody is told.
type OptionSwaps interface {
	// OptionsGivenUp is called by LevelUpCharacter, in the level-up's own transaction tx (the one that holds the
	// character's row and the session's funnel), right after the new sheet and the level-up's record are
	// written and before the transaction commits, once for each level-up that swaps at least one option of a table
	// feature, with every swap of that level in swaps (never empty). It is not called for a preview, for a level-up
	// that is refused, or for a level without such a swap. The character is campaignID's characterID; what the
	// streams are told goes through t, once the transaction committed. An error aborts the level-up: nothing
	// of it is saved and the caller gets the error.
	OptionsGivenUp(ctx context.Context, tx pgx.Tx, campaignID, characterID string, swaps []OptionSwap, t sessiontell.Tell) error
}

// SetOptionSwaps connects the module that hears the swaps of a table feature's options. It is optional: nil, the
// default, calls nobody.
func (s *Service) SetOptionSwaps(h OptionSwaps) { s.optionSwaps = h }
