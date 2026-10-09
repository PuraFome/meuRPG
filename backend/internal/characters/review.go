package characters

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// "Pedir ajustes" (RN-15, MR-024).
//
// The master cannot only approve or reject a pending character (a rejection deletes
// it): he can send it back with a reason. The character stays PENDING, its player edits
// it and sends it again, and the master may approve or reject at any moment. The
// state of the request is a row of character_reviews; no row means REVIEW_AWAITING.
//
// The reason is free text of the master about the player, so it is personal data of the
// player (docs/privacy.md): only the master and the owning player read it, no log line
// carries it (only its length), and it is deleted when the character is approved or
// rejected and, by the table's foreign keys, when the character, the campaign or the
// player's account is deleted. After the resubmission it stays for the master's history
// until one of those.

// MaxReviewReasonLength is the longest reason, in characters.
const MaxReviewReasonLength = 500

// The database's values for character_reviews.status.
const (
	reviewChangesRequested = "changes_requested"
	reviewResubmitted      = "resubmitted"
)

// The events ReviewHost.PublishCharacterEvent tells the streams: the master asked for changes,
// the owner sent the character again, a dead character lives again. They are strings so that
// package play, which implements the interface, needs no import of this one.
const (
	hintChangesRequested = "changes_requested"
	hintResubmitted      = "resubmitted"
	hintRevived          = "revived"
)

// reviewToProto is a review row as the master and the owner read it.
func reviewToProto(r charactersdb.CharacterReview) *charactersv1.CharacterReview {
	out := &charactersv1.CharacterReview{
		Status:      charactersv1.Review_REVIEW_CHANGES_REQUESTED,
		Reason:      r.Reason,
		RequestedAt: timestamppb.New(r.RequestedAt),
	}
	if r.Status == reviewResubmitted {
		out.Status = charactersv1.Review_REVIEW_RESUBMITTED
	}
	if r.ResubmittedAt != nil {
		out.ResubmittedAt = timestamppb.New(*r.ResubmittedAt)
	}
	return out
}

// reviewStatusToProto is the status alone, which is all a list carries.
func reviewStatusToProto(status string) charactersv1.Review {
	switch status {
	case reviewChangesRequested:
		return charactersv1.Review_REVIEW_CHANGES_REQUESTED
	case reviewResubmitted:
		return charactersv1.Review_REVIEW_RESUBMITTED
	}
	return charactersv1.Review_REVIEW_AWAITING
}

// mayReadReview says whether the caller is the master or the character's owner: the only
// ones who read the review (RN-10).
func mayReadReview(m authz.Membership, row charactersdb.Character) bool {
	if isMaster(m) {
		return true
	}
	return row.PlayerUserID != nil && *row.PlayerUserID == m.UserID
}

// fillReview sets what a pending character carries for the master and the owner: the review
// (REVIEW_AWAITING when the master asked nothing) and whether the owner may send it again.
// Everyone else, and any other character, gets nothing. It reads outside a transaction.
func (s *Service) fillReview(ctx context.Context, c *charactersv1.Character, row charactersdb.Character, m authz.Membership) error {
	if row.Kind != kindPlayer || row.Status != statusPending || !mayReadReview(m, row) {
		return nil
	}
	r, err := s.queries.GetCharacterReview(ctx, row.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		c.Review = &charactersv1.CharacterReview{Status: charactersv1.Review_REVIEW_AWAITING}
		return nil
	case err != nil:
		return wrap("read the review", err)
	}
	c.Review = reviewToProto(r)
	c.CanResubmit = !isMaster(m) && r.Status == reviewChangesRequested
	return nil
}

// requiredKey checks the idempotency key of a call that needs one.
func requiredKey(raw string) (string, error) {
	key, err := idem.Clean(raw)
	if err != nil {
		return "", invalidArgument(fieldErr("idempotency_key", "must be 1 to 64 characters"))
	}
	if key == "" {
		return "", invalidArgument(fieldErr("idempotency_key", "is required"))
	}
	return key, nil
}

// keyOf is the key as it is stored: scoped to the campaign and the caller, so two people
// can reuse the same string (idem.Scope).
func keyOf(m authz.Membership, key string) *string {
	return idem.Scope(m.CampaignID+":"+m.UserID, key)
}

// RequestCharacterChanges implements charactersv1connect.CharacterServiceHandler.
func (s *Service) RequestCharacterChanges(
	ctx context.Context,
	req *connect.Request[charactersv1.RequestCharacterChangesRequest],
) (*connect.Response[charactersv1.RequestCharacterChangesResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	key, err := requiredKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	reason, err := names.CleanText(req.Msg.GetReason(), MaxReviewReasonLength)
	if err != nil {
		return nil, invalidArgument(fieldErr("reason", "must be at most %d characters", MaxReviewReasonLength))
	}
	if reason == "" {
		return nil, invalidArgument(fieldErr("reason", "is required"))
	}
	scoped, hash := keyOf(m, key), idem.Hash(req.Msg)

	content, err := s.contentFor(ctx, nil, m.CampaignID) // before the write: a failure after the commit would make the client retry it
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	var row charactersdb.Character
	var replayed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		replayed = false
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
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_PENDING, row.ID)
		}
		if row.PlayerUserID == nil {
			return invalidArgument(fieldErr("character_id", "has no player to answer: approve or reject it"))
		}
		prior, err := q.GetCharacterReview(ctx, row.ID)
		switch {
		case err == nil && prior.RequestKey != nil && *prior.RequestKey == *scoped:
			if err := idem.SameRequest(prior.RequestHash, hash); err != nil {
				return err
			}
			replayed = true // the first call did it: nothing changes, nothing is published
			return nil
		case err != nil && !errors.Is(err, pgx.ErrNoRows):
			return wrap("read the review", err)
		}
		if _, err := q.UpsertCharacterReviewRequest(ctx, charactersdb.UpsertCharacterReviewRequestParams{
			CharacterID: row.ID, CampaignID: m.CampaignID, PlayerUserID: *row.PlayerUserID, Reason: reason,
			Now: s.now(), RequestKey: scoped, RequestHash: hash,
		}); err != nil {
			return wrap("request changes", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "request changes of a character", err)
	}
	if !replayed {
		logging.Event(ctx, s.logger, "character.changes_requested", slog.String("character_id", row.ID), slog.Int("reason_length", utf8.RuneCountInString(reason)))
		s.publishCharacterEvent(ctx, m.CampaignID, row, hintChangesRequested)
	}
	c, err := s.character(ctx, content, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.RequestCharacterChangesResponse{Character: c}), nil
}

// ResubmitCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) ResubmitCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.ResubmitCharacterRequest],
) (*connect.Response[charactersv1.ResubmitCharacterResponse], error) {
	m, err := authz.RequireCampaignMemberOrPending(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	key, err := requiredKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	scoped, hash := keyOf(m, key), idem.Hash(req.Msg)

	content, err := s.contentFor(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	var row charactersdb.Character
	var replayed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		replayed = false
		q := s.queries.WithTx(tx)
		var err error
		row, err = visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		// Only the owner: the master and any other member get the answer of a character
		// that is not there, so nobody probes an ID.
		if isMaster(m) || row.PlayerUserID == nil || *row.PlayerUserID != m.UserID {
			return errCharacterNotFound()
		}
		if row.Status != statusPending {
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_PENDING, row.ID)
		}
		prior, err := q.GetCharacterReview(ctx, row.ID)
		switch {
		case err == nil && prior.ResubmitKey != nil && *prior.ResubmitKey == *scoped:
			if err := idem.SameRequest(prior.ResubmitHash, hash); err != nil {
				return err
			}
			replayed = true
			return nil
		case errors.Is(err, pgx.ErrNoRows) || (err == nil && prior.Status != reviewChangesRequested):
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NO_CHANGES_REQUESTED, row.ID)
		case err != nil:
			return wrap("read the review", err)
		}
		now := s.now()
		if _, err := q.MarkCharacterReviewResubmitted(ctx, charactersdb.MarkCharacterReviewResubmittedParams{
			CharacterID: row.ID, Now: &now, ResubmitKey: scoped, ResubmitHash: hash,
		}); err != nil {
			return wrap("resubmit a character", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "resubmit a character", err)
	}
	if !replayed {
		logging.Event(ctx, s.logger, "character.resubmitted", slog.String("character_id", row.ID))
		s.publishCharacterEvent(ctx, m.CampaignID, row, hintResubmitted)
	}
	c, err := s.character(ctx, content, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.ResubmitCharacterResponse{Character: c}), nil
}

// ReviewHost is what the characters module needs from the play module for the
// reviews and the revivals: the streams, and a combat that holds the character. cmd/api
// connects play.Service (SetReviewHost); without it, a review or a revival still works and
// nothing reaches a stream or a combat.
type ReviewHost interface {
	// PublishCharacterEvent tells the master and the character's owner (ownerUserID, empty
	// for none) that a review changed or a character lives again. A hint with no content
	// (RN-10): the reason is read, never streamed. Call it after the commit.
	PublishCharacterEvent(ctx context.Context, campaignID, ownerUserID, characterID, event string)
	// ReviveInCombat puts the character back in the campaign's combat that is not ended,
	// inside tx, if it is one of its combatants: no longer defeated, no death save counted,
	// still in its place in the order, and acting on its next turn. It writes the line of
	// the combat's log ("O mestre reviveu <name>") and returns the combat's ID, empty when
	// the character is in none (the caller then writes the line to the session).
	ReviveInCombat(ctx context.Context, tx pgx.Tx, campaignID, characterID, actorUserID string, at time.Time) (encounterID string, err error)
	// PublishEncounterChanged tells every stream that the combat changed. Call it after the
	// commit.
	PublishEncounterChanged(ctx context.Context, campaignID, encounterID string)
}

// SetReviewHost connects the play module.
func (s *Service) SetReviewHost(h ReviewHost) { s.reviewHost = h }

// publishCharacterEvent tells the master and the owner of row, after the commit.
func (s *Service) publishCharacterEvent(ctx context.Context, campaignID string, row charactersdb.Character, event string) {
	if s.reviewHost == nil {
		return
	}
	s.reviewHost.PublishCharacterEvent(ctx, campaignID, deref(row.PlayerUserID), row.ID, event)
}

// reviewStatuses reads the status of each open review of the campaign, by character ID.
func (s *Service) reviewStatuses(ctx context.Context, campaignID string) (map[string]string, error) {
	rows, err := s.queries.ListCharacterReviewStatuses(ctx, campaignID)
	if err != nil {
		return nil, wrap("list the reviews", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.CharacterID] = r.Status
	}
	return out, nil
}
