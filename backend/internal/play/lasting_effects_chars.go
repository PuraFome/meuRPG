package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The effects on the characters outside a running combat: how they are read, and how the
// master ends them (RN-22).

// ListCharacterEffects implements playv1connect.LastingEffectServiceHandler.
func (s *Service) ListCharacterEffects(
	ctx context.Context,
	req *connect.Request[playv1.ListCharacterEffectsRequest],
) (*connect.Response[playv1.ListCharacterEffectsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListCharacterEffectsOfCampaign(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the characters' effects", err)
	}
	master := m.Role == authz.RoleMaster
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if !slices.Contains(ids, r.CharacterID) {
			ids = append(ids, r.CharacterID)
		}
	}
	chars := map[string]string{} // id -> name
	owned := map[string]bool{}
	if len(ids) > 0 {
		found, err := s.roster.CombatCharacters(ctx, nil, m.CampaignID, ids)
		if err != nil {
			return nil, s.dbError(ctx, "read the characters", err)
		}
		for _, c := range found {
			chars[c.ID] = c.Name
			owned[c.ID] = c.Player && c.PlayerUserID == m.UserID
		}
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules", err)
	}
	names := s.namesFor(ctx, m.CampaignID)
	out := &playv1.ListCharacterEffectsResponse{}
	for _, r := range rows {
		name, ok := chars[r.CharacterID]
		if !ok {
			continue // reserved or gone: nobody reads it
		}
		// A player reads what the master leaves visible, on their own characters, and the
		// effects for everyone on the others (RN-10).
		if !master && (!r.PlayerVisible || (!owned[r.CharacterID] && r.Audience != "all")) {
			continue
		}
		e := &playv1.CharacterEffect{
			Id: r.ID, CharacterId: r.CharacterID, CharacterName: name, SourceKey: r.SourceKey, SourceNamePt: names(r.SourceKey),
			Concentration: r.Concentration, DurationTextPt: gameTimeText(r.SecondsLeft, r.Concentration),
		}
		if r.SecondsLeft != nil {
			e.SecondsLeft = r.SecondsLeft
		}
		for _, k := range r.ConditionKeys {
			e.ConditionNamesPt = append(e.ConditionNamesPt, names(k))
		}
		mods := modifiersOf(r.Modifiers)
		if d, ok := content.CombatEffect(r.SourceKey); ok {
			e.TagsPt = d.TagsPT
		} else if d, ok := content.CombatSpellEffect(r.SourceKey); ok {
			e.TagsPt = d.TagsPT
		}
		e.TagsPt = append(append([]string{}, e.TagsPt...), modifierTagsPT(mods)...)
		e.TagsPt = append(e.TagsPt, narratedNotesPT(mods)...)
		if master {
			e.PlayerVisible, e.Audience = r.PlayerVisible, audienceProto(r.Audience)
		}
		out.Effects = append(out.Effects, e)
	}
	return connect.NewResponse(out), nil
}

// gameTimeText writes a game time the way the card reads it outside a combat: "dura 1 minuto",
// "dura 54 segundos", "dura 8 horas" (a round is 6 seconds; the app has no wall clock).
func gameTimeText(seconds *int32, concentration bool) string {
	if seconds == nil {
		if concentration {
			return "Dura enquanto quem a conjurou se concentrar"
		}
		return "Dura até o mestre encerrar"
	}
	left := int(*seconds)
	var parts []string
	unit := func(n int, one, many string) {
		if n == 0 {
			return
		}
		if n == 1 {
			parts = append(parts, "1 "+one)
			return
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, many))
	}
	unit(left/3600, "hora", "horas")
	unit(left%3600/60, "minuto", "minutos")
	unit(left%60, "segundo", "segundos")
	if len(parts) == 0 {
		return "Dura menos de um segundo"
	}
	return "Dura " + strings.Join(parts, " e ")
}

// endCharacterEffect is EndLastingEffect for an effect on a character outside a combat: the
// effect goes, or the whole casting does with the cast that held it.
func (s *Service) endCharacterEffect( //nolint:gocognit // the steps of one transaction in one closure, like the other writes of the master
	ctx context.Context, m authz.Membership, msg *playv1.EndLastingEffectRequest, key string,
) (*connect.Response[playv1.EndLastingEffectResponse], error) {
	id, ok := parseID(msg.GetEffectId())
	if !ok {
		return nil, errEffectNotFound()
	}
	hash := idem.Hash(msg)
	var ended int32
	var told []*playv1.CharacterVitals
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ended, told = 0, nil
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, key, eventLastingEnded, hash)
		if err != nil {
			return err
		}
		if again {
			var ev gameTimeEvent
			_ = json.Unmarshal(done.Payload, &ev)
			ended = ev.Ended
			return nil
		}
		row, err := q.GetCharacterEffect(ctx, playdb.GetCharacterEffectParams{ID: id, CampaignID: m.CampaignID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errEffectNotFound()
		}
		if err != nil {
			return fmt.Errorf("find the effect: %w", err)
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), kind: eventLastingEnded, actorUserID: m.UserID, svc: s, master: true})
		if err != nil {
			return err
		}
		if msg.GetScope() == playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP {
			gone, err := q.DeleteCharacterEffectsOfGroup(ctx, playdb.DeleteCharacterEffectsOfGroupParams{CampaignID: m.CampaignID, GroupID: row.GroupID})
			if err != nil {
				return fmt.Errorf("end the casting: %w", err)
			}
			ended = int32(len(gone)) //nolint:gosec // a handful of effects
			if err := s.afterCharacterEffectsGone(ctx, c, gone); err != nil {
				return err
			}
			var touched []string
			for _, g := range gone {
				touched = append(touched, g.CharacterID)
			}
			if err := s.syncArmorBase(ctx, c, touched...); err != nil {
				return err
			}
			if cast, err := q.GetSpellCast(ctx, playdb.GetSpellCastParams{ID: row.GroupID, CampaignID: m.CampaignID}); err == nil && cast.Status == castActive {
				if _, _, _, err := s.closeCast(ctx, c, cast, castEnded, endDismissed); err != nil {
					return err
				}
			}
		} else {
			if err := q.DeleteCharacterEffect(ctx, row.ID); err != nil {
				return fmt.Errorf("end the effect: %w", err)
			}
			ended = 1
			if err := s.afterCharacterEffectsGone(ctx, c, []playdb.CharacterEffect{row}); err != nil {
				return err
			}
			if err := s.syncArmorBase(ctx, c, row.CharacterID); err != nil {
				return err
			}
		}
		told = c.told
		payload, err := json.Marshal(gameTimeEvent{Ended: ended})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventLastingEnded, ActorUserID: &m.UserID,
			Payload: payload, IdempotencyKey: &key, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "end an effect", err)
	}
	s.publishCastsChanged(m.CampaignID, false)
	s.publishVitalsOf(m.CampaignID, told)
	return connect.NewResponse(&playv1.EndLastingEffectResponse{Ended: ended}), nil
}
