// Package play is about playing a campaign at the table. Today it starts,
// ends and lists game sessions (PlayService), which is what locks the
// players' sheets (RN-01, MR-006, MR-011). The live table (the notification
// to players, turns, actions and the session history, ADR-0005 and
// ADR-0007) comes later.
//
// Starting a session locks the sheets in the same transaction that opens
// it, through a SheetLocker: the characters module owns the characters
// table, so this package never touches it. cmd/api connects the two;
// neither package imports the other.
//
// The SQL lives in queries.sql, and sqlc turns it into package playdb.
// Every write runs inside db.InTx, which retries CockroachDB's serialization
// errors (40001).
package play

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/nostore"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// SheetLocker locks a campaign's player sheets when a game session starts
// (RN-01). The characters module implements it
// (characters.Service.LockSheets).
type SheetLocker interface {
	// LockSheets locks, inside tx, the sheets of the campaign's living
	// player characters that are still drafts, ends every permission to
	// edit a story in the campaign, and returns how many sheets it locked.
	LockSheets(ctx context.Context, tx pgx.Tx, campaignID string, at time.Time) (int64, error)
}

// Config holds what the play service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Sheets locks the players' sheets when a session starts. Required.
	Sheets SheetLocker
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
}

// Service implements the PlayService Connect API.
type Service struct {
	pool    *pgxpool.Pool
	queries *playdb.Queries
	sheets  SheetLocker
	logger  *slog.Logger
	now     func() time.Time
}

// The compiler checks that Service implements the handler.
var _ playv1connect.PlayServiceHandler = (*Service)(nil)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Pool == nil:
		return nil, errors.New("play: a Pool is required")
	case cfg.Sheets == nil:
		return nil, errors.New("play: Sheets is required")
	}
	s := &Service{
		pool:    cfg.Pool,
		queries: playdb.New(cfg.Pool),
		sheets:  cfg.Sheets,
		logger:  cfg.Logger,
		now:     cfg.Now,
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
// reads it back. *identity.Service is the real one; tests pass a fake.
type Sessions interface {
	// Interceptor finds the caller's session (from the session cookie).
	Interceptor() connect.Interceptor
	authz.Caller
}

// Mount registers PlayService on a mux. handle is usually
// httpserver.Server.Handle or http.ServeMux.Handle.
//
// sessions tells who is calling (the identity service in production), and
// members tells each caller's role in a campaign (the campaigns service).
// Mount adds, in this order, an interceptor that marks every response
// `Cache-Control: no-store`, the sessions interceptor, and the authz
// interceptor. opts are the Connect options shared by every service.
func (s *Service) Mount(handle func(pattern string, handler http.Handler), sessions Sessions, members authz.MembershipSource, opts ...connect.HandlerOption) {
	// Clip so append copies instead of writing into the caller's array.
	opts = append(slices.Clip(opts), connect.WithInterceptors(
		nostore.Interceptor(),                          // no response is cacheable
		sessions.Interceptor(),                         // who is calling
		authz.Interceptor(sessions, members, s.logger), // what they may do, memoized per request
	))
	handle(playv1connect.NewPlayServiceHandler(s, opts...))
}
