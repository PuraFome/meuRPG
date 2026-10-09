package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// RemoveDamagePart implements playv1connect.CombatServiceHandler: the master takes an
// extra out of a rolled damage, with a reason (PM-06b, decision 17). Sneak Attack
// without an ally near the target in a combat with no map is the usual case: the
// app cannot know, and the master can. The total drops by that part; an NPC that took
// the damage at once gets the difference back, up to its maximum. It cannot be undone:
// the damage would have to be rolled again.
func (s *Service) RemoveDamagePart(
	ctx context.Context,
	req *connect.Request[playv1.RemoveDamagePartRequest],
) (*connect.Response[playv1.RemoveDamagePartResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	pendingID, err := parseCombatID(req.Msg.GetPendingDamageId(), "pending damage")
	if err != nil {
		return nil, err
	}
	partKey := req.Msg.GetPartKey()
	if partKey == "" || len(partKey) > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("part_key must name a part of the damage"))
	}
	reason, err := cleanReason(req.Msg.GetReason())
	if err != nil {
		return nil, err
	}

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventDamagePartRemoved, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		p, err := c.q.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: c.enc.ID, ID: pendingID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("pending damage not found"))
		}
		if err != nil {
			return nil, fmt.Errorf("find the pending damage: %w", err)
		}
		if p.Status != pendingRolled && p.Status != pendingApplied {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_NOT_ROLLED, "the damage was not rolled")
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		attacker, _ := findCombatant(cs, deref(p.AttackerID), combatViewer{master: true})
		target, _ := findCombatant(cs, p.TargetID, combatViewer{master: true})
		// The master gave a player's character the damage already: it went through the
		// vitals, and he corrects them by hand.
		if p.Status == pendingApplied && !holdsHP(target) {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED, "the damage was applied to a character already")
		}
		parts, rolls := readParts(p.Parts), readPartRolls(p.PartRolls)
		if !takeOut(parts, rolls, partKey) {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE, "that part cannot be taken out")
		}

		byType, total := countedByType(rolls)
		// The target's modifiers are the ones the roll used: the stored groups say them.
		var mods []combat.Modifier
		for _, g := range readStepGroups(p.Steps) {
			mods = append(mods, g.Mods...)
		}
		land := settleSteps(byType, mods, nil)
		amount := clamp32(total, 0, math.MaxInt32)
		var after *int32
		if land.changed {
			after = &land.amount
		}
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Pending: p.ID, Key: partKey,
			DamageType: p.DamageType, Rolled: num(p.Amount), Total: amount,
		}
		landed := amount
		if land.changed {
			landed = land.amount
		}
		made.Amount = landed
		if holdsHP(target) && p.Status == pendingApplied {
			now, err := giveBackHitPoints(ctx, c, p, target, landed)
			if err != nil {
				return nil, err
			}
			made.Before, made.After = new(hpOf(target)), &now
			amount = landed
		}
		if err := c.q.SetPendingDamageAmount(ctx, playdb.SetPendingDamageAmountParams{
			ID: p.ID, Amount: &amount, RollTotal: new(clamp32(total, 0, math.MaxInt32)), Parts: mustJSON(parts), PartRolls: mustJSON(rolls), Steps: mustJSON(land.groups), AfterSteps: after,
		}); err != nil {
			return nil, fmt.Errorf("keep the damage without the extra: %w", err)
		}
		row, err := c.q.InsertCombatReason(ctx, playdb.InsertCombatReasonParams{EncounterID: c.enc.ID, Kind: "part_removed", Reason: reason, CreatedAt: c.now})
		if err != nil {
			return nil, fmt.Errorf("keep the reason: %w", err)
		}
		made.ReasonID, made.Amount = row.ID, amount
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "take a part out of a damage", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the removal", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(ctx, res, ev.Pending, combatViewer{master: true})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RemoveDamagePartResponse{Encounter: out, PendingDamage: pending}), nil
}

// countedByType sums the dice and numbers of the parts that count, by damage type, and
// all of them together.
func countedByType(rolls []partRoll) (map[string]int, int) {
	byType := map[string]int{}
	total := 0
	for _, r := range rolls {
		if r.Counted {
			n := max(int(r.Sum)+int(r.Flat)+int(r.Fixed), 0)
			byType[r.DamageType] += n
			total += n
		}
	}
	return byType, total
}

// giveBackHitPoints gives an NPC the hit points a removed extra took: its hit points
// before the damage, with what is left of the damage taken.
func giveBackHitPoints(ctx context.Context, c *combatTx, p playdb.PendingDamage, target playdb.Combatant, landed int32) (hpState, error) {
	var before hpState
	if err := jsonUnmarshal(p.LandedBefore, &before); err != nil {
		return hpState{}, fmt.Errorf("read the hit points before the damage: %w", err)
	}
	dmg := combat.ApplyDamage(int(before.HP), int(before.Temp), int(landed))
	now := hpState{HP: clamp32(dmg.HP, 0, math.MaxInt32), Temp: clamp32(dmg.TempHP, 0, math.MaxInt32), Defeated: dmg.HP == 0}
	if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &now.HP, HpTemp: &now.Temp, Defeated: now.Defeated}); err != nil {
		return hpState{}, fmt.Errorf("give the hit points back: %w", err)
	}
	return now, nil
}

// takeOut marks an extra as removed, and says whether it could be. Only an extra that
// counts comes out; the weapon, an automatic line and a smite (whose slot is spent) stay.
func takeOut(parts []partRecord, rolls []partRoll, key string) bool {
	pi := slices.IndexFunc(parts, func(x partRecord) bool { return x.Key == key })
	ri := slices.IndexFunc(rolls, func(x partRoll) bool { return x.Key == key })
	if pi < 0 || ri < 0 || parts[pi].Kind != partExtra || key == combat.ExtraDivineSmite || key == combat.ExtraDivineSmiteExtra || !rolls[ri].Counted {
		return false
	}
	parts[pi].Removed, rolls[ri].Counted = true, false
	return true
}
