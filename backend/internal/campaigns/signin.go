package campaigns

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/platform/secret"
)

// InviteIntentKind is the sign-in intent that accepts an invite. It lets a
// signed-out player open an invite link and join the campaign in one go,
// with nothing stored in the browser (ADR-0009):
//
//  1. The invite page (/invite#t=<token>) posts a form to POST /auth/login
//     with intent=campaign_invite and intent_payload=<token>.
//  2. Prepare checks the token's format and returns its SHA-256, which the
//     identity module keeps in the login state (10 minutes, single use).
//     The token itself is never stored.
//  3. After the provider, the callback creates the session and calls
//     Complete with the new user as an identity.SignedIn, which only the
//     identity module can fill. Complete accepts the invite for that user
//     exactly as the AcceptInvite RPC would (acceptInvite), and says where
//     to go.
//
// The browser then lands on /campaigns/<campaign_id>, or on
// /invite/error?reason=<reason> if the invite did not work. The reasons are
// the InviteFailure* constants. Someone who has just become a pending member
// (an invite with approval, RN-15) lands on
// /campaigns/<campaign_id>/characters/new instead: the character they
// create there is what the master approves.
const InviteIntentKind = "campaign_invite"

// Why accepting an invite at sign-in failed: the reason in
// /invite/error?reason=<reason>.
const (
	// InviteFailureExpired: the invite's time ran out.
	InviteFailureExpired = "expired"
	// InviteFailureRevoked: the master revoked it.
	InviteFailureRevoked = "revoked"
	// InviteFailureUsedUp: it was used as many times as it allows, maybe
	// by someone else with a leaked link.
	InviteFailureUsedUp = "used_up"
	// InviteFailureNotFound: no invite has this token (or its campaign was
	// deleted).
	InviteFailureNotFound = "not_found"
	// InviteFailureInvalid: what was kept at login start is not a token
	// hash. Only a bug could cause it.
	InviteFailureInvalid = "invalid"
	// InviteFailureUnavailable: the database did not answer; trying the
	// link again may work.
	InviteFailureUnavailable = "unavailable"
)

// InviteIntent returns the handler for InviteIntentKind, for
// identity.Config.Intents.
func (s *Service) InviteIntent() identity.IntentHandler {
	return inviteIntent{s: s}
}

// inviteIntent implements identity.IntentHandler for invites.
type inviteIntent struct {
	s *Service
}

// Prepare implements identity.IntentHandler: it keeps only the token's
// hash. A payload that cannot be a token is refused before the user is sent
// to the provider.
func (inviteIntent) Prepare(payload string) ([]byte, error) {
	hash, ok := secret.Hash(payload)
	if !ok {
		return nil, errors.New("the payload is not an invite token")
	}
	return hash, nil
}

// Complete implements identity.IntentHandler: it accepts the invite for
// the user who has just signed in, and returns the campaign's page, or the
// invite error page with the reason. A member of the campaign (the master
// included) goes to the campaign's page, and no use of the invite is spent.
//
// The user comes as an identity.SignedIn, so only the identity module's
// callback can make Complete act as someone. The zero value, which is all
// other code can build, is refused before anything touches the database.
func (i inviteIntent) Complete(ctx context.Context, who identity.SignedIn, tokenHash []byte) (string, error) {
	userID := who.UserID()
	if userID == "" {
		return "", errors.New("campaign invite: no signed-in user")
	}
	return i.complete(ctx, userID, tokenHash)
}

// complete is Complete once the user is known. The returned error never
// includes the token's hash: it goes to the log.
func (i inviteIntent) complete(ctx context.Context, userID string, tokenHash []byte) (string, error) {
	if len(tokenHash) != sha256.Size {
		return failedInvitePath(InviteFailureInvalid), errors.New("campaign invite: the stored data is not a token hash")
	}
	joined, err := i.s.acceptInvite(ctx, tokenHash, userID)
	if err != nil {
		reason := inviteFailure(err)
		if reason == InviteFailureUnavailable {
			// The caller logs a warning without the cause; this is the
			// database error, for whoever is on call.
			i.s.logger.ErrorContext(ctx, "campaigns: cannot accept an invite at sign-in", "error", err)
		}
		return failedInvitePath(reason), fmt.Errorf("campaign invite: %s", reason)
	}
	return joinedPath(joined), nil
}

// joinedPath is where the app goes after an invite was accepted: the
// campaign's page, or, for someone who has just become a pending member
// (RN-15, MR-024), straight to creating the character the master will
// approve. A pending member who was already pending goes to the campaign's
// page, which shows the character they already created.
func joinedPath(joined joinResult) string {
	if joined.pending && !joined.alreadyMember {
		return "/campaigns/" + joined.campaign.ID + "/characters/new"
	}
	return "/campaigns/" + joined.campaign.ID
}

// inviteFailure turns an error from acceptInvite into the reason for the
// invite error page.
func inviteFailure(err error) string {
	if unusable, ok := errors.AsType[*unusableInviteError](err); ok {
		switch unusable.state {
		case campaignsv1.InviteState_INVITE_STATE_EXPIRED:
			return InviteFailureExpired
		case campaignsv1.InviteState_INVITE_STATE_REVOKED:
			return InviteFailureRevoked
		case campaignsv1.InviteState_INVITE_STATE_USED_UP:
			return InviteFailureUsedUp
		}
		return InviteFailureInvalid
	}
	if errors.Is(err, errNoInvite) {
		return InviteFailureNotFound
	}
	return InviteFailureUnavailable
}

// failedInvitePath is the app's page that explains why an invite did not
// work. The reason is one of the InviteFailure* constants, never a value
// from the request, so the URL holds nothing secret or personal.
func failedInvitePath(reason string) string {
	return "/invite/error?reason=" + reason
}
