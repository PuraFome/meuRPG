package progression

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgconn"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
)

// errBlocked is ProgressionService's failed_precondition, with the XPBlocked
// detail that tells the app why. characterID and mode go in the detail where
// the reason uses them.
func errBlocked(reason progressionv1.XPBlockedReason, characterID string, mode campaignsv1.XpMode) error {
	msg := map[progressionv1.XPBlockedReason]string{
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_MODE_NOT_ALLOWED:       "this campaign's XP mode does not take this",
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_ENCOUNTER_NOT_ENDED:    "the combat has not ended",
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_ALREADY_AWARDED:        "the combat's XP was already given",
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_NOTHING_TO_GIVE:        "there is no XP to give each character",
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_NOTHING_TO_UNDO:        "there is no award to undo",
		progressionv1.XPBlockedReason_XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE: "only the campaign's living player characters can get XP",
	}[reason]
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&progressionv1.XPBlocked{Reason: reason, CharacterId: characterID, XpMode: mode}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// invalidArgument is an invalid_argument that names the field and the rule,
// never the value.
func invalidArgument(field string, err error) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s %w", field, err))
}

// isUniqueViolation reports whether err is a unique-index violation (23505)
// of the named index.
func isUniqueViolation(err error, name string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505" && pgErr.ConstraintName == name
}
