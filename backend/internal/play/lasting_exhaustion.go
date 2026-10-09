package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Exhaustion (SRD 5.1, Conditions): six levels the master sets, each adding to the ones
// below it (1 disadvantage on ability checks; 2 speed halved; 3 disadvantage on attack rolls
// and saving throws; 4 hit point maximum halved; 5 speed 0; 6 death). A character keeps its
// level in its vitals, an NPC in the combat; level 4 halves the maximum and cuts the current
// hit points to it, taking the level off gives the maximum back and never heals, and level 6
// takes a character to the death confirmation (ConfirmDeath) as the third failed death save
// does. A long rest with food and drink takes one level off (SRD, Long Rest); the app does not
// count food, so the master takes it off here, and the rest offers it.

// exhaustionTarget is who the master changed the exhaustion of.
type exhaustionTarget struct {
	characterID string
	combatant   *playdb.Combatant
}

// SetExhaustion implements playv1connect.LastingEffectServiceHandler.
func (s *Service) SetExhaustion(
	ctx context.Context,
	req *connect.Request[playv1.SetExhaustionRequest],
) (*connect.Response[playv1.SetExhaustionResponse], error) {
	m, key, subject, err := s.exhaustionCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId(), req.Msg.GetCharacterId(), req.Msg.GetCombatantId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetLevel() < 0 || req.Msg.GetLevel() > combat.MaxExhaustion {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("level must be 0 to 6"))
	}
	level, hpMax, out, err := s.changeExhaustion(ctx, m, key, idem.Hash(req.Msg), subject, req.Msg.GetLevel(), req.Msg.GetExpectedLevel(), req.Msg.GetConfirmDeath(), eventExhaustion)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SetExhaustionResponse{Level: level, HitPointsMax: hpMax, Encounter: out}), nil
}

// LowerExhaustion implements playv1connect.LastingEffectServiceHandler.
func (s *Service) LowerExhaustion(
	ctx context.Context,
	req *connect.Request[playv1.LowerExhaustionRequest],
) (*connect.Response[playv1.LowerExhaustionResponse], error) {
	m, key, subject, err := s.exhaustionCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId(), req.Msg.GetCharacterId(), req.Msg.GetCombatantId())
	if err != nil {
		return nil, err
	}
	by := req.Msg.GetBy()
	if by < 1 || by > combat.MaxExhaustion {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("by must be 1 to 6"))
	}
	if req.Msg.GetReason() == playv1.ExhaustionLowerReason_EXHAUSTION_LOWER_REASON_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is a long rest or the master"))
	}
	// A level lowered by a count: the target level is worked out inside, from the level now.
	level, hpMax, out, err := s.changeExhaustion(ctx, m, key, idem.Hash(req.Msg), subject, -by, req.Msg.GetExpectedLevel(), false, eventExhaustion)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.LowerExhaustionResponse{Level: level, HitPointsMax: hpMax, Encounter: out}), nil
}

// exhaustionCall checks who calls and names the subject: a character, or a combatant of a combat.
func (s *Service) exhaustionCall(ctx context.Context, campaignID, rawKey, rawEnc, rawChar, rawComb string) (authz.Membership, string, exhaustionTarget, error) {
	m, err := authz.RequireCampaignRole(ctx, campaignID, authz.RoleMaster)
	if err != nil {
		return m, "", exhaustionTarget{}, err
	}
	key, err := parseKey(rawKey)
	if err != nil {
		return m, "", exhaustionTarget{}, err
	}
	switch {
	case rawChar != "":
		id, ok := parseID(rawChar)
		if !ok {
			return m, key, exhaustionTarget{}, errCharacterNotFound()
		}
		return m, key, exhaustionTarget{characterID: id}, nil
	case rawComb != "":
		if _, err := parseCombatID(rawEnc, "encounter"); err != nil {
			return m, key, exhaustionTarget{}, err
		}
		id, err := parseCombatID(rawComb, "combatant")
		if err != nil {
			return m, key, exhaustionTarget{}, err
		}
		return m, key, exhaustionTarget{combatant: &playdb.Combatant{ID: id, EncounterID: rawEnc}}, nil
	}
	return m, key, exhaustionTarget{}, connect.NewError(connect.CodeInvalidArgument, errors.New("set character_id or combatant_id"))
}

// changeExhaustion sets the level (a negative number takes that many levels off the one it is
// now) of a character or a combatant in one transaction: the vitals or the combatant's row, the
// state the effects leave, the death confirmation of level 6 and the line of the log.
func (s *Service) changeExhaustion(ctx context.Context, m authz.Membership, key string, hash *string, subject exhaustionTarget, level, expected int32, confirmDeath bool, kind string) (int32, int32, *playv1.Encounter, error) {
	var final, hpMax int32
	var vitals *playv1.CharacterVitals
	var secret bool
	encID := ""
	if subject.combatant != nil {
		encID = subject.combatant.EncounterID
		if id, err := parseCombatID(encID, "encounter"); err == nil {
			encID = id
		}
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: hash, kind: kind, encounterID: encID}, func(c *combatTx) (any, error) {
		vitals = nil
		var cs []playdb.Combatant
		var who *playdb.Combatant
		if c.enc.ID == "" { // a character, and the combat of the session if it runs
			enc, err := c.q.GetLatestEncounter(ctx, c.session.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("find the encounter: %w", err)
			}
			if err == nil && enc.Status != statusEnded {
				c.enc = enc
			}
		}
		if c.enc.ID != "" {
			var err error
			if cs, err = c.q.ListCombatants(ctx, c.enc.ID); err != nil {
				return nil, fmt.Errorf("list the combatants: %w", err)
			}
		}
		switch {
		case subject.combatant != nil:
			cb, err := findCombatant(cs, subject.combatant.ID, viewerOf(m))
			if err != nil {
				return nil, err
			}
			who = &cb
		default:
			if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.Kind == kindPlayer && o.CharacterID == subject.characterID }); i >= 0 {
				who = &cs[i]
			}
		}
		// The character's level is its vitals'; an NPC's is its combatant's.
		var before int32
		if who != nil && who.Kind != kindPlayer {
			before = who.ExhaustionLevel
		} else {
			charID := subject.characterID
			if who != nil {
				charID = who.CharacterID
			}
			v, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, charID)
			if err != nil {
				return nil, err
			}
			before = v.GetExhaustionLevel()
		}
		if expected != before {
			return nil, connect.NewError(connect.CodeAborted, errors.New("the level of exhaustion is not the one the screen shows"))
		}
		target := level
		if level < 0 {
			target = max(before+level, 0)
		}
		if target == combat.MaxExhaustion && !confirmDeath && target != before {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("level 6 is death: confirm_death must be true"))
		}
		ev := actionEvent{Round: c.enc.Round, Secret: who != nil && who.Hidden}
		if who != nil {
			ev.Actor = who.ID
			c.characterID = &who.CharacterID
		}
		if who != nil && who.Kind != kindPlayer {
			hp, err := s.setNPCExhaustion(ctx, c, *who, target)
			if err != nil {
				return nil, err
			}
			hpMax = hp
		} else {
			charID := subject.characterID
			if who != nil {
				charID = who.CharacterID
			}
			_, after, err := s.vitals.SetExhaustion(ctx, c.tx, c.session.CampaignID, charID, target)
			if err != nil {
				return nil, err
			}
			vitals, hpMax = after, after.GetHitPointsMax()
			if charID != "" && who == nil {
				c.characterID = &charID
			}
			if who != nil && target == combat.MaxExhaustion {
				// Level 6 is death: the character goes to the confirmation as the third failed death
				// save does; only the master's ConfirmDeath says it died (RN-03).
				if err := c.q.SetCombatantDeathSaves(ctx, playdb.SetCombatantDeathSavesParams{ID: who.ID, DeathSuccesses: 0, DeathFailures: 3, DeathSaveRolled: who.DeathSaveRolled, Defeated: who.Defeated}); err != nil {
					return nil, fmt.Errorf("take the character to the death confirmation: %w", err)
				}
			}
		}
		final = target
		if who != nil {
			if err := c.q.SetCombatantExhaustionLevel(ctx, playdb.SetCombatantExhaustionLevelParams{ID: who.ID, ExhaustionLevel: target}); err != nil {
				return nil, fmt.Errorf("copy the exhaustion: %w", err)
			}
			if err := s.refreshCombatants(ctx, c, who.ID); err != nil {
				return nil, err
			}
		}
		if c.enc.ID != "" {
			var err error
			if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
				return nil, fmt.Errorf("touch the encounter: %w", err)
			}
		}
		// The exhaustion is the owner's and the master's: no line for the other players (RN-10).
		ev.Secret = true
		secret = true
		ev.Lasting = &lastingEvent{Key: "condition:exhaustion", Change: "exhaustion", Level: target, Before: before}
		if who != nil {
			ev.Lasting.Targets = []string{who.ID}
		}
		return ev, nil
	})
	if err != nil {
		return 0, 0, nil, s.dbError(ctx, "set the exhaustion", err)
	}
	var enc *playv1.Encounter
	if res.encounterID != "" {
		enc, err = s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
			s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
			s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !secret)
		})
		if err != nil {
			return 0, 0, nil, err
		}
	}
	s.publishVitals(m.CampaignID, vitals)
	return final, hpMax, enc, nil
}

// setNPCExhaustion puts a level of exhaustion on an NPC: from level 4 its maximum is halved and its
// hit points cut to it, and the maximum it had is kept to give it back; level 6 defeats it. It returns
// the maximum it has now.
func (s *Service) setNPCExhaustion(ctx context.Context, c *combatTx, who playdb.Combatant, level int32) (int32, error) {
	base := num(who.HpMax)
	if who.HpMaxBase != nil {
		base = *who.HpMaxBase
	}
	newMax := clamp32(combat.ExhaustedMaxHP(int(base), int(level)), 0, 1<<30)
	current := num(who.HpCurrent)
	var baseKept *int32
	if level >= 4 {
		baseKept = &base
		current = min(current, newMax)
	}
	pct := int32(100)
	switch ex := combat.ExhaustionAt(int(level)); {
	case ex.SpeedZero:
		pct = 0
	case ex.SpeedHalved:
		pct = 50
	}
	defeated := who.Defeated || level >= combat.MaxExhaustion
	if level >= combat.MaxExhaustion {
		current = 0
	}
	var hpMax, hpCur *int32
	if who.HpMax != nil {
		hpMax, hpCur = &newMax, &current
	}
	if err := c.q.SetCombatantExhaustion(ctx, playdb.SetCombatantExhaustionParams{ID: who.ID, ExhaustionLevel: level, HpMax: hpMax, HpCurrent: hpCur, HpMaxBase: baseKept, EffectSpeedPct: pct, Defeated: defeated}); err != nil {
		return 0, fmt.Errorf("set the exhaustion: %w", err)
	}
	if defeated && !who.Defeated && c.enc.Status == statusActive {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return 0, fmt.Errorf("list the combatants: %w", err)
		}
		if _, err := leaveTurn(ctx, c, cs, who); err != nil {
			return 0, err
		}
	}
	return newMax, nil
}
