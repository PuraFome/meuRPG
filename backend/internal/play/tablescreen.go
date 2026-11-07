package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The table screen (issue #648): the TV at the table shows what the master chose. The choice is three
// columns of the open session (tv_mirror, tv_pin, tv_choice_revision); what the TV shows now is decided
// by resolveStage from facts read for the TV, never from the master's own reads.

// errStaleTableScreen is the answer to a choice written over one that changed since it was read.
func errStaleTableScreen() error {
	return connect.NewError(connect.CodeAborted, errors.New("the table screen's choice changed since you read it: read it again and try again"))
}

// tvChoiceOf is the master's stored choice.
func tvChoiceOf(session playdb.GameSession) tvChoice {
	return tvChoice{Mirror: session.TvMirror, Pin: playv1.TableScreenKind(session.TvPin)}
}

// GetTableScreen implements playv1connect.TableScreenServiceHandler.
func (s *Service) GetTableScreen(
	ctx context.Context,
	req *connect.Request[playv1.GetTableScreenRequest],
) (*connect.Response[playv1.GetTableScreenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	var session playdb.GameSession
	var facts tvFacts
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if session, err = openSessionWith(ctx, s.queriesIn(tx), m.CampaignID); err != nil {
			return err
		}
		facts, err = s.tvFactsOf(ctx, tx, session)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the table screen", err)
	}
	name, err := s.campaignNameOf(ctx, m)
	if err != nil {
		return nil, err
	}
	party, err := s.partyNames(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	choice := tvChoiceOf(session)
	pick := resolveStage(choice, facts)
	stage := &playv1.TableScreenStage{Kind: pick.Kind}
	projectStageBody(stage)
	return connect.NewResponse(&playv1.GetTableScreenResponse{
		ChoiceRevision: session.TvChoiceRevision,
		Choice:         &playv1.TableScreenChoice{Mirror: choice.Mirror, Pin: choice.Pin},
		Stage:          stage,
		Notice:         pick.Notice,
		CampaignName:   name,
		SessionNumber:  session.SessionNumber,
		Party:          party,
		Options:        tvOptions(facts),
	}), nil
}

// projectStageBody sets the content of the stage the kind asks for. Only the idle screen has one so
// far: it carries nothing. The other kinds leave the body unset until their projectors are written;
// each is a list of field copies from the reads a player with no character makes.
func projectStageBody(stage *playv1.TableScreenStage) {
	if stage.GetKind() == playv1.TableScreenKind_TABLE_SCREEN_KIND_IDLE {
		stage.Body = &playv1.TableScreenStage_Idle{Idle: &playv1.TvIdle{}}
	}
}

// campaignNameOf is the name of the master's campaign.
func (s *Service) campaignNameOf(ctx context.Context, m authz.Membership) (string, error) {
	campaigns, err := s.campaigns.ActiveCampaigns(ctx, m.UserID)
	if err != nil {
		return "", s.dbError(ctx, "read the campaign's name", err)
	}
	for _, c := range campaigns {
		if c.GetId() == m.CampaignID {
			return c.GetName(), nil
		}
	}
	return "", connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
}

// partyNames are the names of the living player characters: nobody dead, no NPC, no character waiting
// for the master's approval (CombatParty lists only the living, active ones).
func (s *Service) partyNames(ctx context.Context, campaignID string) ([]string, error) {
	party, err := s.roster.CombatParty(ctx, nil, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party", err)
	}
	names := make([]string, 0, len(party))
	for _, c := range party {
		if c.Player && !c.Reserved {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

// tvFactsOf reads what resolveStage needs of the open session, inside tx.
func (s *Service) tvFactsOf(ctx context.Context, tx pgx.Tx, session playdb.GameSession) (tvFacts, error) {
	var f tvFacts
	q := s.queriesIn(tx)
	if session.ShownImageID != nil {
		v, err := shownViewersOf(ctx, q, session)
		if err != nil {
			return f, err
		}
		f.ImageToTable = v.all
	}
	enc, err := q.GetOpenEncounter(ctx, session.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return f, err
	}
	f.CombatActive = err == nil && enc.Status == "active"
	if session.OpenScenePointID != nil {
		_, err := s.maps.ScenePoint(ctx, tx, session.CampaignID, *session.OpenScenePointID)
		if err != nil && connect.CodeOf(err) != connect.CodeNotFound { // not found: the point stopped being a scene
			return f, err
		}
		f.SceneOpen = err == nil
		f.SceneForAll = session.OpenSceneEveryone
	}
	if session.CurrentMapID != nil {
		fog, err := s.maps.MapFog(ctx, tx, session.CampaignID, *session.CurrentMapID)
		if err != nil && connect.CodeOf(err) != connect.CodeNotFound { // not found: the map is gone
			return f, err
		}
		f.HasMap = err == nil
		f.FogOn, f.GroupVision = fog.Fog, fog.GroupVision
	}
	return f, nil
}

// SetTableScreen implements playv1connect.TableScreenServiceHandler.
//
// One transaction locks the open session's row, checks the revision of the choice, writes the three
// columns and bumps the revision. It writes nothing else. After the commit the master's streams
// hear table_screen_changed; no player's stream does (RN-10).
func (s *Service) SetTableScreen(
	ctx context.Context,
	req *connect.Request[playv1.SetTableScreenRequest],
) (*connect.Response[playv1.SetTableScreenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	want := req.Msg.GetChoice()
	if want == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("choice is required"))
	}
	pin := want.GetPin()
	if _, known := playv1.TableScreenKind_name[int32(pin)]; !known {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pin is not a known item"))
	}
	if !want.GetMirror() && !validTvPin(pin) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pin must be the idle screen, the map, the image, the scene or the combat when the TV does not mirror"))
	}
	var updated playdb.GameSession
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		session, err := lockOpenSessionOrNone(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		if session.TvChoiceRevision != req.Msg.GetChoiceRevision() {
			return errStaleTableScreen()
		}
		keep := session.TvPin // a mirror choice with no pin keeps the pin it had
		if validTvPin(pin) {
			keep = int16(pin) //nolint:gosec // G115: a known kind, 1 to 5
		}
		updated, err = q.SetTableScreenChoice(ctx, playdb.SetTableScreenChoiceParams{ID: session.ID, TvMirror: want.GetMirror(), TvPin: keep})
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "set the table screen", err)
	}
	s.hub.Publish(m.CampaignID, live.Event{
		Audience: live.Audience{Master: true},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_TableScreenChanged_{
			TableScreenChanged: &playv1.WatchGameSessionResponse_TableScreenChanged{ChoiceRevision: updated.TvChoiceRevision},
		}},
	})
	choice := tvChoiceOf(updated)
	return connect.NewResponse(&playv1.SetTableScreenResponse{
		ChoiceRevision: updated.TvChoiceRevision,
		Choice:         &playv1.TableScreenChoice{Mirror: choice.Mirror, Pin: choice.Pin},
	}), nil
}

// WatchTableScreen implements playv1connect.TableScreenServiceHandler. The TV tab's stream is not
// served yet: the master is checked and the call answers `unimplemented`.
func (s *Service) WatchTableScreen(
	ctx context.Context,
	req *connect.Request[playv1.WatchTableScreenRequest],
	_ *connect.ServerStream[playv1.WatchTableScreenResponse],
) error {
	if _, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster); err != nil {
		return err
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("the table screen's stream is not served yet"))
}
