package characters

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"connectrpc.com/connect"
	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// errCharacterNotFound is the answer for a character that is not in the
// campaign, or that the caller may not see. Its text is the same in both
// cases (ADR-0011).
func errCharacterNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("character not found"))
}

// errStaleRevision is the answer when the character changed since the
// client read it (AIP-154).
func errStaleRevision() error {
	return connect.NewError(connect.CodeAborted, errors.New("the character changed since you opened it; reload it and try again"))
}

// errBlocked is CharacterService's failed_precondition, with the
// CharacterBlocked detail that tells the app why.
func errBlocked(reason charactersv1.CharacterBlockedReason, characterID string) error {
	msg := map[charactersv1.CharacterBlockedReason]string{
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_SHEET_LOCKED:            "the sheet is locked: a game session has started, so only the master edits it",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD:          "the character is dead, so only the master edits its sheet",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS: "you already have a living character in this campaign",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_STORY_LOCKED:            "the story is locked: ask the master to allow editing it",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_PENDING:             "the character does not wait for approval: once approved, it stays in the campaign",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL:       "the character waits for your approval: approve or reject it instead",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_HIT_POINTS_AVERAGE_ONLY: "the table gives everybody the average hit points: there is no roll",
	}[reason]
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&charactersv1.CharacterBlocked{Reason: reason, CharacterId: characterID}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// errBlockedByContent is errBlocked for a table entry that stops the call
// (ARCHIVED_CONTENT, TABLE_CONTENT_STAYS): the detail names the key.
func errBlockedByContent(reason charactersv1.CharacterBlockedReason, characterID, contentKey string) error {
	msg := map[charactersv1.CharacterBlockedReason]string{
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ARCHIVED_CONTENT:     "the master archived a table entry this sheet picks: it is not a new choice any more",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_SWITCHED_OFF_CONTENT: "the master switched off an option this sheet picks: it is not a new choice for players",
		charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_TABLE_CONTENT_STAYS:  "the sheet uses the table's own content, so it cannot go to another campaign",
	}[reason]
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&charactersv1.CharacterBlocked{Reason: reason, CharacterId: characterID, ContentKey: contentKey}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// errPermission is the answer for a member whose role may not do this.
func errPermission(msg string) error {
	return connect.NewError(connect.CodePermissionDenied, errors.New(msg))
}

// invalidArgument turns a fieldError into the error the client gets. The
// message names the field and the rule, never the value, which could be
// personal data.
func invalidArgument(err error) error {
	if fe, ok := errors.AsType[*fieldError](err); ok {
		out := connect.NewError(connect.CodeInvalidArgument, fe)
		if detail, detailErr := connect.NewErrorDetail(&charactersv1.InvalidField{Field: fe.field}); detailErr == nil {
			out.AddDetail(detail)
		}
		return out
	}
	return err
}

// parseUUID returns an ID in canonical form, or false when it is not a
// UUID: such an ID names nothing, and is answered as "not found" without a
// trip to the database.
func parseUUID(id string) (string, bool) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", false
	}
	return u.String(), true
}

// isUniqueViolation reports whether err is a unique-index violation (23505)
// of the named index or constraint. CockroachDB names it in the error's
// constraint field.
func isUniqueViolation(err error, name string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

// dbError turns an error from the database, or from inside a transaction,
// into the Connect error the client gets. Connect errors pass through as
// they are: they are the answers the handlers chose. Anything else is
// logged, without personal data, and hidden behind a generic message.
func (s *Service) dbError(ctx context.Context, action string, err error) error {
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		return connectErr
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// The character passed its checks and was deleted before the next
		// query: it is gone now.
		return errCharacterNotFound()
	}
	s.logger.ErrorContext(ctx, "characters: cannot "+action, "error", err)
	if errors.Is(err, errCorruptDocument) {
		return connect.NewError(connect.CodeInternal, errors.New("this character cannot be read right now"))
	}
	if _, ok := errors.AsType[*crdb.MaxRetriesExceededError](err); ok {
		// db.InTx retried a serialization conflict (40001) and gave up.
		return connect.NewError(connect.CodeAborted, errors.New("too many changes at the same time, please try again"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("cannot reach the database right now, please try again"))
}

// wrap adds what failed to an error from a query, keeping it unwrappable
// (%w), so db.InTx still sees a 40001 and retries.
func wrap(what string, err error) error {
	return fmt.Errorf("%s: %w", what, err)
}
