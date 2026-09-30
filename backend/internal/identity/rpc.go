package identity

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/identity/v1"
)

// sessionKey is the context key for the current Session.
type sessionKey struct{}

// SessionFromContext returns the session that Interceptor found for this
// request, if any.
func SessionFromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	return s, ok
}

// RequireSession returns the caller's session, or an `unauthenticated`
// Connect error to return as is. It fails closed: a handler whose service
// was mounted without Interceptor always gets the error.
func RequireSession(ctx context.Context) (Session, error) {
	if s, ok := SessionFromContext(ctx); ok {
		return s, nil
	}
	err := connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to continue"))
	err.Meta().Set("Cache-Control", "no-store")
	return Session{}, err
}

// Interceptor returns a Connect interceptor that reads the session cookie,
// looks the session up and, when it is valid, puts it in the context for
// SessionFromContext and RequireSession.
//
// A missing or invalid session is not an error here: some RPCs work signed
// out, so each handler decides with RequireSession. A database failure is
// an error (`unavailable`), because answering `unauthenticated` would make
// the app think the user signed out.
func (s *Service) Interceptor() connect.Interceptor {
	return &sessionInterceptor{service: s}
}

type sessionInterceptor struct {
	service *Service
}

// WrapUnary implements connect.Interceptor.
func (i *sessionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		ctx, err := i.service.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (i *sessionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *sessionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.service.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// authenticate adds the request's session to ctx, when there is a valid one.
func (s *Service) authenticate(ctx context.Context, header http.Header) (context.Context, error) {
	token, ok := cookieValue(header, SessionCookieName)
	if !ok {
		return ctx, nil
	}
	session, err := s.lookupSession(ctx, token)
	switch {
	case err == nil:
		return context.WithValue(ctx, sessionKey{}, session), nil
	case errors.Is(err, errNoSession):
		return ctx, nil
	default:
		s.logger.ErrorContext(ctx, "cannot look up the session", "error", err)
		return ctx, connect.NewError(connect.CodeUnavailable, errors.New("cannot check the session right now, please try again"))
	}
}

// GetMe implements identityv1connect.IdentityServiceHandler.
func (s *Service) GetMe(
	ctx context.Context,
	_ *connect.Request[identityv1.GetMeRequest],
) (*connect.Response[identityv1.GetMeResponse], error) {
	session, err := RequireSession(ctx)
	if err != nil {
		return nil, err
	}
	res := connect.NewResponse(&identityv1.GetMeResponse{
		User:             &identityv1.User{Id: session.UserID},
		SessionExpiresAt: timestamppb.New(session.ExpiresAt),
	})
	res.Header().Set("Cache-Control", "no-store")
	return res, nil
}

// SignOut implements identityv1connect.IdentityServiceHandler. It deletes
// the session row, so the token stops working at once on every server
// instance, and clears the cookie.
func (s *Service) SignOut(
	ctx context.Context,
	_ *connect.Request[identityv1.SignOutRequest],
) (*connect.Response[identityv1.SignOutResponse], error) {
	clearCookie := expiredCookie(SessionCookieName).String()

	session, err := RequireSession(ctx)
	if err != nil {
		// Also clear a stale cookie (expired or already revoked), so the
		// browser stops sending it.
		if connectErr, ok := errors.AsType[*connect.Error](err); ok {
			connectErr.Meta().Add("Set-Cookie", clearCookie)
		}
		return nil, err
	}
	if err := s.store.RevokeSession(ctx, session.ID); err != nil {
		s.logger.ErrorContext(ctx, "cannot revoke the session", "error", err)
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("cannot sign out right now, please try again"))
	}

	res := connect.NewResponse(&identityv1.SignOutResponse{})
	res.Header().Add("Set-Cookie", clearCookie)
	res.Header().Set("Cache-Control", "no-store")
	return res, nil
}

// disabledService answers every IdentityService RPC with `unavailable`, for
// a server where sign-in is not configured (see MountDisabled).
type disabledService struct{}

// GetMe implements identityv1connect.IdentityServiceHandler.
func (disabledService) GetMe(
	context.Context,
	*connect.Request[identityv1.GetMeRequest],
) (*connect.Response[identityv1.GetMeResponse], error) {
	return nil, connect.NewError(connect.CodeUnavailable, errDisabled)
}

// SignOut implements identityv1connect.IdentityServiceHandler.
func (disabledService) SignOut(
	context.Context,
	*connect.Request[identityv1.SignOutRequest],
) (*connect.Response[identityv1.SignOutResponse], error) {
	return nil, connect.NewError(connect.CodeUnavailable, errDisabled)
}
