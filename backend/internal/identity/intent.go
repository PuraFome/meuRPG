package identity

import (
	"context"
	"fmt"
	"maps"
	"regexp"
)

// A sign-in intent is something the user asked for while signed out, which
// needs them signed in: today, accepting a campaign invite. The app sends it
// with the sign-in itself, the server keeps it inside the login state, and
// the callback completes it right after the session exists. Nothing about
// it waits in the browser (no localStorage or sessionStorage), and nothing
// secret goes in a URL (ADR-0009):
//
//	POST /auth/login    return_to=/invite&intent=campaign_invite&intent_payload=<token>
//	  handler.Prepare(<token>)            returns what to keep: the token's hash
//	  saved in oidc_login_states          single use, at most 10 minutes
//	GET /auth/callback
//	  session created, as for any sign-in
//	  handler.Complete(ctx, who, hash)    returns where to go: /campaigns/<id>
//
// This package does not know what an intent means, and it does not import
// the modules that do. They provide one IntentHandler per kind, and
// cmd/api wires them in through Config.Intents.
//
// Complete acts as a user, so the user comes as a SignedIn: a value only
// this package can fill, and only right after it created that user's
// session. No other code can call Complete for an arbitrary user.

// MaxIntentDataBytes is the most an IntentHandler's Prepare may return. The
// oidc_login_states_intent CHECK says the same.
const MaxIntentDataBytes = 256

const (
	// maxIntentPayloadLength bounds intent_payload before any handler sees
	// it. An invite token is 43 characters.
	maxIntentPayloadLength = 1024

	// maxLoginFormBytes bounds the whole POST /auth/login body: return_to,
	// intent and intent_payload at their longest, URL-encoded, fit in it.
	maxLoginFormBytes = 8 << 10 // 8 KiB
)

// intentKindPattern is what an intent kind looks like, e.g.
// campaign_invite. The oidc_login_states_intent CHECK allows 1 to 32
// characters.
var intentKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// SignedIn is the user who has just signed in, as IntentHandler.Complete
// receives it. It is a capability: its field is unexported and there is no
// constructor, so only this package can make one with a user in it, and it
// does so only in the callback, right after it created that user's session.
// Code elsewhere can only write SignedIn{}, which holds no user, and every
// handler must refuse that (UserID returns "").
type SignedIn struct {
	userID string
}

// UserID returns the ID of the user who has just signed in, or "" for the
// zero value, which a handler must refuse.
func (s SignedIn) UserID() string { return s.userID }

// IntentHandler completes one kind of sign-in intent.
type IntentHandler interface {
	// Prepare runs when the sign-in starts, before the browser goes to the
	// provider. It checks the payload the app sent and returns what to keep
	// until the callback: the least that Complete needs, and never a secret
	// as it came (keep its hash instead). It may return at most
	// MaxIntentDataBytes.
	//
	// An error means the payload is not valid: the sign-in does not start,
	// and the browser gets a 400. The error is logged, so its text must not
	// include the payload.
	Prepare(payload string) ([]byte, error)

	// Complete runs in the callback, right after the session of who was
	// created, with what Prepare returned. It returns the path on this site
	// to send the browser to. It must refuse, with an error, a who whose
	// UserID is "": that SignedIn did not come from this package.
	//
	// A non-nil error means the intent failed. The sign-in stands anyway:
	// the session is kept, the error is logged (so it must not include
	// secrets), and the browser goes to redirectPath if it is set (an error
	// page, say), or else to return_to. Either way, redirectPath must pass
	// the same checks as return_to, or return_to is used instead.
	Complete(ctx context.Context, who SignedIn, data []byte) (redirectPath string, err error)
}

// checkIntents validates the intent handlers given to New and returns a
// copy, so a later change to the caller's map cannot change the Service.
func checkIntents(intents map[string]IntentHandler) (map[string]IntentHandler, error) {
	for kind, handler := range intents {
		if !intentKindPattern.MatchString(kind) {
			return nil, fmt.Errorf("identity: intent kind %q must match %s", kind, intentKindPattern)
		}
		if handler == nil {
			return nil, fmt.Errorf("identity: intent %q has no handler", kind)
		}
	}
	return maps.Clone(intents), nil
}

// prepareIntent runs the Prepare step of the named intent. A loginError
// means the request is refused; its reason is safe to log.
func (s *Service) prepareIntent(kind, payload string) ([]byte, *loginError) {
	handler, ok := s.intents[kind]
	if !ok {
		// The kind came from the client, so it is not logged.
		return nil, badLogin("unknown_intent", nil)
	}
	if len(payload) > maxIntentPayloadLength {
		return nil, badLogin("intent_payload_too_long", nil)
	}
	data, err := handler.Prepare(payload)
	if err != nil {
		return nil, badLogin("invalid_intent_payload", fmt.Errorf("%s: %w", kind, err))
	}
	if len(data) > MaxIntentDataBytes {
		// A bug in the handler, not in the request.
		return nil, unavailable("intent_data_too_long", fmt.Errorf("%s returned %d bytes, more than %d", kind, len(data), MaxIntentDataBytes))
	}
	if data == nil {
		data = []byte{} // set: "an intent with empty data", not "no intent"
	}
	return data, nil
}

// afterSignIn completes the sign-in's intent, if it has one, and returns
// where to send the browser: always a path on this site. It never fails the
// sign-in; the session already exists.
func (s *Service) afterSignIn(ctx context.Context, session Session, login LoginState) string {
	// return_to passed safeReturnTo before it was stored; checking again
	// costs nothing and keeps this redirect safe on its own.
	fallback, ok := safeReturnTo(login.ReturnTo)
	if !ok {
		fallback = "/"
	}
	if login.IntentKind == "" {
		return fallback
	}

	// IntentKind was checked against s.intents when it was stored, so it is
	// safe to log.
	handler, ok := s.intents[login.IntentKind]
	if !ok {
		// Only possible if the server restarted, with other handlers,
		// between the login and the callback.
		s.logger.WarnContext(ctx, "sign-in intent skipped", "intent", login.IntentKind, "reason", "unknown_intent")
		return fallback
	}
	// The only place a SignedIn gets a user: the session was just created.
	path, err := handler.Complete(ctx, SignedIn{userID: session.UserID}, login.IntentData)
	if err != nil {
		s.logger.WarnContext(ctx, "sign-in intent failed", "intent", login.IntentKind, "error", err)
	} else {
		s.logger.InfoContext(ctx, "sign-in intent completed", "intent", login.IntentKind)
	}
	if path == "" {
		return fallback
	}
	safe, ok := safeReturnTo(path)
	if !ok {
		s.logger.ErrorContext(ctx, "sign-in intent returned a path outside this site; using return_to", "intent", login.IntentKind)
		return fallback
	}
	return safe
}
