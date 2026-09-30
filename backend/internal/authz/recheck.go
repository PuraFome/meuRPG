package authz

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

// Streams check again (ADR-0005, ADR-0011).
//
// A unary call is checked once, at its start, and ends a moment later. A
// stream (PlayService.WatchGameSession) may stay open for many minutes, and
// the request's memo would keep answering from its first read. So a
// streaming handler calls RecheckCampaignMember from time to time: it reads
// the login session and the membership again, from the database, and the
// stream ends with the same error a new call would get.

// SessionRechecker is a Caller that can also read the caller's login
// session again, to notice a sign-out, a revoked session or an expired one
// while a stream runs. The identity module implements it
// (identity.Service.RecheckSession).
type SessionRechecker interface {
	Caller
	// RecheckSession reads the login session behind ctx again, and returns
	// an `unauthenticated` Connect error, to return as is, if it is no
	// longer valid. Other errors mean the store could not answer.
	RecheckSession(ctx context.Context) error
}

// RecheckCampaignMember is RequireCampaignMember for a long-lived stream
// that checks again while it runs: it reads the caller's login session and
// their membership again, skipping the request's memo, and answers exactly
// as RequireCampaignMember would for a new call (`unauthenticated`,
// `not_found`, or `unavailable` when the database does not answer).
//
// It needs the service to be mounted with Interceptor, and a Caller that
// implements SessionRechecker; without either, it fails closed
// (`internal`).
func RecheckCampaignMember(ctx context.Context, campaignID string) (Membership, error) {
	current, err := memoFrom(ctx)
	if err != nil {
		return Membership{}, err
	}
	rechecker, ok := current.caller.(SessionRechecker)
	if !ok {
		// A programming error: the stream cannot notice a sign-out, so it
		// must not go on.
		return Membership{}, connect.NewError(connect.CodeInternal, errors.New("the session cannot be checked again for this service"))
	}
	if err := rechecker.RecheckSession(ctx); err != nil {
		if _, ok := errors.AsType[*connect.Error](err); ok {
			return Membership{}, err
		}
		current.logger.ErrorContext(ctx, "cannot check the session again", "error", err)
		return Membership{}, connect.NewError(connect.CodeUnavailable, errors.New("cannot check the session right now, please try again"))
	}
	// A new, empty memo: the membership is read from the database again.
	fresh := &memo{caller: current.caller, source: current.source, logger: current.logger, procedure: current.procedure}
	return RequireCampaignMember(context.WithValue(ctx, memoKey{}, fresh), campaignID)
}
