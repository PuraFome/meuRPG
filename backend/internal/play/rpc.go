package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"uuid"

	"connectrpc.com/connect"
	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Every handler starts with one explicit check, authz.RequireCampaignRole
// for what only the master may do and authz.RequireCampaignMember for
// reads. The check's error is already the right Connect error, so handlers
// return it as is.

// StartGameSession implements playv1connect.PlayServiceHandler.
func (s *Service) StartGameSession(
	ctx context.Context,
	req *connect.Request[playv1.StartGameSessionRequest],
) (*connect.Response[playv1.StartGameSessionResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}

	now := s.now()
	var session playdb.GameSession
	var locked int64
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		// One open session per campaign. This check gives the clear error;
		// the unique index makes it hold when two starts race (below).
		_, err := q.GetOpenGameSession(ctx, m.CampaignID)
		switch {
		case err == nil:
			return errSessionOpen()
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the open session: %w", err)
		}
		number, err := q.NextSessionNumber(ctx, m.CampaignID)
		if err != nil {
			return fmt.Errorf("next session number: %w", err)
		}
		session, err = q.InsertGameSession(ctx, playdb.InsertGameSessionParams{
			CampaignID: m.CampaignID, SessionNumber: number, StartedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert game session: %w", err)
		}
		// RN-01, in the same transaction: the session and the lock happen
		// together, or not at all.
		locked, err = s.sheets.LockSheets(ctx, tx, m.CampaignID, now)
		if err != nil {
			return fmt.Errorf("lock sheets: %w", err)
		}
		return nil
	})
	if isUniqueViolation(err) {
		// Another start of the same campaign won the race.
		return nil, errSessionOpen()
	}
	if err != nil {
		return nil, s.dbError(ctx, "start a game session", err)
	}
	// A campaign has a handful of player characters, so the count always
	// fits; the check only makes the conversion provably safe.
	var lockedCount int32
	if locked > 0 && locked <= math.MaxInt32 {
		lockedCount = int32(locked)
	}
	return connect.NewResponse(&playv1.StartGameSessionResponse{
		GameSession:      sessionToProto(session),
		LockedSheetCount: lockedCount,
	}), nil
}

// EndGameSession implements playv1connect.PlayServiceHandler.
func (s *Service) EndGameSession(
	ctx context.Context,
	req *connect.Request[playv1.EndGameSessionRequest],
) (*connect.Response[playv1.EndGameSessionResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetGameSessionId())
	if err != nil {
		return nil, errSessionNotFound()
	}

	var session playdb.GameSession
	var ended bool // this call ended an open session
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		ended = false
		// Lock the row first: a vitals correction in progress finishes
		// before the session ends, and this call learns whether the session
		// was still open.
		current, err := q.GetGameSessionForUpdate(ctx, playdb.GetGameSessionForUpdateParams{CampaignID: m.CampaignID, ID: id.String()})
		if errors.Is(err, pgx.ErrNoRows) {
			return errSessionNotFound()
		}
		if err != nil {
			return fmt.Errorf("find game session: %w", err)
		}
		if current.EndedAt != nil {
			session = current // ended before: nothing changes
			return nil
		}
		if current.ShownImageKeep && current.ShownImageID != nil {
			// The image left with the players outlives the session (MR-028).
			if err := s.maps.LeaveImage(ctx, tx, m.CampaignID, *current.ShownImageID, s.now()); err != nil {
				return err
			}
		}
		session, err = q.EndGameSession(ctx, playdb.EndGameSessionParams{
			CampaignID: m.CampaignID, ID: id.String(), Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("end game session: %w", err)
		}
		ended = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "end a game session", err)
	}
	res := sessionToProto(session)
	if ended {
		// Every stream of the session ends (WatchGameSession).
		s.hub.Publish(m.CampaignID, live.Event{
			Audience: live.Audience{Everyone: true},
			Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_SessionEnded_{
				SessionEnded: &playv1.WatchGameSessionResponse_SessionEnded{GameSession: res},
			}},
		})
	}
	return connect.NewResponse(&playv1.EndGameSessionResponse{GameSession: res}), nil
}

// ListGameSessions implements playv1connect.PlayServiceHandler.
func (s *Service) ListGameSessions(
	ctx context.Context,
	req *connect.Request[playv1.ListGameSessionsRequest],
) (*connect.Response[playv1.ListGameSessionsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	sessions, err := s.queries.ListGameSessions(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list game sessions", err)
	}
	res := &playv1.ListGameSessionsResponse{}
	for _, session := range sessions {
		res.GameSessions = append(res.GameSessions, sessionToProto(session))
	}
	return connect.NewResponse(res), nil
}

func sessionToProto(s playdb.GameSession) *playv1.GameSession {
	out := &playv1.GameSession{
		Id:            s.ID,
		CampaignId:    s.CampaignID,
		SessionNumber: s.SessionNumber,
		StartedAt:     timestamppb.New(s.StartedAt),
	}
	if s.EndedAt != nil {
		out.EndedAt = timestamppb.New(*s.EndedAt)
	}
	return out
}

// errSessionOpen is StartGameSession's failed_precondition.
func errSessionOpen() error {
	return errBlocked(playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_SESSION_ALREADY_OPEN,
		"the campaign already has an open game session; end it first")
}

// errNoOpenSession is the live session's failed_precondition: the campaign
// has no open session.
func errNoOpenSession() error {
	return errBlocked(playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION,
		"the campaign has no open game session")
}

// errBlocked is PlayService's failed_precondition, with the
// GameSessionBlocked detail that tells the app why.
func errBlocked(reason playv1.GameSessionBlockedReason, msg string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&playv1.GameSessionBlocked{Reason: reason}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// errSessionNotFound is the answer for a session that is not in the
// campaign.
func errSessionNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("game session not found"))
}

// isUniqueViolation reports whether err is a unique violation (23505).
// Inside StartGameSession's transaction only game_sessions' unique indexes
// can raise it (one open session per campaign, and the session number), so
// it means that another start of the same campaign won the race.
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}

// dbError turns an error from the database, or from inside a transaction,
// into the Connect error the client gets, as in package campaigns.
func (s *Service) dbError(ctx context.Context, action string, err error) error {
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		return connectErr
	}
	s.logger.ErrorContext(ctx, "play: cannot "+action, "error", err)
	if _, ok := errors.AsType[*crdb.MaxRetriesExceededError](err); ok {
		return connect.NewError(connect.CodeAborted, errors.New("too many changes at the same time, please try again"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("cannot reach the database right now, please try again"))
}
