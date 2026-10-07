// Package notes is the players' private notes (MR-030, Etapa 8, D6): what a
// player writes down during the campaign, tagged with a scene the group
// discovered, listed next to the clues the master revealed to them
// (NotesService).
//
// The notes are the player's alone. The master does not read them (question
// 60), and neither does another player: every query filters by the author, so
// another person's note is never found, and a call by the campaign's master
// answers `not_found`, as a note that is not there (RN-20). A note's text is
// free text a person writes, so it is never logged and never goes in an
// event.
//
// The scenes and the clues are the maps module's tables: which scenes the
// group discovered, and which clues the master revealed to each player. The
// notes module reads both through the Scenes interface, which cmd/api connects
// (maps.SessionMaps), so no package imports another's internals. The table
// player_notes is this module's. Deleting the account deletes the notes (the
// foreign key); there is no way yet to leave a campaign or be removed from it
// as an active member, and the day there is, it must delete that member's
// notes in the same transaction.
//
// The SQL lives in queries.sql, and sqlc turns it into package notesdb. Every
// write runs inside db.InTx, which retries CockroachDB's serialization errors
// (40001).
package notes

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1/notesv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/notes/link"
	"github.com/PuraFome/meuRPG/backend/internal/notes/notesdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/nostore"
	"github.com/PuraFome/meuRPG/backend/internal/platform/rpcerr"
)

// The limits (proposals of the plan, D6).
const (
	// MaxNotes is how many notes a player may have in a campaign. The clues
	// the master revealed do not count.
	MaxNotes = 300
	// maxNoteText is the longest note, in characters (player_notes_text_length).
	maxNoteText = 2000
)

// Scenes is what the notes need from the maps module (maps.SessionMaps
// implements it).
type Scenes interface {
	// DiscoveredScenes returns the scenes the campaign's group discovered
	// (MR-030), with their current names, oldest discovery first.
	DiscoveredScenes(ctx context.Context, campaignID string) ([]link.Scene, error)
	// ReceivedClues returns the clues revealed to the player in the campaign
	// (MR-029), newest first, with the text they were given.
	ReceivedClues(ctx context.Context, campaignID, userID string) ([]link.Clue, error)
}

// Config holds what the notes service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Scenes tells the discovered scenes and the received clues. Required.
	Scenes Scenes
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
}

// Service implements the NotesService Connect API.
type Service struct {
	pool    *pgxpool.Pool
	queries *notesdb.Queries
	scenes  Scenes
	logger  *slog.Logger
	now     func() time.Time
}

// The compiler checks that Service implements the handler.
var _ notesv1connect.NotesServiceHandler = (*Service)(nil)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Pool == nil:
		return nil, errors.New("notes: a Pool is required")
	case cfg.Scenes == nil:
		return nil, errors.New("notes: Scenes is required")
	}
	s := &Service{pool: cfg.Pool, queries: notesdb.New(cfg.Pool), scenes: cfg.Scenes, logger: cfg.Logger, now: cfg.Now}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// Sessions is what this package needs to know who is calling: the Connect
// interceptor that finds the caller's session, and the authz.Caller that reads
// it back. *identity.Service is the real one; tests pass a fake.
type Sessions interface {
	Interceptor() connect.Interceptor
	authz.Caller
}

// Mount registers NotesService on a mux. handle is usually
// httpserver.Server.Handle or http.ServeMux.Handle.
//
// sessions tells who is calling (the identity service in production), and
// members tells each caller's role in a campaign (the campaigns service). The
// service gets, in this order, an interceptor that marks every response
// `Cache-Control: no-store`, the sessions interceptor, and the authz
// interceptor. opts are the Connect options shared by every service.
func (s *Service) Mount(handle func(pattern string, handler http.Handler), sessions Sessions, members authz.MembershipSource, opts ...connect.HandlerOption) {
	// Clip so append copies instead of writing into the caller's array.
	opts = append(slices.Clip(opts), connect.WithInterceptors(
		nostore.Interceptor(),                          // no response is cacheable
		sessions.Interceptor(),                         // who is calling
		authz.Interceptor(sessions, members, s.logger), // what they may do, memoized per request
	))
	handle(notesv1connect.NewNotesServiceHandler(s, opts...))
}

// dbError maps an error from the database to the Connect error the client gets
// (see platform/rpcerr).
func (s *Service) dbError(ctx context.Context, action string, err error) error {
	return rpcerr.FromDB(ctx, s.logger, "notes", action, err)
}
