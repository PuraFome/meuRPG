package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// arcaneRecoveryEvent is the payload of an arcane_recovery_used event: the slots that
// came back (level and count) and their combined level. Numbers only.
type arcaneRecoveryEvent struct {
	Slots  []arcaneSlot `json:"slots"`
	Levels int32        `json:"levels"`
}

type arcaneSlot struct {
	Level int32 `json:"level"`
	Count int32 `json:"count"`
}

// arcaneSlots reads the slots of the request: one entry per level, a count of at least 1.
func arcaneSlots(req *playv1.UseArcaneRecoveryRequest) (map[int]int, error) {
	if len(req.GetSlots()) == 0 || len(req.GetSlots()) > maxSlotLevel {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slots must name one to nine spell levels"))
	}
	out := map[int]int{}
	for _, s := range req.GetSlots() {
		level, count := int(s.GetLevel()), int(s.GetCount())
		if level < 1 || level > maxSlotLevel {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a slot level must be 1 to 9"))
		}
		if count < 1 || count > maxSlotsPerLevel {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the count of slots of a level must be at least 1"))
		}
		if _, dup := out[level]; dup {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("each slot level may appear once"))
		}
		out[level] = count
	}
	return out, nil
}

// maxSlotsPerLevel bounds the count a request names (no sheet has more slots of a level).
const maxSlotsPerLevel = 9

// UseArcaneRecovery implements playv1connect.ResourceServiceHandler: the wizard's
// recovery of expended spell slots after a short rest.
func (s *Service) UseArcaneRecovery(
	ctx context.Context,
	req *connect.Request[playv1.UseArcaneRecoveryRequest],
) (*connect.Response[playv1.UseArcaneRecoveryResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	characterID, err := uuid.Parse(req.Msg.GetCharacterId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	key, err := uuid.Parse(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	slots, err := arcaneSlots(req.Msg)
	if err != nil {
		return nil, err
	}
	keyText, charText := key.String(), characterID.String()
	hash := idem.Hash(req.Msg)

	var ev arcaneRecoveryEvent
	var after *playv1.CharacterVitals
	var repeated bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ev, after, repeated = arcaneRecoveryEvent{}, nil, false // a retry starts over
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, keyText, eventArcaneRecovery, hash)
		if err != nil {
			return err
		}
		if again {
			if done.CharacterID == nil || *done.CharacterID != charText {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			if err := json.Unmarshal(done.Payload, &ev); err != nil {
				return fmt.Errorf("read the arcane recovery event: %w", err)
			}
			repeated = true
			return nil
		}
		// The character's player or the master. Another player's character is not theirs.
		current, err := s.vitals.GetVitalsTx(ctx, tx, m.CampaignID, charText)
		if err != nil {
			return err
		}
		if m.Role != authz.RoleMaster && (current.GetPlayerUserId() == "" || current.GetPlayerUserId() != m.UserID) {
			return connect.NewError(connect.CodePermissionDenied, errors.New("only the character's player or the master may do this"))
		}
		if err := activeCombatBlocks(ctx, q, session.ID); err != nil {
			return err
		}
		_, afterVitals, levels, err := s.vitals.UseArcaneRecovery(ctx, tx, m.CampaignID, charText, slots)
		if err != nil {
			return resourceError(err)
		}
		// "when you finish a short rest": the latest rest of the session is a short one. Asked
		// after the keeper so a character that cannot use the feature is told so first; the
		// refusal rolls the keeper's write back with the transaction.
		kind, err := q.GetLatestRestKindInSession(ctx, session.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read the latest rest: %w", err)
		}
		if kind != rules.RestShort {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_SHORT_REST, "Arcane Recovery needs a short rest first")
		}
		after = afterVitals
		ev = arcaneRecoveryEvent{Levels: clamp32(levels, 1, 1<<10)}
		for level, count := range slots {
			ev.Slots = append(ev.Slots, arcaneSlot{Level: clamp32(level, 1, maxSlotLevel), Count: clamp32(count, 1, maxSlotsPerLevel)})
		}
		slices.SortFunc(ev.Slots, func(a, b arcaneSlot) int { return int(a.Level - b.Level) })
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventArcaneRecovery, ActorUserID: &m.UserID, CharacterID: &charText,
			Payload: payload, IdempotencyKey: &keyText, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "use arcane recovery", err)
	}
	if repeated {
		current, err := s.vitals.GetVitals(ctx, m.CampaignID, charText)
		if err != nil {
			return nil, s.dbError(ctx, "get vitals", err)
		}
		return connect.NewResponse(&playv1.UseArcaneRecoveryResponse{Vitals: current, RecoveredLevels: ev.Levels}), nil
	}
	s.publishVitals(m.CampaignID, after)
	logging.Event(ctx, s.logger, "arcane_recovery.used", slog.String("character_id", charText), slog.Int("levels", int(ev.Levels)))
	return connect.NewResponse(&playv1.UseArcaneRecoveryResponse{Vitals: after, RecoveredLevels: ev.Levels}), nil
}
