package play

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

const (
	defaultMessagePage = 50
	maxMessagePage     = 200
	// summaryMessages is how many lines the session summary carries at most.
	summaryMessages = 500
)

func errMessageNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("message not found"))
}

// ListSessionMessages implements playv1connect.PlayServiceHandler.
func (s *Service) ListSessionMessages(
	ctx context.Context,
	req *connect.Request[playv1.ListSessionMessagesRequest],
) (*connect.Response[playv1.ListSessionMessagesResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	sessionID, err := s.messagesSession(ctx, m.CampaignID, req.Msg.GetGameSessionId())
	if err != nil {
		return nil, err
	}
	size := int(req.Msg.GetPageSize())
	if size <= 0 {
		size = defaultMessagePage
	}
	size = min(size, maxMessagePage)
	v := viewer{master: m.Role == authz.RoleMaster, userID: m.UserID}
	var before *string
	if raw := req.Msg.GetBeforeId(); raw != "" {
		if before, err = s.readableCursor(ctx, m.CampaignID, raw, v); err != nil {
			return nil, err
		}
	}
	views, older, err := s.messagePage(ctx, sessionID, before, size, v)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ListSessionMessagesResponse{Messages: views, HasOlder: older, VoiceEnabled: s.msgs.voiceOn()}), nil
}

// messagesSession is the session whose lines are read: the one asked for, or the open one.
func (s *Service) messagesSession(ctx context.Context, campaignID, raw string) (string, error) {
	if raw == "" {
		session, err := s.queries.GetOpenGameSession(ctx, campaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errNoOpenSession()
		}
		if err != nil {
			return "", s.dbError(ctx, "find the open session", err)
		}
		return session.ID, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", errSessionNotFound()
	}
	session, err := s.queries.GetGameSessionInCampaign(ctx, playdb.GetGameSessionInCampaignParams{CampaignID: campaignID, ID: id.String()})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errSessionNotFound()
	}
	if err != nil {
		return "", s.dbError(ctx, "find the session", err)
	}
	return session.ID, nil
}

// readableCursor checks that the line a page starts before is one the viewer may read: any other is
// answered like one that does not exist, so the cursor tells nothing about hidden lines (RN-10).
func (s *Service) readableCursor(ctx context.Context, campaignID, raw string, v viewer) (*string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, errMessageNotFound()
	}
	row, err := s.readableMessage(ctx, campaignID, id.String(), v)
	if err != nil {
		return nil, err
	}
	return &row.ID, nil
}

// readableMessage is the line of the campaign if the viewer may read it, else errMessageNotFound: the
// same answer for a line that does not exist and one that is not theirs (RN-36).
func (s *Service) readableMessage(ctx context.Context, campaignID, id string, v viewer) (playdb.SessionMessage, error) {
	row, err := s.queries.GetSessionMessageForReader(ctx, playdb.GetSessionMessageForReaderParams{ID: id, CampaignID: campaignID, IsMaster: v.master, UserID: v.userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return playdb.SessionMessage{}, errMessageNotFound()
	}
	if err != nil {
		return playdb.SessionMessage{}, s.dbError(ctx, "find a message", err)
	}
	return row, nil
}

// messagePage reads up to size lines the viewer may read, newest first in the table and returned oldest
// first, and says whether older ones are left.
func (s *Service) messagePage(ctx context.Context, sessionID string, before *string, size int, v viewer) ([]*playv1.SessionMessage, bool, error) {
	rows, err := s.queries.ListSessionMessages(ctx, playdb.ListSessionMessagesParams{
		GameSessionID: sessionID, IsMaster: v.master, UserID: v.userID, BeforeID: before, PageSize: int32(size + 1), //nolint:gosec // G115: at most 201
	})
	if err != nil {
		return nil, false, s.dbError(ctx, "list the messages", err)
	}
	older := len(rows) > size
	rows = rows[:min(len(rows), size)]
	slices.Reverse(rows)
	views, err := s.messageViews(ctx, rows, v, nil)
	return views, older, err
}

// conversationOf is the lines of a session the member may read, for the summary: the last
// summaryMessages, oldest first.
func (s *Service) conversationOf(ctx context.Context, m authz.Membership, sessionID string) ([]*playv1.SessionMessage, bool, error) {
	return s.messagePage(ctx, sessionID, nil, summaryMessages, viewer{master: m.Role == authz.RoleMaster, userID: m.UserID})
}
