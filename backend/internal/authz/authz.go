// Package authz answers "may the caller do this in this campaign?" (ADR-0011).
//
// Roles are per campaign (RN-05): a row in campaign_members says a user is
// the master or a player of one campaign. That row is the only source of
// truth, and it is read on every request, so removing someone from a
// campaign takes effect on their very next call. There is no token carrying
// roles and no cache between requests.
//
// Handlers call one explicit check before doing anything:
//
//	userID, err := authz.RequireSignedIn(ctx)                           // anyone signed in
//	m, err := authz.RequireCampaignMember(ctx, campaignID)             // any member
//	m, err := authz.RequireCampaignRole(ctx, campaignID, authz.RoleMaster) // masters only
//	m, err := authz.RequireCampaignMemberOrPending(ctx, campaignID)    // any member, or a pending one (pending.go)
//	if err != nil {
//		return nil, err // already the right Connect error
//	}
//
// A membership is active or pending (RN-15, MR-024). Only an active member
// is a member: RequireCampaignMember and RequireCampaignRole answer a
// pending member exactly as they answer someone outside the campaign. The
// few calls a pending member may make are listed in pending.go, and only
// RequireCampaignMemberOrPending lets them in, for those calls alone.
//
// The errors are chosen so that nobody learns about campaigns they are not
// in:
//
//	no valid session                        unauthenticated
//	not a member, or no such campaign       not_found (the two look the same)
//	a member without the needed role        permission_denied
//
// Who is calling comes from a Caller: the identity module in production,
// which reads the session its own interceptor found, and a fake in tests.
// This package never sets the caller itself, and nothing outside identity's
// interceptor can (docs/arquitetura.md, "Quem está chamando").
//
// Interceptor must wrap the Connect service, after identity's session
// interceptor. It gives each request a small memo, so a request that checks
// the same campaign twice reads campaign_members once. Without it, every
// check fails closed.
//
// A stream lives much longer than a request, so its handler also calls
// RecheckCampaignMember from time to time (recheck.go): the login session
// and the membership are read again, skipping the memo.
package authz

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"uuid"

	"connectrpc.com/connect"
)

// Role is what a member may do in one campaign.
type Role string

// The roles. There is no hierarchy: a master is not also a player, so a
// check that any member passes is RequireCampaignMember, not
// RequireCampaignRole(ctx, id, RolePlayer).
const (
	// RoleMaster runs the campaign ("mestre" in the app).
	RoleMaster Role = "master"
	// RolePlayer plays in it ("jogador" in the app).
	RolePlayer Role = "player"
)

// Status says whether a membership is in force (RN-15, MR-024). It is
// campaign_members.status.
type Status string

// The statuses.
const (
	// StatusActive is a member: everything this package talks about.
	StatusActive Status = "active"
	// StatusPending is someone who accepted an invite that requires
	// approval and waits for the master to approve their character. They
	// are not a member yet: see pending.go.
	StatusPending Status = "pending"
)

// Membership is the caller's place in one campaign.
type Membership struct {
	CampaignID string
	UserID     string
	Role       Role
	// Pending is true for a pending member (RN-15, MR-024). Only
	// RequireCampaignMemberOrPending ever returns such a Membership, and
	// only for the calls in pendingMayCall (pending.go); a handler that got
	// one must keep the caller to what the allowance is for.
	Pending bool
}

// Caller tells who is calling. The identity module implements it
// (identity.Service.UserID) from the session that its interceptor found for
// the request; tests pass a fake. It only reads: there is no way to set the
// caller from outside the identity module.
type Caller interface {
	// UserID returns the signed-in caller's user ID, or an `unauthenticated`
	// Connect error to return as is.
	UserID(ctx context.Context) (string, error)
}

// ErrNotMember is what a MembershipSource returns when the user is not a
// member of the campaign, including when the campaign does not exist.
var ErrNotMember = errors.New("authz: not a member of the campaign")

// MembershipSource looks up a user's place in a campaign. The campaigns
// module implements it with one primary-key read of campaign_members.
type MembershipSource interface {
	// CampaignMembership returns userID's role and status in campaignID, or
	// ErrNotMember.
	CampaignMembership(ctx context.Context, campaignID, userID string) (Role, Status, error)
}

// RequireSignedIn returns the caller's user ID, or a Connect error to return
// as is: `unauthenticated` without a session. It is the whole check for
// methods that anyone signed in may call, such as creating a campaign.
func RequireSignedIn(ctx context.Context) (string, error) {
	memo, err := memoFrom(ctx)
	if err != nil {
		return "", err
	}
	return memo.caller.UserID(ctx)
}

// RequireCampaignMember returns the caller's membership in the campaign, or
// a Connect error to return as is: `unauthenticated` without a session,
// `not_found` if the caller is not a member (or is only a pending member,
// or there is no such campaign, or campaignID is not even a UUID). Handlers
// should use the returned Membership.CampaignID, which is in canonical
// form, from then on.
func RequireCampaignMember(ctx context.Context, campaignID string) (Membership, error) {
	m, _, err := lookUp(ctx, campaignID)
	if err != nil {
		return Membership{}, err
	}
	if m.Pending {
		// Not a member yet (RN-15): the same answer as for a stranger.
		return Membership{}, errNotFound()
	}
	return m, nil
}

// lookUp finds the caller's membership in the campaign, active or pending,
// with the request's memo. Its errors are already the right Connect errors;
// every Require* function decides what a pending membership may do.
func lookUp(ctx context.Context, campaignID string) (Membership, *memo, error) {
	memo, err := memoFrom(ctx)
	if err != nil {
		return Membership{}, nil, err
	}
	userID, err := memo.caller.UserID(ctx)
	if err != nil {
		return Membership{}, nil, err
	}

	// An ID that is not a UUID cannot name a campaign. Parsing here also
	// keeps garbage away from the database, and the canonical form
	// (lowercase, with dashes) is what Membership.CampaignID returns.
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return Membership{}, nil, errNotFound()
	}
	campaignID = id.String()

	role, status, err := memo.membership(ctx, campaignID, userID)
	switch {
	case err == nil:
		// Anything but "active" counts as pending, the most limited place:
		// a status this code does not know can never grant more.
		return Membership{CampaignID: campaignID, UserID: userID, Role: role, Pending: status != StatusActive}, memo, nil
	case errors.Is(err, ErrNotMember):
		return Membership{}, nil, errNotFound()
	default:
		memo.logger.ErrorContext(ctx, "cannot check campaign membership", "error", err)
		return Membership{}, nil, connect.NewError(connect.CodeUnavailable, errors.New("cannot check permissions right now, please try again"))
	}
}

// RequireCampaignRole is RequireCampaignMember, plus `permission_denied`
// when the caller's role in the campaign is not exactly role. A pending
// member gets `not_found`, as from RequireCampaignMember.
func RequireCampaignRole(ctx context.Context, campaignID string, role Role) (Membership, error) {
	m, err := RequireCampaignMember(ctx, campaignID)
	if err != nil {
		return Membership{}, err
	}
	if m.Role != role {
		return Membership{}, noStore(connect.NewError(connect.CodePermissionDenied, errors.New("only the campaign's "+string(role)+" can do this")))
	}
	return m, nil
}

// errNotFound is the answer for a campaign the caller cannot see. Its text
// is the same whether or not the campaign exists.
func errNotFound() error {
	return noStore(connect.NewError(connect.CodeNotFound, errors.New("campaign not found")))
}

func noStore(err *connect.Error) *connect.Error {
	err.Meta().Set("Cache-Control", "no-store")
	return err
}

// Interceptor returns a Connect interceptor that gives every request (and
// every stream) its own memo: who the caller is (from caller), which RPC
// was called (for pendingMayCall), and the memberships already looked up
// (from source). Errors from source are logged to logger, which may be nil
// for slog.Default().
//
// The memo lives as long as the request. For a long-lived stream that is
// the whole stream, so a streaming handler that must notice a removal while
// it runs checks again with RecheckCampaignMember (ADR-0011).
func Interceptor(caller Caller, source MembershipSource, logger *slog.Logger) connect.Interceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &interceptor{caller: caller, source: source, logger: logger}
}

type interceptor struct {
	caller Caller
	source MembershipSource
	logger *slog.Logger
}

// withMemo returns ctx with a new memo for a call to procedure (such as
// "/meurpg.campaigns.v1.CampaignService/GetCampaign"), which Connect's
// handler fills in and a client cannot change.
func (i *interceptor) withMemo(ctx context.Context, procedure string) context.Context {
	return context.WithValue(ctx, memoKey{}, &memo{caller: i.caller, source: i.source, logger: i.logger, procedure: procedure})
}

// WrapUnary implements connect.Interceptor.
func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		return next(i.withMemo(ctx, req.Spec().Procedure), req)
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(i.withMemo(ctx, conn.Spec().Procedure), conn)
	}
}

// Middleware is Interceptor for plain HTTP routes, such as the image upload
// and download (package maps): it gives every request its own memo, so the
// same Require* checks work in the handler. Mount it inside the route, so
// the memo's "procedure" is the route's pattern (such as "POST
// /uploads/images", from http.Request.Pattern). No route is in
// pendingMayCall, so a pending member gets `not_found` from every check, as
// from any call outside that list.
//
// Who is calling still comes from caller, which must be able to find the
// request's session: identity's AuthenticateRequest has to run first.
func Middleware(caller Caller, source MembershipSource, logger *slog.Logger) func(http.Handler) http.Handler {
	i := Interceptor(caller, source, logger).(*interceptor)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(i.withMemo(r.Context(), r.Pattern)))
		})
	}
}

// memoKey is the context key for the request's memo.
type memoKey struct{}

// memoFrom returns the request's memo. Without one, the service was mounted
// without Interceptor: a programming error, so refuse rather than guess.
func memoFrom(ctx context.Context) (*memo, error) {
	m, ok := ctx.Value(memoKey{}).(*memo)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("authorization is not configured for this service"))
	}
	return m, nil
}

// memo remembers, for one request, the memberships already looked up. A
// "not a member" answer is remembered too; errors are not, so a later check
// in the same request tries the database again.
type memo struct {
	caller Caller
	source MembershipSource
	logger *slog.Logger
	// procedure is the RPC being served, set by the interceptor.
	procedure string

	mu      sync.Mutex // a handler may check from several goroutines
	members map[memoEntry]memoResult
}

type memoEntry struct{ campaignID, userID string }

type memoResult struct {
	role   Role
	status Status
	err    error // nil or ErrNotMember
}

func (m *memo) membership(ctx context.Context, campaignID, userID string) (Role, Status, error) {
	key := memoEntry{campaignID, userID}

	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.members[key]; ok {
		return r.role, r.status, r.err
	}

	// Holding the lock during the lookup is fine: requests do not share a
	// memo, and a request rarely checks two campaigns at the same time.
	role, status, err := m.source.CampaignMembership(ctx, campaignID, userID)
	if err != nil && !errors.Is(err, ErrNotMember) {
		return "", "", err
	}
	if m.members == nil {
		m.members = map[memoEntry]memoResult{}
	}
	m.members[key] = memoResult{role: role, status: status, err: err}
	return role, status, err
}
