// Package campaigns manages campaigns, their members and the invites that
// let new players in (MR-001, MR-002, MR-003).
//
// A campaign's master creates it and shares invite links; a player signs in
// and accepts an invite, which makes them a player of the campaign. A
// signed-out player can do both at once: the invite rides the sign-in as an
// identity sign-in intent (signin.go). Who may
// do what is decided by package authz from campaign_members, which this
// package owns: Service implements authz.MembershipSource.
//
// The SQL lives in queries.sql, and sqlc turns it into package campaignsdb.
// Every write runs inside db.InTx, which retries CockroachDB's
// serialization errors (40001).
package campaigns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
)

// MaxNameLength is the longest campaign name, in characters. The
// campaigns_name_length CHECK says the same.
const MaxNameLength = 80

// Invite rules. RN-07 is still open, so the defaults follow ADR-0009's
// proposal (one use, 7 days) and the master can change them within limits.
const (
	DefaultInviteUses = 1
	// MaxInviteUses keeps an invite from becoming a public link: a table
	// rarely has more than a handful of players.
	MaxInviteUses = 20

	DefaultInviteLifetime = 7 * 24 * time.Hour
	MinInviteLifetime     = 5 * time.Minute
	// MaxInviteLifetime matches the campaign_invites_lifetime CHECK and
	// bounds how long an invite row lives (docs/privacidade.md).
	MaxInviteLifetime = 30 * 24 * time.Hour
)

// Profiles tells what to call users. The identity module implements it
// (identity.PostgresStore.DisplayNames), so this package never reads the
// users table itself.
type Profiles interface {
	// DisplayNames returns the display names of the given users, keyed by
	// user ID. Users without one are left out.
	DisplayNames(ctx context.Context, userIDs []string) (map[string]string, error)
}

// Config holds what the campaigns service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Profiles gives members' display names. Required.
	Profiles Profiles
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now. Tests move it
	// forward, e.g. past an invite's 7 days.
	Now func() time.Time
}

// Service implements the CampaignService Connect API and
// authz.MembershipSource.
type Service struct {
	pool     *pgxpool.Pool
	queries  *campaignsdb.Queries
	profiles Profiles
	logger   *slog.Logger
	now      func() time.Time
}

// The compiler checks that Service implements both interfaces.
var (
	_ campaignsv1connect.CampaignServiceHandler = (*Service)(nil)
	_ authz.MembershipSource                    = (*Service)(nil)
)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	if cfg.Pool == nil {
		return nil, errors.New("campaigns: a Pool is required")
	}
	if cfg.Profiles == nil {
		return nil, errors.New("campaigns: Profiles is required")
	}
	s := &Service{
		pool:     cfg.Pool,
		queries:  campaignsdb.New(cfg.Pool),
		profiles: cfg.Profiles,
		logger:   cfg.Logger,
		now:      cfg.Now,
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// Sessions is what this package needs to know who is calling: an
// interceptor that finds the caller's session, and the authz.Caller that
// reads it back. *identity.Service is the real one. Tests pass a fake, so
// nothing here, or in package authz, can set the caller itself
// (docs/arquitetura.md, "Quem está chamando").
type Sessions interface {
	// Interceptor finds the caller's session (from the session cookie).
	Interceptor() connect.Interceptor
	authz.Caller
}

// Mount registers CampaignService on a mux. handle is usually
// httpserver.Server.Handle or http.ServeMux.Handle.
//
// sessions tells who is calling (the identity service in production). Mount
// adds its interceptor, then the authz interceptor, backed by sessions and
// by this service's campaign_members, and one that marks every response
// `Cache-Control: no-store`. opts are the Connect options shared by every
// service.
func (s *Service) Mount(handle func(pattern string, handler http.Handler), sessions Sessions, opts ...connect.HandlerOption) {
	// Clip so append copies instead of writing into the caller's array.
	opts = append(slices.Clip(opts), connect.WithInterceptors(
		noStore{},                                // no response is cacheable
		sessions.Interceptor(),                   // who is calling
		authz.Interceptor(sessions, s, s.logger), // what they may do, memoized per request
	))
	handle(campaignsv1connect.NewCampaignServiceHandler(s, opts...))
}

// noStore marks every response, errors included, `Cache-Control: no-store`:
// they all describe the caller's campaigns. It uses Set, so a response that
// already says so (identity's unauthenticated error does) is not sent the
// header twice.
type noStore struct{}

// WrapUnary implements connect.Interceptor.
func (noStore) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		res, err := next(ctx, req)
		if err != nil {
			if connectErr, ok := errors.AsType[*connect.Error](err); ok {
				connectErr.Meta().Set("Cache-Control", "no-store")
			}
			return nil, err
		}
		res.Header().Set("Cache-Control", "no-store")
		return res, nil
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (noStore) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (noStore) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		conn.ResponseHeader().Set("Cache-Control", "no-store")
		return next(ctx, conn)
	}
}

// CampaignRole implements authz.MembershipSource with one primary-key read
// of campaign_members.
func (s *Service) CampaignRole(ctx context.Context, campaignID, userID string) (authz.Role, error) {
	role, err := s.queries.GetMemberRole(ctx, campaignsdb.GetMemberRoleParams{CampaignID: campaignID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", authz.ErrNotMember
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	return authz.Role(role), nil
}
