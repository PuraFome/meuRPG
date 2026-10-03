package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What a turn can do (MR-012, MR-014, Etapa 6, slice 6.4a): the options of a
// combatant, the attack in two steps (the roll to hit, then the damage), the
// standard actions that only spend the economy, and the master's hand on an
// NPC's hit points. The helpers are in combat_write.go, the visibility in
// combat_view.go; the undo is in combat_undo.go and the log in
// combat_log.go.
//
// An attack is two writes because a physical d20 and its damage dice are
// typed one after the other (RN-18). The hit opens a pending_damages row; an
// NPC target takes the damage at once, a player's character waits for the
// master (RN-02) in "rolled" until he applies or discards it.

// The most damage or healing the master types for an NPC, and the most
// temporary hit points (as for a character's vitals).
const (
	maxHitPointChange = 9999
	maxTempHitPoints  = 999
	// meleeReachFt is the reach of a melee attack that says none: 5 ft.
	meleeReachFt = 5
)

// damageTypePT names the damage types in Portuguese for the log; the keys are
// the content's ("damage-type:fire").
var damageTypePT = map[string]string{
	"damage-type:acid": "ácido", "damage-type:bludgeoning": "concussão", "damage-type:cold": "frio",
	"damage-type:fire": "fogo", "damage-type:force": "energia", "damage-type:lightning": "elétrico",
	"damage-type:necrotic": "necrótico", "damage-type:piercing": "perfurante", "damage-type:poison": "veneno",
	"damage-type:psychic": "psíquico", "damage-type:radiant": "radiante", "damage-type:slashing": "cortante",
	"damage-type:thunder": "trovejante",
}

// The standard actions that are not taken through TakeAction.
const (
	standardAttack = "standard:attack"
	standardCast   = "standard:cast-a-spell"
)

// gate says why the combatant cannot act now, or UNSPECIFIED when it can: the
// combat is not running, the combatant is out of the fight or not on turn, or
// a player's character is down. GetTurnOptions disables every option with it,
// and the writes refuse with the matching EncounterBlocked.
func (s *Service) gate(ctx context.Context, tx pgx.Tx, campaignID string, e playdb.Encounter, c playdb.Combatant) (rulesv1.DisabledReasonCode, error) {
	switch {
	case e.Status != statusActive:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE, nil
	case c.Defeated:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED, nil
	case e.CurrentCombatantID == nil || *e.CurrentCombatantID != c.ID:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN, nil
	}
	down, err := s.isDown(ctx, tx, campaignID, c)
	if err != nil {
		return 0, err
	}
	if down {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN, nil
	}
	return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED, nil
}

// isDown says whether a player's character is at 0 hit points ("Caído"): its
// vitals say so. An NPC is never down: it is defeated.
// A write passes its transaction, so it reads what it will overwrite; a read
// passes nil.
func (s *Service) isDown(ctx context.Context, tx pgx.Tx, campaignID string, c playdb.Combatant) (bool, error) {
	if c.Kind != kindPlayer {
		return false, nil
	}
	var v *playv1.CharacterVitals
	var err error
	if tx != nil {
		v, err = s.vitals.GetVitalsTx(ctx, tx, campaignID, c.CharacterID)
	} else {
		v, err = s.vitals.GetVitals(ctx, campaignID, c.CharacterID)
	}
	if err != nil {
		return false, err
	}
	return v.GetHitPointsMax() > 0 && v.GetHitPointsCurrent() == 0, nil
}

// gateError is the failed_precondition a write answers with for a reason of
// gate.
func gateError(code rulesv1.DisabledReasonCode) error {
	switch code {
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_COMBATANT_DOWN, "the character is down")
	}
	// Not on turn, or out of the fight (a defeated combatant has no turn).
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN, "it is not this combatant's turn")
}

// mustActNow checks, inside a write, that the combatant is the one that may
// act now (on turn, in a running combat, not down).
func (s *Service) mustActNow(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	code, err := s.gate(ctx, c.tx, c.session.CampaignID, c.enc, who)
	if err != nil {
		return err
	}
	if code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return gateError(code)
	}
	return nil
}

func turnOf(c playdb.Combatant) link.Turn {
	return link.Turn{
		ActionUsed: c.ActionUsed, BonusActionUsed: c.BonusActionUsed, ReactionUsed: c.ReactionUsed,
		Dashed: c.Dashed, SpeedFt: int(c.SpeedFt), MovementUsedFt: int(c.MovementUsedFt),
	}
}

// diceRoll builds the API's DiceRoll.
func diceRoll(count, sides int32, faces []int32, modifier, total int32, physical bool) *playv1.DiceRoll {
	return &playv1.DiceRoll{DiceCount: count, DiceSides: sides, Faces: faces, Modifier: modifier, Total: total, Physical: physical}
}

// faces32 converts the faces a roll gave.
func faces32(faces []int) []int32 {
	out := make([]int32, len(faces))
	for i, f := range faces {
		out[i] = clamp32(f, 0, 1000)
	}
	return out
}

// reachFt is how far an attack reaches: its range, its long range, or 5 ft
// for a melee attack that says none.
func reachFt(rangeFt, longRangeFt int32) int32 {
	return max(rangeFt, longRangeFt, meleeReachFt)
}

// distanceFt is the distance between two combatants on the grid, a king's
// move at 5 ft a square (RN-21); false when either has no square.
func distanceFt(a, b playdb.Combatant) (int32, bool) {
	if !placed(a) || !placed(b) {
		return 0, false
	}
	return clamp32(combat.GridDistanceFt(int(*a.GridCol), int(*a.GridRow), int(*b.GridCol), int(*b.GridRow)), 0, math.MaxInt32), true
}

// GetTurnOptions implements playv1connect.CombatServiceHandler.
func (s *Service) GetTurnOptions(
	ctx context.Context,
	req *connect.Request[playv1.GetTurnOptionsRequest],
) (*connect.Response[playv1.GetTurnOptionsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	enc, err := s.queries.GetEncounterInSession(ctx, playdb.GetEncounterInSessionParams{GameSessionID: session.ID, ID: encID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("encounter not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the encounter", err)
	}
	d, err := loadEncounter(ctx, s.queries, enc)
	if err != nil {
		return nil, s.dbError(ctx, "read the encounter", err)
	}
	v := viewerOf(m)
	who, err := findCombatant(d.cs, combID, v)
	if err != nil {
		return nil, err
	}
	if err := v.mayAct(who); err != nil {
		return nil, err
	}

	opts, err := s.roster.CombatTurnOptions(ctx, m.CampaignID, who.CharacterID, turnOf(who))
	if err != nil {
		return nil, s.dbError(ctx, "work out the turn options", err)
	}
	code, err := s.gate(ctx, nil, m.CampaignID, enc, who)
	if err != nil {
		return nil, s.dbError(ctx, "check the combatant's turn", err)
	}
	if code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		disableAll(opts, code)
	}
	res := &playv1.GetTurnOptionsResponse{Options: opts, YourTurn: enc.CurrentCombatantID != nil && *enc.CurrentCombatantID == who.ID}
	for _, a := range opts.GetAttacks() {
		if a.GetAttack().GetSaveDc() > 0 {
			continue // a saving throw, not an attack roll: the spells slice
		}
		res.AttackTargets = append(res.AttackTargets, &playv1.AttackTargets{
			AttackKey: a.GetAttack().GetKey(), Targets: targetsFor(d.cs, who, v, reachFt(a.GetAttack().GetRangeFt(), a.GetAttack().GetLongRangeFt())),
		})
	}
	open, err := s.queries.ListOpenPendingDamages(ctx, enc.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the pending damage", err)
	}
	for _, p := range open {
		if p.AttackerID == who.ID && pendingVisible(p, d.cs, v) {
			res.PendingDamages = append(res.PendingDamages, pendingProto(p, d.cs))
		}
	}
	return connect.NewResponse(res), nil
}

// disableAll disables every option with the reason: nothing can be done off
// turn, outside a running combat or by a combatant that is out.
func disableAll(o *rulesv1.TurnOptions, code rulesv1.DisabledReasonCode) {
	reason := func() *rulesv1.DisabledReason { return &rulesv1.DisabledReason{Code: code} }
	for _, a := range o.GetAttacks() {
		a.Enabled, a.Reason = false, reason()
	}
	for _, sp := range o.GetSpells() {
		sp.Enabled, sp.Reason = false, reason()
	}
	for _, a := range slices.Concat(o.GetStandardActions(), o.GetFeatureActions()) {
		a.Enabled, a.Reason = false, reason()
	}
}

// targetsFor lists who the combatant's attack of the given reach can target,
// as the viewer sees them: not itself, not a defeated one, not a hidden one
// for a player (RN-10), in turn order, each with the distance and whether it
// is too far.
func targetsFor(cs []playdb.Combatant, attacker playdb.Combatant, v combatViewer, reach int32) []*playv1.TargetInReach {
	var out []*playv1.TargetInReach
	for _, t := range cs {
		if t.ID == attacker.ID || t.Defeated || !v.sees(t) {
			continue
		}
		target := &playv1.TargetInReach{CombatantId: t.ID, Label: t.Label, State: stateOf(t)}
		if dist, ok := distanceFt(attacker, t); ok {
			target.DistanceFt = &dist
			target.TooFar = dist > reach
		} else {
			target.TooFar = !v.master // a player cannot attack what has no square (RollAttack)
		}
		out = append(out, target)
	}
	return out
}

// pendingVisible says whether the viewer may get a pending damage: a player
// never learns anything about a target they cannot see (RN-10, RN-20), for
// instance an NPC the master hid after the hit.
func pendingVisible(p playdb.PendingDamage, cs []playdb.Combatant, v combatViewer) bool {
	i := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == p.TargetID })
	return i >= 0 && v.sees(cs[i])
}

// pendingProto builds the PendingDamage that the master and the attacker's
// player get. cs are the combat's combatants, for the target's defeat.
func pendingProto(p playdb.PendingDamage, cs []playdb.Combatant) *playv1.PendingDamage {
	out := &playv1.PendingDamage{
		Id: p.ID, AttackerId: p.AttackerID, TargetId: p.TargetID, AttackKey: p.AttackKey,
		Status: pendingStatusToProto[p.Status], Critical: p.Critical,
		DiceCount: p.DiceCount, DiceSides: p.DiceSides, Bonus: p.DiceBonus,
		DamageTypeKey: p.DamageType, DamageTypePt: damageTypePT[p.DamageType],
	}
	if p.Amount != nil {
		out.Amount = *p.Amount
		out.Roll = diceRoll(p.DiceCount, p.DiceSides, p.Faces, p.DiceBonus, *p.Amount, p.Physical)
	}
	if p.Status == pendingApplied {
		out.TargetDefeated = slices.ContainsFunc(cs, func(c playdb.Combatant) bool { return c.ID == p.TargetID && c.Kind == kindNPC && c.Defeated })
	}
	return out
}

var pendingStatusToProto = map[string]playv1.PendingDamageStatus{
	pendingAwaitingRoll: playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL,
	pendingRolled:       playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED,
	pendingApplied:      playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED,
	pendingDiscarded:    playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED,
}

var outcomeToProto = map[string]playv1.AttackOutcome{
	outcomeHit:  playv1.AttackOutcome_ATTACK_OUTCOME_HIT,
	outcomeCrit: playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT,
	outcomeMiss: playv1.AttackOutcome_ATTACK_OUTCOME_MISS,
}

// rollInput is how the caller says a roll comes: rolled by the app, or a
// number typed from physical dice.
type rollInput struct {
	inApp bool
	typed int
}

// mustRollThisWay checks a player's way of rolling against the campaign's dice
// setting (RN-18): only a mode the master forced binds them (a typed value is
// refused when everybody rolls in the app, the app's roll when everybody rolls
// real dice); with "each player chooses" they pick on every roll, and their
// preference is only the default the screen offers. The master rolls either
// way, for anyone.
func (s *Service) mustRollThisWay(ctx context.Context, m authz.Membership, in rollInput) error {
	if m.Role == authz.RoleMaster {
		return nil
	}
	force, err := s.dice.ForcedDice(ctx, m.CampaignID, m.UserID)
	if err != nil {
		return err
	}
	if force.refuses(in.inApp) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
	}
	return nil
}

// RollAttack implements playv1connect.CombatServiceHandler.
func (s *Service) RollAttack(
	ctx context.Context,
	req *connect.Request[playv1.RollAttackRequest],
) (*connect.Response[playv1.RollAttackResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
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
	attackerID, err := parseCombatID(req.Msg.GetAttackerId(), "combatant")
	if err != nil {
		return nil, err
	}
	targetID, err := parseCombatID(req.Msg.GetTargetId(), "combatant")
	if err != nil {
		return nil, err
	}
	attackKey := req.Msg.GetAttackKey()
	if attackKey == "" || len(attackKey) > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("attack_key must name one of the attacker's attacks"))
	}
	var in rollInput
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.RollAttackRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.RollAttackRequest_D20Face:
		in.typed = int(roll.D20Face)
		if in.typed < 1 || in.typed > 20 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("d20_face must be 1 to 20"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or d20_face"))
	}
	v := viewerOf(m)

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventAttackRolled, encounterID: encID}, func(c *combatTx) (any, error) {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		attacker, err := findCombatant(cs, attackerID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(attacker); err != nil {
			return nil, err
		}
		target, err := findCombatant(cs, targetID, v)
		if err != nil {
			return nil, err
		}
		if target.ID == attacker.ID {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an attacker cannot be its own target"))
		}
		if err := s.mustActNow(ctx, c, attacker); err != nil {
			return nil, err
		}
		if target.Defeated {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_DEFEATED, "the target is defeated")
		}
		// RN-18, then the economy: a player has one action a turn. The master
		// has the last word, and an NPC with several attacks is his to run.
		if !v.master {
			if err := s.mustRollThisWay(ctx, m, in); err != nil {
				return nil, err
			}
			if attacker.ActionUsed {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
			}
		}

		attackerSheet, err := s.roster.CombatSheet(ctx, m.CampaignID, attacker.CharacterID)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(attackerSheet.Attacks, func(a link.Attack) bool { return a.Key == attackKey })
		if i < 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("attack_key is not one of the attacker's attacks"))
		}
		attack := attackerSheet.Attacks[i]
		if attack.Save {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("this attack asks for a saving throw, not an attack roll"))
		}
		if !v.master { // RN-21: the reach is a player's limit; the master has the last word
			if !placed(attacker) || !placed(target) {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PLACED, "the attacker and the target must be on the map")
			}
			dist, _ := distanceFt(attacker, target)
			if reach := reachFt(clamp32(attack.RangeFt, 0, math.MaxInt32), clamp32(attack.LongRangeFt, 0, math.MaxInt32)); dist > reach {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH, "the target is beyond the attack's range",
					func(b *playv1.EncounterBlocked) { b.MissingFt = dist - reach })
			}
		}

		// The roll, and what it did against the target's armor class. The
		// class stays on the server (RN-20).
		expr := dice.Expr{Count: 1, Sides: 20, Modifier: attack.ToHit}
		var roll dice.Result
		if in.inApp {
			if roll, err = dice.Roll(s.roller, expr); err != nil {
				return nil, fmt.Errorf("roll the attack: %w", err)
			}
		} else if roll, err = dice.Physical(expr, in.typed); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("d20_face must be 1 to 20"))
		}
		face := in.typed
		if in.inApp {
			face = roll.Faces[0]
		}
		targetSheet, err := s.roster.CombatSheet(ctx, m.CampaignID, target.CharacterID)
		if err != nil {
			return nil, err
		}
		result := combat.ResolveAttack(attack.ToHit, targetSheet.ArmorClass, face)

		if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{
			ID: attacker.ID, ActionUsed: true, BonusActionUsed: attacker.BonusActionUsed, ReactionUsed: attacker.ReactionUsed, Dashed: attacker.Dashed,
		}); err != nil {
			return nil, fmt.Errorf("spend the action: %w", err)
		}
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Key: attackKey,
			D20: clamp32(face, 1, 20), Modifier: clamp32(attack.ToHit, math.MinInt32, math.MaxInt32), Total: clamp32(result.Total, math.MinInt32, math.MaxInt32),
			Physical: !in.inApp, Outcome: outcomeMiss, ActionBefore: attacker.ActionUsed,
			TargetAC: clamp32(targetSheet.ArmorClass, 0, math.MaxInt32),
		}
		if result.Hit {
			made.Outcome = outcomeHit
			if result.Critical {
				made.Outcome = outcomeCrit
			}
			p, err := c.q.InsertPendingDamage(ctx, playdb.InsertPendingDamageParams{
				EncounterID: c.enc.ID, AttackerID: attacker.ID, TargetID: target.ID, AttackKey: attackKey, Critical: result.Critical,
				DiceCount: clamp32(combat.DiceToRoll(rules.DiceFormula{Count: attack.DiceCount}, result.Critical), 0, 100),
				DiceSides: clamp32(attack.DiceSides, 0, 100), DiceBonus: clamp32(attack.DiceBonus, -1000, 1000),
				DamageType: attack.DamageType, CreatedAt: c.now,
			})
			if err != nil {
				return nil, fmt.Errorf("open the pending damage: %w", err)
			}
			made.Pending = p.ID
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll an attack", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the attack", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(ctx, res, ev.Pending, v)
	if err != nil {
		return nil, err
	}
	roll := &playv1.AttackRoll{
		AttackerId: ev.Actor, TargetId: ev.Target, AttackKey: ev.Key,
		D20: diceRoll(1, 20, faceList(ev), ev.Modifier, ev.Total, ev.Physical), Outcome: outcomeToProto[ev.Outcome],
	}
	if v.master {
		roll.TargetArmorClass = ptr(ev.TargetAC) // "Acertou contra CA 18": never a player's
	}
	return connect.NewResponse(&playv1.RollAttackResponse{Encounter: out, PendingDamage: pending, Roll: roll}), nil
}

// faceList is the face of the d20 of an attack roll; none for a physical one,
// whose d20 was typed (the typed face is still in D20: the master and the
// player know it, so it is sent as the face too).
func faceList(ev actionEvent) []int32 { return []int32{ev.D20} }

// resultEvent is the event a write made: the one the closure made, or, for a
// retry the closure never ran for, the one stored the first time.
func resultEvent(res combatResult, made actionEvent) (actionEvent, error) {
	if !res.repeated {
		return made, nil
	}
	return readEvent(res.payload)
}

// pendingFor reads the pending damage an event is about, for the answer;
// nil when there is none (a miss) or an undo took it away meanwhile.
func (s *Service) pendingFor(ctx context.Context, res combatResult, id string, v combatViewer) (*playv1.PendingDamage, error) {
	if id == "" {
		return nil, nil
	}
	p, err := s.queries.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: res.encounterID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "read the pending damage", err)
	}
	cs, err := s.queries.ListCombatants(ctx, res.encounterID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	if !pendingVisible(p, cs, v) {
		return nil, nil
	}
	return pendingProto(p, cs), nil
}

// RollDamage implements playv1connect.CombatServiceHandler.
func (s *Service) RollDamage(
	ctx context.Context,
	req *connect.Request[playv1.RollDamageRequest],
) (*connect.Response[playv1.RollDamageResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
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
	var in rollInput
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.RollDamageRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.RollDamageRequest_TypedSum:
		in.typed = int(roll.TypedSum)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_sum"))
	}
	v := viewerOf(m)

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventDamageRolled, encounterID: encID}, func(c *combatTx) (any, error) {
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
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		attacker, err := findCombatant(cs, p.AttackerID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(attacker); err != nil {
			return nil, err
		}
		target, _ := findCombatant(cs, p.TargetID, combatViewer{master: true})
		if !v.sees(target) {
			return nil, errCombatantNotFound() // a target hidden after the hit: the master resolves it
		}
		switch p.Status {
		case pendingRolled:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_ALREADY_ROLLED, "the damage was rolled already")
		case pendingApplied, pendingDiscarded:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED, "the damage was applied or discarded already")
		}

		// The damage: the dice, doubled on a critical hit when it hit, plus the
		// modifier once. A flat damage needs no dice (and so no way of rolling).
		expr := dice.Expr{Count: int(p.DiceCount), Sides: int(p.DiceSides), Modifier: int(p.DiceBonus)}
		var roll dice.Result
		if p.DiceCount == 0 {
			roll = dice.Result{Expr: expr, Modifier: expr.Modifier, Total: expr.Modifier}
		} else {
			if !v.master {
				if err := s.mustRollThisWay(ctx, m, in); err != nil {
					return nil, err
				}
			}
			if in.inApp {
				if roll, err = dice.Roll(s.roller, expr); err != nil {
					return nil, fmt.Errorf("roll the damage: %w", err)
				}
			} else if roll, err = dice.Physical(expr, in.typed); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("typed_sum must be %d to %d", expr.Count, expr.Count*expr.Sides))
			}
		}
		amount := clamp32(max(roll.Total, 0), 0, math.MaxInt32) // damage is never below 0
		faces := faces32(roll.Faces)

		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: p.TargetID, Pending: p.ID, Key: p.AttackKey,
			DiceCount: p.DiceCount, DiceSides: p.DiceSides, Modifier: p.DiceBonus, Faces: faces, Amount: amount,
			DamageType: p.DamageType, Critical: p.Critical, Physical: roll.Physical,
		}
		status := pendingRolled // a player's character waits for the master (RN-02)
		if target.Kind == kindNPC {
			// An NPC takes it at once: temporary hit points first, then the
			// hit points; at 0 it is defeated and the turns skip it.
			dmg := combat.ApplyDamage(int(num(target.HpCurrent)), int(num(target.HpTemp)), int(amount))
			after := hpState{HP: clamp32(dmg.HP, 0, math.MaxInt32), Temp: clamp32(dmg.TempHP, 0, math.MaxInt32), Defeated: dmg.HP == 0}
			if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &after.HP, HpTemp: &after.Temp, Defeated: after.Defeated}); err != nil {
				return nil, fmt.Errorf("apply the damage: %w", err)
			}
			before := hpOf(target)
			made.Before, made.After, made.Applied = &before, &after, true
			status = pendingApplied
		}
		if _, err := c.q.SetPendingDamageRolled(ctx, playdb.SetPendingDamageRolledParams{
			ID: p.ID, Status: status, Faces: faces, Physical: roll.Physical, Amount: &amount, ResolvedAt: resolvedAt(status, c),
		}); err != nil {
			return nil, fmt.Errorf("save the damage roll: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll damage", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the damage", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(ctx, res, ev.Pending, v)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RollDamageResponse{Encounter: out, PendingDamage: pending}), nil
}

// resolvedAt is when a pending damage was settled: now for an applied one, not
// yet for one that waits for the master.
func resolvedAt(status string, c *combatTx) *time.Time {
	if status == pendingApplied {
		return &c.now
	}
	return nil
}

// ApplyPendingDamage implements playv1connect.CombatServiceHandler.
func (s *Service) ApplyPendingDamage(
	ctx context.Context,
	req *connect.Request[playv1.ApplyPendingDamageRequest],
) (*connect.Response[playv1.ApplyPendingDamageResponse], error) {
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

	var made actionEvent
	var vitals *playv1.CharacterVitals // the target's, after
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventDamageApplied, encounterID: encID}, func(c *combatTx) (any, error) {
		vitals = nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		p, attacker, target, err := s.settling(ctx, c, pendingID)
		if err != nil {
			return nil, err
		}
		switch p.Status {
		case pendingAwaitingRoll:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_NOT_ROLLED, "the damage was not rolled yet")
		case pendingApplied, pendingDiscarded:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED, "the damage was applied or discarded already")
		}
		if target.Kind != kindPlayer {
			return nil, fmt.Errorf("pending damage %s is rolled for an NPC", p.ID) // an NPC takes it when rolled
		}

		// RN-02: the damage goes through the character's vitals, temporary hit
		// points first, never below 0. At 0 it is down ("Caído"); the death
		// saves and a massive damage's instant death come with the next slice.
		now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, target.CharacterID)
		if err != nil {
			return nil, err
		}
		dmg := combat.ApplyDamage(int(now.GetHitPointsCurrent()), int(now.GetHitPointsTemporary()), int(num(p.Amount)))
		hp, temp := clamp32(dmg.HP, 0, math.MaxInt32), clamp32(dmg.TempHP, 0, math.MaxInt32)
		before, after, err := s.vitals.AdjustVitals(ctx, c.tx, c.session.CampaignID, target.CharacterID,
			&playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: &hp, HitPointsTemporary: &temp})
		if err != nil {
			return nil, err
		}
		vitals = after
		if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingApplied, ResolvedAt: &c.now}); err != nil {
			return nil, fmt.Errorf("apply the pending damage: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &target.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Pending: p.ID, Key: p.AttackKey,
			Amount: num(p.Amount), DamageType: p.DamageType,
			Before: &hpState{HP: before.GetHitPointsCurrent(), Temp: before.GetHitPointsTemporary()},
			After:  &hpState{HP: after.GetHitPointsCurrent(), Temp: after.GetHitPointsTemporary()},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "apply a pending damage", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the applied damage", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
		s.publishVitals(m.CampaignID, vitals)
	})
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(ctx, res, ev.Pending, combatViewer{master: true})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ApplyPendingDamageResponse{Encounter: out, PendingDamage: pending}), nil
}

// publishVitals tells the master and the character's player the vitals
// changed; nothing when the change did not touch any.
func (s *Service) publishVitals(campaignID string, v *playv1.CharacterVitals) {
	if v == nil {
		return
	}
	s.hub.Publish(campaignID, live.Event{
		Audience: vitalsAudience(v),
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_VitalsChanged_{
			VitalsChanged: &playv1.WatchGameSessionResponse_VitalsChanged{Vitals: v},
		}},
	})
}

// settling finds the pending damage the master settles and the two
// combatants it is about, all in the change's own transaction.
func (s *Service) settling(ctx context.Context, c *combatTx, pendingID string) (p playdb.PendingDamage, attacker, target playdb.Combatant, err error) {
	p, err = c.q.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: c.enc.ID, ID: pendingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, attacker, target, connect.NewError(connect.CodeNotFound, errors.New("pending damage not found"))
	}
	if err != nil {
		return p, attacker, target, fmt.Errorf("find the pending damage: %w", err)
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return p, attacker, target, fmt.Errorf("list the combatants: %w", err)
	}
	master := combatViewer{master: true}
	if attacker, err = findCombatant(cs, p.AttackerID, master); err != nil {
		return p, attacker, target, err
	}
	target, err = findCombatant(cs, p.TargetID, master)
	return p, attacker, target, err
}

// DiscardPendingDamage implements playv1connect.CombatServiceHandler.
func (s *Service) DiscardPendingDamage(
	ctx context.Context,
	req *connect.Request[playv1.DiscardPendingDamageRequest],
) (*connect.Response[playv1.DiscardPendingDamageResponse], error) {
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

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventDamageDiscarded, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		p, attacker, target, err := s.settling(ctx, c, pendingID)
		if err != nil {
			return nil, err
		}
		if p.Status == pendingApplied || p.Status == pendingDiscarded {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED, "the damage was applied or discarded already")
		}
		if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingDiscarded, ResolvedAt: &c.now}); err != nil {
			return nil, fmt.Errorf("discard the pending damage: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Pending: p.ID, Key: p.AttackKey,
			Amount: num(p.Amount), PrevStatus: p.Status,
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "discard a pending damage", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the discarded damage", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(ctx, res, ev.Pending, combatViewer{master: true})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.DiscardPendingDamageResponse{Encounter: out, PendingDamage: pending}), nil
}

// TakeAction implements playv1connect.CombatServiceHandler.
func (s *Service) TakeAction(
	ctx context.Context,
	req *connect.Request[playv1.TakeActionRequest],
) (*connect.Response[playv1.TakeActionResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
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
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	actionKey := req.Msg.GetActionKey()
	if actionKey == standardAttack || actionKey == standardCast {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an attack is rolled with RollAttack; casting comes with the next slice"))
	}
	v := viewerOf(m)

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventActionTaken, encounterID: encID}, func(c *combatTx) (any, error) {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		who, err := findCombatant(cs, combID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(who); err != nil {
			return nil, err
		}
		if err := s.mustActNow(ctx, c, who); err != nil {
			return nil, err
		}
		opts, err := s.roster.CombatTurnOptions(ctx, m.CampaignID, who.CharacterID, turnOf(who))
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(opts.GetStandardActions(), func(a *rulesv1.ActionOption) bool { return a.GetAction().GetKey() == actionKey })
		if i < 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action_key is not one of the standard actions"))
		}
		action := opts.GetStandardActions()[i]
		// The master has the last word: he may act again with the action used.
		if !v.master && !action.GetEnabled() {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
		}
		after := who
		switch action.GetAction().GetEconomy() {
		case rulesv1.ActionEconomy_ACTION_ECONOMY_BONUS_ACTION:
			after.BonusActionUsed = true
		case rulesv1.ActionEconomy_ACTION_ECONOMY_REACTION:
			after.ReactionUsed = true
		default:
			after.ActionUsed = true
		}
		if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{
			ID: who.ID, ActionUsed: after.ActionUsed, BonusActionUsed: after.BonusActionUsed, ReactionUsed: after.ReactionUsed, Dashed: after.Dashed,
		}); err != nil {
			return nil, fmt.Errorf("spend the action: %w", err)
		}
		if actionKey == "standard:dash" {
			if err := markDashed(ctx, c.q, who.ID); err != nil {
				return nil, err
			}
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &who.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: who.Hidden, Actor: who.ID, Key: actionKey,
			ActionBefore: who.ActionUsed, BonusBefore: who.BonusActionUsed, ReactionBefore: who.ReactionUsed, DashedBefore: who.Dashed,
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "take an action", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the action", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.TakeActionResponse{Encounter: out}), nil
}

// AdjustCombatantHitPoints implements playv1connect.CombatServiceHandler.
func (s *Service) AdjustCombatantHitPoints(
	ctx context.Context,
	req *connect.Request[playv1.AdjustCombatantHitPointsRequest],
) (*connect.Response[playv1.AdjustCombatantHitPointsResponse], error) {
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
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	var mode string
	var amount int32
	switch change := req.Msg.GetChange().(type) {
	case *playv1.AdjustCombatantHitPointsRequest_Damage:
		mode, amount = "damage", change.Damage
	case *playv1.AdjustCombatantHitPointsRequest_Heal:
		mode, amount = "heal", change.Heal
	case *playv1.AdjustCombatantHitPointsRequest_HitPoints:
		mode, amount = "set", change.HitPoints
	}
	temp := req.Msg.HitPointsTemporary
	switch {
	case mode == "" && temp == nil:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set damage, heal, hit_points or hit_points_temporary"))
	case mode != "" && (amount < 0 || amount > maxHitPointChange):
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s must be 0 to %d", mode, maxHitPointChange))
	case temp != nil && (*temp < 0 || *temp > maxTempHitPoints):
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("hit_points_temporary must be 0 to %d", maxTempHitPoints))
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventHitPointsAdjusted, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		target, err := findCombatant(cs, combID, viewerOf(m))
		if err != nil {
			return nil, err
		}
		if target.Kind != kindNPC {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a player's character keeps its hit points in its vitals: use AdjustCharacterVitals"))
		}
		hp, tmp, hpMax := int(num(target.HpCurrent)), int(num(target.HpTemp)), int(num(target.HpMax))
		switch mode {
		case "damage":
			dmg := combat.ApplyDamage(hp, tmp, int(amount))
			hp, tmp = dmg.HP, dmg.TempHP
		case "heal":
			hp = combat.ApplyHeal(hp, hpMax, int(amount)).HP
		case "set":
			if int(amount) > hpMax {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("hit_points must be 0 to %d", hpMax))
			}
			hp = int(amount)
		}
		if temp != nil {
			tmp = int(*temp)
		}
		after := hpState{HP: clamp32(hp, 0, math.MaxInt32), Temp: clamp32(tmp, 0, math.MaxInt32), Defeated: hp == 0}
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &after.HP, HpTemp: &after.Temp, Defeated: after.Defeated}); err != nil {
			return nil, fmt.Errorf("save the hit points: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		before := hpOf(target)
		c.characterID = &target.CharacterID
		return actionEvent{
			Round: c.enc.Round, Secret: true, Actor: target.ID, Mode: mode, Amount: max(amount, 0),
			Delta:  after.HP - before.HP,
			Before: &before, After: &after,
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "adjust a combatant's hit points", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, false) // the master's correction: his line only
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AdjustCombatantHitPointsResponse{Encounter: out}), nil
}
