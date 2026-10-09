package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

const eventGameTimeAdvanced = "game_time_advanced"

// maxGameTimeStep is the most a call moves game time on: a day.
const maxGameTimeStep = 24 * 3600

type gameTimeEvent struct {
	Seconds int32 `json:"seconds"`
	Ended   int32 `json:"ended"`
}

// AdvanceGameTime implements playv1connect.LastingEffectServiceHandler.
func (s *Service) AdvanceGameTime(
	ctx context.Context,
	req *connect.Request[playv1.AdvanceGameTimeRequest],
) (*connect.Response[playv1.AdvanceGameTimeResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := uuid.Parse(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	if req.Msg.GetSeconds() < 1 || req.Msg.GetSeconds() > maxGameTimeStep {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("seconds must be 1 to 86400"))
	}
	keyText, hash := key.String(), idem.Hash(req.Msg)
	var ended int32
	var told bool
	var vitals []*playv1.CharacterVitals
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ended, told, vitals = 0, false, nil
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, keyText, eventGameTimeAdvanced, hash)
		if err != nil {
			return err
		}
		if again {
			var ev gameTimeEvent
			_ = json.Unmarshal(done.Payload, &ev)
			ended = ev.Ended
			return nil
		}
		if err := activeCombatBlocks(ctx, q, session.ID); err != nil {
			return err
		}
		before, err := q.ListCharacterEffectsOfCampaign(ctx, m.CampaignID)
		if err != nil {
			return fmt.Errorf("list the effects: %w", err)
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), kind: eventGameTimeAdvanced, actorUserID: m.UserID, svc: s, master: true})
		if err != nil {
			return err
		}
		if err := s.advanceGameTime(ctx, c, req.Msg.GetSeconds()); err != nil {
			return err
		}
		vitals = c.told
		after, err := q.ListCharacterEffectsOfCampaign(ctx, m.CampaignID)
		if err != nil {
			return fmt.Errorf("list the effects: %w", err)
		}
		ended = int32(len(before) - len(after)) //nolint:gosec // a handful of effects
		payload, err := json.Marshal(gameTimeEvent{Seconds: req.Msg.GetSeconds(), Ended: ended})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventGameTimeAdvanced, ActorUserID: &m.UserID,
			Payload: payload, IdempotencyKey: &keyText, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		told = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "move game time on", err)
	}
	if told {
		s.publishCastsChanged(m.CampaignID, false)
		s.publishVitalsOf(m.CampaignID, vitals)
	}
	return connect.NewResponse(&playv1.AdvanceGameTimeResponse{EffectsEnded: ended}), nil
}
