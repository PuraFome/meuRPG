package play

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What the session shows at the table, one of each, side by side:
//   - the current map (game_sessions.current_map_id, SetCurrentMap), which
//     the players see and which setting reveals (RN-10);
//   - a gallery image the master shows (game_sessions.shown_image_id,
//     SetShownImage, MR-028): a handout, such as a portrait or a letter. It
//     reveals nothing else.
//
// The maps, their points and tokens, and the gallery are the maps module's;
// this package only keeps what is on screen, and carries the maps' changes
// on the live stream. The two modules need each other, so each declares
// what it needs, the other implements it, and cmd/api connects them:
//   - here, MapKeeper (maps.SessionMaps): check and reveal the current map,
//     inside this package's transaction, and read the shown image;
//   - there, maps.LiveSession (this Service: OnScreen and Publish, below): a
//     player also sees the current map and the shown image, and the maps'
//     changes go out on the stream.

// SetCurrentMap implements playv1connect.PlayServiceHandler.
func (s *Service) SetCurrentMap(
	ctx context.Context,
	req *connect.Request[playv1.SetCurrentMapRequest],
) (*connect.Response[playv1.SetCurrentMapResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, err := optionalID(req.Msg.GetMapId(), "map not found")
	if err != nil {
		return nil, err
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		// Lock the open session, like every change made during it: an end
		// in progress finishes first, and this call then sees no session.
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		if mapID != nil {
			// The players see the current map, so it is revealed with it,
			// in the same transaction (RN-10).
			if err := s.maps.RevealMap(ctx, tx, m.CampaignID, *mapID, s.now()); err != nil {
				return err
			}
		}
		if _, err := q.SetCurrentMap(ctx, playdb.SetCurrentMapParams{ID: session.ID, CurrentMapID: mapID}); err != nil {
			return fmt.Errorf("set the current map: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set the current map", err)
	}

	current := deref(mapID)
	s.Publish(m.CampaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CurrentMapChanged_{
		CurrentMapChanged: &playv1.WatchGameSessionResponse_CurrentMapChanged{MapId: current},
	}})
	return connect.NewResponse(&playv1.SetCurrentMapResponse{CurrentMapId: current}), nil
}

// SetShownImage implements playv1connect.PlayServiceHandler.
func (s *Service) SetShownImage(
	ctx context.Context,
	req *connect.Request[playv1.SetShownImageRequest],
) (*connect.Response[playv1.SetShownImageResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	imageID, err := optionalID(req.Msg.GetImageId(), "image not found")
	if err != nil {
		return nil, err
	}
	var shown *playv1.ShownImage
	if imageID != nil {
		// The image must be the campaign's. The foreign key keeps it from
		// disappearing before the commit (below).
		if shown, err = s.maps.ShownImage(ctx, m.CampaignID, *imageID); err != nil {
			return nil, s.dbError(ctx, "find the image to show", err)
		}
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		if _, err := q.SetShownImage(ctx, playdb.SetShownImageParams{ID: session.ID, ShownImageID: imageID}); err != nil {
			return fmt.Errorf("set the shown image: %w", err)
		}
		return nil
	})
	if isForeignKeyViolation(err) {
		// The master deleted the image in another tab meanwhile.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("image not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "set the shown image", err)
	}

	s.Publish(m.CampaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_ShownImageChanged_{
		ShownImageChanged: &playv1.WatchGameSessionResponse_ShownImageChanged{Image: shown},
	}})
	return connect.NewResponse(&playv1.SetShownImageResponse{ShownImage: shown}), nil
}

// OnScreen returns what the campaign's open game session shows: the IDs of
// its current map and of the gallery image the master shows, each "" when
// there is none, both "" when no session is open. It implements
// maps.LiveSession, in one read; its errors are ordinary errors, for the
// maps module to log.
func (s *Service) OnScreen(ctx context.Context, campaignID string) (currentMapID, shownImageID string, err error) {
	row, err := s.queries.GetOnScreen(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil // no open session
	}
	if err != nil {
		return "", "", fmt.Errorf("read what the session shows: %w", err)
	}
	return deref(row.CurrentMapID), deref(row.ShownImageID), nil
}

// Publish sends an event on the campaign's live streams: to the master's
// always, and to the players' only when players is true. The maps module
// decides that for its own changes, because only it knows what each change
// touches (RN-10). Without an open session nobody is subscribed, and it
// does nothing. It implements maps.LiveSession; ev must not be changed
// after the call.
func (s *Service) Publish(campaignID string, players bool, ev *playv1.WatchGameSessionResponse) {
	s.hub.Publish(campaignID, live.Event{Audience: live.Audience{Master: true, Everyone: players}, Message: ev})
}

// optionalID reads an optional ID from a request: nil when empty, a
// `not_found` with notFound when it is not a UUID (it names nothing).
func optionalID(raw, notFound string) (*string, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New(notFound))
	}
	text := id.String()
	return &text, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// isForeignKeyViolation reports whether err is a foreign key violation
// (23503).
func isForeignKeyViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23503"
}
