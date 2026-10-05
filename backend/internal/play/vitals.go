package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The kinds of session_events rows (session_events_kind_valid).
const eventCharacterVitalsAdjusted = "character_vitals_adjusted"

// AdjustCharacterVitals implements playv1connect.PlayServiceHandler: the
// master's correction of a character's vitals (RN-02).
//
// One transaction locks the open session's row, checks the idempotency
// key, changes the vitals (through the VitalsKeeper) and appends the
// session event. The lock makes two corrections in the same session take
// turns, so the events get their numbers in order. Only after the commit
// does the change go out on the live streams.
func (s *Service) AdjustCharacterVitals(
	ctx context.Context,
	req *connect.Request[playv1.AdjustCharacterVitalsRequest],
) (*connect.Response[playv1.AdjustCharacterVitalsResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
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
	keyText, charText := key.String(), characterID.String()

	var after *playv1.CharacterVitals
	var repeated bool
	var touched *playdb.Encounter
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		after, repeated, touched = nil, false, nil // a retry starts over
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession() // RN-02: during the session
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}

		done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{
			GameSessionID: session.ID, IdempotencyKey: &keyText,
		})
		switch {
		case err == nil:
			if done.Kind != eventCharacterVitalsAdjusted || done.CharacterID == nil || *done.CharacterID != charText {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			repeated = true // a retry of a correction already made
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the event of this idempotency key: %w", err)
		}

		// Healing from 0 also resets the death saves of the character's combatant
		// in the session's combat (RN-03).
		before, adjusted, err := s.changeVitals(ctx, q, tx, session.ID, m.CampaignID, charText, req.Msg)
		if err != nil {
			return err
		}
		payload, err := vitalsPayload(before, adjusted)
		if err != nil {
			return err
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID:  session.ID,
			Seq:            seq,
			Kind:           eventCharacterVitalsAdjusted,
			ActorUserID:    &m.UserID,
			CharacterID:    &charText,
			Payload:        payload,
			IdempotencyKey: &keyText,
			CreatedAt:      s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		// A combat in progress with this character shows its state ("Caído", the
		// healing) from the vitals: its revision goes up in the same transaction, so
		// every screen reads it again.
		if enc, err := q.GetOpenEncounter(ctx, session.ID); err == nil {
			cs, err := q.ListCombatants(ctx, enc.ID)
			if err != nil {
				return fmt.Errorf("list the combatants: %w", err)
			}
			if slices.ContainsFunc(cs, func(c playdb.Combatant) bool { return c.CharacterID == charText }) {
				t, err := q.TouchEncounter(ctx, enc.ID)
				if err != nil {
					return fmt.Errorf("touch the encounter: %w", err)
				}
				touched = &t
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find the open encounter: %w", err)
		}
		after = adjusted
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "adjust a character's vitals", err)
	}
	if repeated {
		// Nothing changed and nothing goes out: answer with the vitals as
		// they are now.
		current, err := s.vitals.GetVitals(ctx, m.CampaignID, charText)
		if err != nil {
			return nil, s.dbError(ctx, "get vitals", err)
		}
		return connect.NewResponse(&playv1.AdjustCharacterVitalsResponse{Vitals: current}), nil
	}
	s.hub.Publish(m.CampaignID, live.Event{
		Audience: vitalsAudience(after),
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_VitalsChanged_{
			VitalsChanged: &playv1.WatchGameSessionResponse_VitalsChanged{Vitals: after},
		}},
	})
	if touched != nil {
		s.publishEncounterChanged(ctx, m.CampaignID, *touched)
	}
	return connect.NewResponse(&playv1.AdjustCharacterVitalsResponse{Vitals: after}), nil
}

// vitalsNumbers are the numbers a session event keeps about a character's
// vitals: no names, no free text (docs/privacidade.md).
type vitalsNumbers struct {
	HitPointsCurrent   int32   `json:"hit_points_current"`
	HitPointsTemporary int32   `json:"hit_points_temporary"`
	SpellSlotsUsed     []int32 `json:"spell_slots_used,omitempty"` // index 0 is spell level 1
	PactSlotsUsed      int32   `json:"pact_slots_used,omitempty"`
	HitDiceUsed        int32   `json:"hit_dice_used"`
	// ResourcesUsed are the uses spent of the class and race resources, by
	// resource key (Etapa 6).
	ResourcesUsed map[string]int32 `json:"resources_used,omitempty"`
}

func numbersOf(v *playv1.CharacterVitals) vitalsNumbers {
	n := vitalsNumbers{
		HitPointsCurrent:   v.GetHitPointsCurrent(),
		HitPointsTemporary: v.GetHitPointsTemporary(),
		PactSlotsUsed:      v.GetPactSlots().GetUsed(),
		HitDiceUsed:        v.GetHitDiceUsed(),
	}
	for _, r := range v.GetResources() {
		if r.GetUsed() > 0 {
			if n.ResourcesUsed == nil {
				n.ResourcesUsed = map[string]int32{}
			}
			n.ResourcesUsed[r.GetKey()] = r.GetUsed()
		}
	}
	for _, slot := range v.GetSpellSlots() {
		level := int(slot.GetLevel())
		if level < 1 || level > 9 {
			continue // never: the characters module sends levels 1 to 9
		}
		for len(n.SpellSlotsUsed) < level {
			n.SpellSlotsUsed = append(n.SpellSlotsUsed, 0)
		}
		n.SpellSlotsUsed[level-1] = slot.GetUsed()
	}
	return n
}

// vitalsPayload is a character_vitals_adjusted event's payload: the numbers
// before and after, so the history can tell what the master changed and,
// later, undo it.
func vitalsPayload(before, after *playv1.CharacterVitals) ([]byte, error) {
	b, err := json.Marshal(struct {
		Before vitalsNumbers `json:"before"`
		After  vitalsNumbers `json:"after"`
	}{numbersOf(before), numbersOf(after)})
	if err != nil {
		return nil, fmt.Errorf("encode the event payload: %w", err)
	}
	return b, nil
}
