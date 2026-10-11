package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// charEffectEvent is the payload of the master's giving an effect to characters outside a
// combat: keys and ids, never a name.
type charEffectEvent struct {
	Key     string   `json:"key"`
	Targets []string `json:"targets"`
	Effects []string `json:"effects"`
}

// AddCharacterEffect implements playv1connect.LastingEffectServiceHandler.
func (s *Service) AddCharacterEffect( //nolint:gocognit,gocyclo // the steps of one transaction in one closure, like the other writes of the master
	ctx context.Context,
	req *connect.Request[playv1.AddCharacterEffectRequest],
) (*connect.Response[playv1.AddCharacterEffectResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	bad := func(text string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(text)) }
	if n := len(req.Msg.GetCharacterIds()); n < 1 || n > maxEffectTargets {
		return nil, bad("character_ids must have 1 to 10 characters")
	}
	var characterIDs []string
	for _, raw := range req.Msg.GetCharacterIds() {
		id, ok := parseID(raw)
		if !ok {
			return nil, bad("character_ids must be UUIDs")
		}
		if slices.Contains(characterIDs, id) {
			return nil, bad("character_ids repeats a character")
		}
		characterIDs = append(characterIDs, id)
	}
	if utf8.RuneCountInString(req.Msg.GetPlayerLabel()) > maxEffectLabel {
		return nil, bad("player_label must have at most 30 characters")
	}
	kind := durationKindOf(req.Msg.GetDurationKind())
	seconds := req.Msg.GetSeconds()
	switch kind {
	case rules.EffectDurationRounds:
		if seconds < 1 || seconds > maxGameTimeStep {
			return nil, bad("seconds must be 1 to 86400")
		}
	case "", rules.EffectDurationUntilDismissed, rules.EffectDurationLongRest:
		seconds = 0
	case rules.EffectDurationConcentration:
		kind, seconds = rules.EffectDurationUntilDismissed, 0 // nobody concentrates on it: it lasts until the master ends it
	default:
		return nil, bad("duration_kind has no meaning outside a combat")
	}
	catalogKey := req.Msg.GetCatalogKey()
	keyText, hash := key, idem.Hash(req.Msg)
	var made charEffectEvent
	var vitals []*playv1.CharacterVitals
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		made, vitals = charEffectEvent{}, nil
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, keyText, eventLastingAdded, hash)
		if err != nil {
			return err
		}
		if again {
			return json.Unmarshal(done.Payload, &made)
		}
		found, err := s.roster.CombatCharacters(ctx, tx, m.CampaignID, characterIDs)
		if err != nil {
			return err
		}
		for _, id := range characterIDs {
			if !slices.ContainsFunc(found, func(ch link.Character) bool { return ch.ID == id && ch.Player && !ch.Reserved }) {
				return connect.NewError(connect.CodeNotFound, errors.New("character not found"))
			}
		}
		// A character in an open combat holds its effects there.
		if enc, err := q.GetOpenEncounter(ctx, session.ID); err == nil {
			cs, err := q.ListCombatants(ctx, enc.ID)
			if err != nil {
				return fmt.Errorf("list the combatants: %w", err)
			}
			if slices.ContainsFunc(cs, func(o playdb.Combatant) bool {
				return slices.Contains(characterIDs, o.CharacterID) && backedByCharacter(o)
			}) {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("the character is in a combat: add the effect there"))
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find the open encounter: %w", err)
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), kind: eventLastingAdded, actorUserID: m.UserID, svc: s, master: true})
		if err != nil {
			return err
		}
		content, err := s.contentOf(ctx, c)
		if err != nil {
			return err
		}
		var (
			def        *rules.EffectDef
			conditions []string
			sourceKind = "master"
			audience   = rules.EffectVisibilityPublic
			fallback   = rules.EffectDurationUntilDismissed
			fallbackS  *int32
		)
		switch {
		case strings.HasPrefix(catalogKey, "condition:"):
			info, ok := content.ConditionInfo(catalogKey)
			if !ok || catalogKey == conditionExhaustion {
				return bad("catalog_key is not an effect the master may add")
			}
			conditions, audience = []string{catalogKey}, info.Visibility
		case strings.HasPrefix(catalogKey, "spell:"):
			d, ok := content.CombatSpellEffect(catalogKey)
			if !ok {
				return bad("catalog_key is not an effect the master may add")
			}
			def, sourceKind = d, "spell"
			if d.Visibility != "" {
				audience = d.Visibility
			}
			if secs, timed := content.SpellEffectSeconds(catalogKey); timed && secs > 0 {
				fallback, fallbackS = rules.EffectDurationRounds, new(clamp32(secs, 1, maxGameTimeStep))
			}
		default:
			d, ok := content.CombatEffect(catalogKey)
			if !ok || d.Kind != "master" {
				return bad("catalog_key is not an effect the master may add")
			}
			def = d
			if d.Visibility != "" {
				audience = d.Visibility
			}
			if d.Duration != nil && d.Duration.Kind == rules.EffectDurationRounds {
				fallback, fallbackS = rules.EffectDurationRounds, new(clamp32(d.Duration.Rounds*rules.SecondsPerRound, 1, maxGameTimeStep))
			}
		}
		var secondsLeft *int32
		switch kind {
		case "":
			kind, secondsLeft = fallback, fallbackS
		case rules.EffectDurationRounds:
			secondsLeft = &seconds
		}
		if kind == rules.EffectDurationRounds && secondsLeft == nil {
			return bad("seconds is required for a timed effect")
		}
		if a := req.Msg.GetAudience(); a != playv1.EffectAudience_EFFECT_AUDIENCE_UNSPECIFIED {
			audience = map[playv1.EffectAudience]string{playv1.EffectAudience_EFFECT_AUDIENCE_ALL: rules.EffectVisibilityPublic, playv1.EffectAudience_EFFECT_AUDIENCE_OWNER: rules.EffectVisibilityOwner}[a]
		}
		dbAudience := "all"
		if audience == rules.EffectVisibilityOwner {
			dbAudience = audienceOwner
		}
		visible := true
		if v := req.Msg.PlayerVisible; v != nil {
			visible = *v
		}
		var label *string
		if l := req.Msg.GetPlayerLabel(); l != "" {
			label = &l
		}
		ability := ""
		if def != nil {
			if ability, err = s.abilityChoiceOf(ctx, c, catalogKey, req.Msg.GetAbilityKey()); err != nil {
				return err
			}
		}
		group := uuid.New().String()
		var touched []string
		for _, id := range characterIDs {
			mods := []rules.EffectModifier{}
			if def != nil {
				var ok bool
				opts, err := s.castOptsOf(ctx, c, def, ability, "")
				if err != nil {
					return err
				}
				if mods, ok, err = s.modifiersFor(ctx, c, def, id, opts); err != nil {
					return err
				} else if !ok {
					continue // Armadura Arcana on a creature that wears armor takes no hold
				}
			}
			body, err := json.Marshal(mods)
			if err != nil {
				return fmt.Errorf("encode the modifiers: %w", err)
			}
			if sourceKind == "spell" {
				if _, err := q.DeleteCharacterEffectsOfSpell(ctx, playdb.DeleteCharacterEffectsOfSpellParams{CharacterID: id, SourceKey: catalogKey, GroupID: group}); err != nil {
					return fmt.Errorf("replace the same spell: %w", err)
				}
			}
			p := playdb.InsertCharacterEffectParams{
				CampaignID: m.CampaignID, CharacterID: id, GroupID: group, SourceKey: catalogKey, SourceKind: sourceKind,
				ConditionKeys: append([]string{}, conditions...), Modifiers: body, DurationKind: kind, SecondsLeft: secondsLeft,
				PlayerVisible: visible, Audience: dbAudience, PlayerLabel: label, CreatedAt: c.now,
			}
			if def != nil {
				p.ConditionKeys = append([]string{}, def.Conditions...)
				if def.EndSave != nil {
					p.EndSaveAbility = &def.EndSave.Ability
				}
				if def.StartSave != nil {
					p.StartSaveAbility = &def.StartSave.Ability
					if def.StartSave.OnFail != "" {
						p.OnFailEffect = &def.StartSave.OnFail
					}
				}
				if t := def.Trigger; t != nil {
					p.TriggerDice, p.TriggerDamageType, p.TriggerMaxTriggers = &t.Dice, &t.DamageType, new(int32(t.MaxTriggers)) //nolint:gosec // a small number of the file
				}
			}
			row, err := q.InsertCharacterEffect(ctx, p)
			if err != nil {
				return fmt.Errorf("put the effect on the character: %w", err)
			}
			made.Targets, made.Effects = append(made.Targets, id), append(made.Effects, row.ID)
			touched = append(touched, id)
		}
		if len(made.Effects) == 0 {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the effect takes no hold of these characters"))
		}
		made.Key = catalogKey
		if err := s.syncArmorBase(ctx, c, touched...); err != nil {
			return err
		}
		vitals = c.told
		payload, err := json.Marshal(made)
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventLastingAdded, ActorUserID: &m.UserID,
			Payload: payload, IdempotencyKey: &keyText, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "give an effect to characters", err)
	}
	s.publishCastsChanged(m.CampaignID, false)
	s.publishVitalsOf(m.CampaignID, vitals)
	return connect.NewResponse(&playv1.AddCharacterEffectResponse{EffectIds: made.Effects}), nil
}
