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
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// What a turn can do (MR-012, MR-014): the options of a
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
	case !actsNow(e, c): // its group is not on turn, or its part of a joint turn ended
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN, nil
	}
	down, err := s.isDown(ctx, tx, campaignID, c)
	if err != nil {
		return 0, err
	}
	if down {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN, nil
	}
	// An effect or a condition that takes the actions away (RN-22).
	if code := cannotActCode(c); code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return code, nil
	}
	// A surprised combatant does not move, act or react until its first turn ends.
	surprised, err := s.surprisedNow(ctx, tx, e, c)
	if err != nil {
		return 0, err
	}
	if surprised {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED, nil
	}
	return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED, nil
}

// isDown says whether a player's character is at 0 hit points ("Caído"): its
// vitals say so. A character that left the sheet's active state (it died, and the
// combat was not told) is out of the fight too: down, never an error. An NPC and a
// creature are never down: they are defeated.
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
	if connect.CodeOf(err) == connect.CodeNotFound {
		return true, nil
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
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED:
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED, "the combatant is surprised")
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_INCAPACITATED, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_EFFECT_LETHARGY:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_ACT, "the combatant cannot act now")
	}
	// Not on turn, or out of the fight (a defeated combatant has no turn).
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN, "it is not this combatant's turn")
}

// mustNotBeDown refuses a player's character at 0 hit points: a down character
// does not move (RN-03, RN-21). tx is the open transaction, or nil for a read.
func (s *Service) mustNotBeDown(ctx context.Context, tx pgx.Tx, campaignID string, who playdb.Combatant) error {
	down, err := s.isDown(ctx, tx, campaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return nil
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
	if err := s.mustNotHold(ctx, c); err != nil { // a question waits for the master: nobody's turn goes on
		return err
	}
	return s.mustNotWait(ctx, c, who)
}

func turnOf(c playdb.Combatant) link.Turn {
	return link.Turn{
		ActionUsed: c.ActionUsed, BonusActionUsed: c.BonusActionUsed, ReactionUsed: c.ReactionUsed, AttacksMade: int(c.AttacksMade), AttackKey: conv.OrZero(c.ActionAttackKey), FlurryLeft: int(c.BonusAttacksLeft), ActionSurged: c.ActionSurged, SpellCast: c.SpellCast, BonusSpellCast: c.BonusSpellCast,
		Dashed: c.Dashed, SpeedFt: int(max(c.SpeedFt, c.SpeedFlyFt)), MovementUsedFt: int(c.MovementUsedDft) / 10,
		MovementUsedDFt: int(c.MovementUsedDft), LastMoveDFt: int(c.LastMoveDft), Disengaged: c.Disengaged,
		JumpLongDFt: int(c.JumpLongDft), JumpHighDFt: int(c.JumpHighDft),
		JumpLongBonusDFt: int(c.JumpLongBonusDft), JumpDoubled: c.JumpDoubled,
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

// attackReach is how far the attack reaches for a player: its range, or for an
// opportunity attack the melee reach, whatever range the weapon has when thrown.
func attackReach(a link.Attack, asReaction bool) int32 {
	if asReaction {
		return meleeReachFt
	}
	return reachFt(clamp32(a.RangeFt, 0, math.MaxInt32), clamp32(a.LongRangeFt, 0, math.MaxInt32))
}

// distanceFt is the distance between two combatants on the grid for an attack's
// reach or a spell's range: the squares between the centers, rounded down, times
// 5 ft (RN-21, grid.RangeFt), so a diagonal neighbor is 5 ft away; false when
// either has no square.
func distanceFt(a, b playdb.Combatant) (int32, bool) {
	if !placed(a) || !placed(b) {
		return 0, false
	}
	return clamp32(grid.RangeFt(squareOfCombatant(a), squareOfCombatant(b)), 0, math.MaxInt32), true
}

// spellTargetList lists who a spell can target as the viewer sees them: the
// caster itself and every combatant that is not defeated (one at 0 hit points
// can be healed), in turn order, with the distance and whether the spell's range
// cannot reach it. A spell that stays on the caster lists only the caster. In a
// combat without a grid (theatre) there is no distance and nothing is too far: the
// master judges the range (RN-25).
func spellTargetList(terrain grid.Terrain, cs []playdb.Combatant, caster playdb.Combatant, v combatViewer, sp link.Spell, theatre bool) []*playv1.TargetInReach {
	reach, limited := reachOf(sp)
	pool := coverPool(cs, v)
	var out []*playv1.TargetInReach
	for _, t := range cs {
		if t.Defeated || !v.sees(t) || (selfOnly(sp) && t.ID != caster.ID) {
			continue
		}
		cover := coverAgainst(terrain, caster, t, pool)
		target := withCover(&playv1.TargetInReach{CombatantId: t.ID, Label: t.Label, State: stateOf(t)}, cover)
		target.Untargetable = cover.total() && !v.master && !sp.Area && t.ID != caster.ID
		if theatre {
			out = append(out, target)
			continue
		}
		if dist, ok := distanceFt(caster, t); ok {
			target.DistanceFt = &dist
			target.TooFar = limited && t.ID != caster.ID && dist > reach
		} else if t.ID == caster.ID {
			dist := int32(0)
			target.DistanceFt = &dist
		} else {
			target.TooFar = limited && !v.master // a player cannot cast at what has no square (CastSpell)
		}
		out = append(out, target)
	}
	return out
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
// is too far. In a combat without a grid (theatre) there is no distance and
// nothing is too far: every combatant the rules allow is a target, and the master
// judges the reach (RN-25).
func targetsFor(terrain grid.Terrain, cs []playdb.Combatant, attacker playdb.Combatant, v combatViewer, reach int32, theatre bool) []*playv1.TargetInReach {
	pool := coverPool(cs, v)
	var out []*playv1.TargetInReach
	for _, t := range cs {
		if t.ID == attacker.ID || t.Defeated || !v.sees(t) {
			continue
		}
		cover := coverAgainst(terrain, attacker, t, pool)
		target := withCover(&playv1.TargetInReach{CombatantId: t.ID, Label: t.Label, State: stateOf(t)}, cover)
		target.Untargetable = cover.total() && !v.master
		if theatre {
			out = append(out, target)
			continue
		}
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

// withCover puts the cover a target has against the attacker on its list entry.
func withCover(t *playv1.TargetInReach, cv coverView) *playv1.TargetInReach {
	t.Cover, t.CoverSource = coverDegreeProto(cv.key()), cv.source
	return t
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
		Id: p.ID, AttackerId: conv.OrZero(p.AttackerID), TargetId: p.TargetID, AttackKey: p.AttackKey,
		Status: pendingStatusToProto[p.Status], Critical: p.Critical,
		CriticalRule: pendingCriticalRule(p.Critical, p.CriticalMaxRule), CriticalMax: p.CriticalMax,
		DiceCount: p.DiceCount, DiceSides: p.DiceSides, Bonus: p.DiceBonus,
		ExtraDiceCount: p.ExtraDice, ExtraDiceNamePt: extraDiceName(p.ExtraDice, p.SavageDice),
		DamageTypeKey: p.DamageType, DamageTypePt: damageTypePT[p.DamageType],
		CastId: conv.OrZero(p.CastID), Healing: p.Healing, Half: p.Half, AppliedAmount: p.AppliedAmount,
		TrapPointId: conv.OrZero(p.TrapPointID), // a trap's damage has no attacker (MR-035)
	}
	if p.Amount != nil {
		out.Amount = *p.Amount
		total := *p.Amount
		if p.RollTotal != nil { // a half damage's roll is the whole one
			total = *p.RollTotal
		}
		out.Roll = diceRoll(p.DiceCount+p.ExtraDice, p.DiceSides, p.Faces, p.DiceBonus+p.CriticalMax, total, p.Physical)
	}
	if p.Status == pendingApplied {
		out.TargetDefeated = slices.ContainsFunc(cs, func(c playdb.Combatant) bool { return c.ID == p.TargetID && holdsHP(c) && c.Defeated })
	}
	return out
}

var pendingStatusToProto = map[string]playv1.PendingDamageStatus{
	pendingAwaitingReaction: playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_REACTION,
	pendingAwaitingRoll:     playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL,
	pendingRolled:           playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED,
	pendingApplied:          playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED,
	pendingDiscarded:        playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED,
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
	// typedFaces are the faces of a physical d20 pair (advantage or disadvantage,
	// d20_faces); typed alone is one face.
	typedFaces []int
	// pool says typed is the sum of the physical dice of a spell's pool (not a
	// d20 face): CastSpell's pool_sum.
	pool bool
	// extraFaces are the faces of the physical dice an effect adds to the roll
	// (extra_die_faces: Bênção, Orientação).
	extraFaces []int32
}

// mustRollThisWay checks a player's way of rolling against the campaign's dice
// setting (RN-18): only a mode the master forced binds them (a typed value is
// refused when everybody rolls in the app, the app's roll when everybody rolls
// real dice); with "each player chooses" they pick on every roll, and their
// preference is only the default the screen offers. The master rolls either
// way, for anyone. Inside a change it reads the setting in the change's
// transaction (tx).
func (s *Service) mustRollThisWay(ctx context.Context, tx pgx.Tx, m authz.Membership, in rollInput) error {
	if m.Role == authz.RoleMaster {
		return nil
	}
	force, err := s.dice.ForcedDice(ctx, tx, m.CampaignID, m.UserID)
	if err != nil {
		return err
	}
	if force.refuses(in.inApp) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
	}
	return nil
}

// mustReactNow checks, inside a write, that the combatant may spend its
// reaction now: the combat is running, it is not out of the fight or down, and
// it is not its own turn (then an attack spends the action).
func (s *Service) mustReactNow(ctx context.Context, c *combatTx, who playdb.Combatant, onOffer bool) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	switch {
	case c.enc.Status != statusActive:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE)
	case who.Defeated:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED)
	case actsNow(c.enc, who) && !onOffer: // an opportunity attack may come in a joint turn, on any turn (SRD)
		return connect.NewError(connect.CodeInvalidArgument, errors.New("on its own turn an attack spends the action, not the reaction"))
	}
	down, err := s.isDown(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return s.mustNotBeSurprised(ctx, c, who)
}

// resultEvent is the event a write made: the one the closure made, or, for a
// retry the closure never ran for, the one stored the first time.
func resultEvent(res combatResult, made actionEvent) (actionEvent, error) {
	if !res.repeated {
		if res.stamped != nil {
			return *res.stamped, nil // with who could see it (combat_fog.go)
		}
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
	out := s.pendingView(ctx, res.session.CampaignID, p, cs, v)
	if p.TrapPointID != nil && v.master && s.traps != nil { // a fired trap is public; its name is the master's card's
		if names, err := s.traps.TrapNames(ctx, res.session.CampaignID, []string{*p.TrapPointID}); err == nil {
			out.TrapName = names[*p.TrapPointID]
		}
	}
	return out, nil
}

// afterResistance is the damage that lands on the target once its stat block's
// resistances, vulnerabilities and immunities to the damage type are applied, after
// the bonuses and the half of a saving throw (SRD 5.1). Only a target that takes
// the damage at once and has a stat block is adjusted: an NPC made from a creature
// and a creature. A player's character keeps its Rage and other resistances as
// notes, for the master to apply when he sets the amount (RN-02).
func (s *Service) afterResistance(ctx context.Context, c *combatTx, target playdb.Combatant, damageType string, amount int32) (int32, error) {
	if !holdsHP(target) || damageType == "" || amount <= 0 {
		return amount, nil
	}
	mods, err := s.roster.DamageModifiers(ctx, c.tx, c.session.CampaignID, target.CharacterID, conv.OrZero(target.MonsterKey))
	if err != nil {
		return 0, err
	}
	return clamp32(combat.AdjustForType(int(amount), damageType, mods), 0, math.MaxInt32), nil
}

// landDamage puts a rolled damage or heal on its target, as far as the target's
// kind lets it land at once. An NPC takes a damage at once: temporary hit points
// first, then the hit points, defeated at 0 (and a heal puts it back above 0).
// A heal lands at once on a player's character too, up to its maximum, through
// the vitals (a character healed from 0 gets up and its death saves reset). A
// damage on a player's character waits for the master: nothing lands. The hit
// says what changed, with what an undo needs.
func (s *Service) landDamage(ctx context.Context, c *combatTx, p playdb.PendingDamage, target playdb.Combatant, amount int32) (damageHit, *playv1.CharacterVitals, error) {
	hit := damageHit{Pending: p.ID, Target: target.ID, Amount: amount, Half: p.Half}
	switch {
	case p.Healing:
		healed, vit, err := s.healCombatant(ctx, c, target, amount)
		healed.Pending, healed.Half = p.ID, p.Half
		return healed, vit, err
	case holdsHP(target): // an NPC and a creature take a damage at once
		dmg := combat.ApplyDamage(int(conv.OrZero(target.HpCurrent)), int(conv.OrZero(target.HpTemp)), int(amount))
		after := hpState{HP: clamp32(dmg.HP, 0, math.MaxInt32), Temp: clamp32(dmg.TempHP, 0, math.MaxInt32), Defeated: dmg.HP == 0}
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &after.HP, HpTemp: &after.Temp, Defeated: after.Defeated}); err != nil {
			return hit, nil, fmt.Errorf("apply the damage: %w", err)
		}
		before := hpOf(target)
		hit.Before, hit.After, hit.Applied = &before, &after, true
		hit.ConcentrationDC = concentrationDC(target, amount-clamp32(dmg.Absorbed, 0, math.MaxInt32))
	}
	return hit, nil, nil
}

// healCombatant heals a combatant: an NPC's hit points, up to its maximum (it
// comes back above 0), or a player's character's through the vitals, up to its
// maximum (healed from 0 it gets up and its death saves reset, RN-03). The hit
// says how much it regained, with what an undo needs; the vitals are the
// character's after.
func (s *Service) healCombatant(ctx context.Context, c *combatTx, target playdb.Combatant, amount int32) (damageHit, *playv1.CharacterVitals, error) {
	hit := damageHit{Target: target.ID, Applied: true}
	if holdsHP(target) {
		before := hpOf(target)
		r := combat.ApplyHeal(int(before.HP), int(conv.OrZero(target.HpMax)), int(amount))
		after := hpState{HP: clamp32(r.HP, 0, math.MaxInt32), Temp: before.Temp, Defeated: r.HP == 0}
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &after.HP, HpTemp: &after.Temp, Defeated: after.Defeated}); err != nil {
			return hit, nil, fmt.Errorf("apply the heal: %w", err)
		}
		hit.Before, hit.After, hit.Amount = &before, &after, clamp32(r.Healed, 0, math.MaxInt32)
		return hit, nil, nil
	}
	now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, target.CharacterID)
	if err != nil {
		return hit, nil, err
	}
	if w := now.GetWildShape(); w != nil {
		// In a beast form the healing is the beast's (MR-037).
		r := combat.ApplyHeal(int(w.GetHitPointsCurrent()), int(w.GetHitPointsMax()), int(amount))
		hp := clamp32(r.HP, 0, math.MaxInt32)
		before, after, err := s.vitalsOf(ctx, c, target.CharacterID, &playv1.AdjustCharacterVitalsRequest{WildShapeHitPointsCurrent: &hp})
		if err != nil {
			return hit, nil, err
		}
		hit.DeathBefore = deathOf(target)
		hit.Before, hit.After = new(hpStateOf(before)), new(hpStateOf(after))
		hit.Amount = clamp32(r.Healed, 0, math.MaxInt32)
		return hit, after, nil
	}
	r := combat.ApplyHeal(int(now.GetHitPointsCurrent()), int(now.GetHitPointsMax()), int(amount))
	hp := clamp32(r.HP, 0, math.MaxInt32)
	before, after, err := s.vitalsOf(ctx, c, target.CharacterID, &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: &hp})
	if err != nil {
		return hit, nil, err
	}
	hit.DeathBefore = deathOf(target)
	hit.Before = new(hpStateOf(before))
	hit.After = new(hpStateOf(after))
	hit.Amount = clamp32(r.Healed, 0, math.MaxInt32)
	return hit, after, nil
}

// concentrationDC is the Constitution save DC to keep concentrating after a
// combatant took damage (RN-22): 0 when it does not concentrate or took none.
// The damage it takes is what temporary hit points did not absorb, even when it
// drops the combatant below 0.
func concentrationDC(target playdb.Combatant, taken int32) int32 {
	if target.ConcentrationSpell == nil || taken <= 0 {
		return 0
	}
	return clamp32(combat.ConcentrationDC(int(taken)), 0, math.MaxInt32)
}

// takenBy is what a damage of amount cost a combatant: what temporary hit points
// did not absorb, even when it drops the combatant below 0.
func takenBy(amount int32, before, after *hpState) int32 {
	if before == nil || after == nil {
		return max(amount, 0)
	}
	return max(amount-max(before.Temp-after.Temp, 0), 0)
}

// castConcentrationDC notes what a damage of a spell cost its target (taken) and
// gives the Constitution save DC the target owes for the whole cast (RN-22): the
// SRD asks one save for each source of damage, and a spell is one source however
// many damage types it deals. It is 0 while another pending damage of the cast is
// still to be settled for the target, and when the target does not concentrate;
// the pending damage that settles last carries the DC of the sum of what the
// cast's damages cost the target.
func (s *Service) castConcentrationDC(ctx context.Context, c *combatTx, p playdb.PendingDamage, target playdb.Combatant, taken int32) (int32, error) {
	if err := c.q.SetPendingDamageTaken(ctx, playdb.SetPendingDamageTakenParams{ID: p.ID, Taken: &taken}); err != nil {
		return 0, fmt.Errorf("note what the damage cost: %w", err)
	}
	all, err := c.q.ListCastPendingDamages(ctx, playdb.ListCastPendingDamagesParams{EncounterID: c.enc.ID, CastID: p.CastID})
	if err != nil {
		return 0, fmt.Errorf("list the cast's pending damage: %w", err)
	}
	var total int32
	for _, o := range all {
		if o.TargetID != p.TargetID || o.Healing {
			continue
		}
		switch o.Status {
		case pendingApplied:
			total = clamp32(int(total)+int(conv.OrZero(o.Taken)), 0, math.MaxInt32)
		case pendingDiscarded:
		default:
			return 0, nil // another damage type of the cast has still to land on it
		}
	}
	return concentrationDC(target, total), nil
}

// decorate adds to a pending damage what its event knows and the row does not:
// the concentration DC (for the master and the target's own player) and the
// death save failures a damage at 0 caused.
func decorate(p *playv1.PendingDamage, ev actionEvent, v combatViewer, owner func(targetID string) (playdb.Combatant, bool)) {
	if p == nil {
		return
	}
	var dc int32
	if p.GetId() == ev.Pending {
		dc = ev.ConcentrationDC
		p.DeathFailuresAdded = ev.FailuresAdded
		if t, ok := owner(p.GetTargetId()); ok && v.hiddenFrom(ev.DeathHidden, t) { // RN-24: the owner's and the master's
			p.DeathFailuresAdded = 0
		}
	}
	for _, h := range ev.Settled {
		if h.Pending == p.GetId() {
			dc = h.ConcentrationDC
		}
	}
	if t, ok := owner(p.GetTargetId()); ok && dc > 0 && (v.master || v.owns(t)) {
		p.ConcentrationDc = &dc
	}
}

// membersOf returns a lookup of the combat's combatants, for decorate.
func (s *Service) membersOf(ctx context.Context, res combatResult) func(string) (playdb.Combatant, bool) {
	cs, _ := s.queries.ListCombatants(ctx, res.encounterID)
	return func(id string) (playdb.Combatant, bool) {
		i := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == id })
		if i < 0 {
			return playdb.Combatant{}, false
		}
		return cs[i], true
	}
}

// resolvedAt is when a pending damage was settled: now for an applied one, not
// yet for one that waits for the master.
func resolvedAt(status string, c *combatTx) *time.Time {
	if status == pendingApplied {
		return &c.now
	}
	return nil
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
	if target, err = findCombatant(cs, p.TargetID, master); err != nil {
		return p, attacker, target, err
	}
	if p.AttackerID == nil {
		return p, target, target, nil // a trap's damage (MR-035) has no attacker: the target stands in
	}
	attacker, err = findCombatant(cs, *p.AttackerID, master)
	return p, attacker, target, err
}

// discardHit drops a pending damage the master discards, and returns the windows that
// waited on it (a hit's Shield, or the master's check): they close with it
// (settleReactions finds their trigger gone), and the discard's event keeps them so that
// its undo opens them again (reopenWindows).
func discardHit(ctx context.Context, c *combatTx, p playdb.PendingDamage) ([]string, error) {
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return nil, err
	}
	var windows []string
	for _, w := range open {
		if conv.OrZero(w.PendingDamageID) == p.ID {
			windows = append(windows, w.ID)
		}
	}
	if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingDiscarded, ResolvedAt: &c.now}); err != nil {
		return nil, fmt.Errorf("discard the pending damage: %w", err)
	}
	return windows, nil
}

// The feature actions with their own effect on the numbers (the others only
// spend the economy and a use and go to the log). The keys are the SRD feature
// keys ("feature:second-wind").
const (
	secondWind  = "feature:second-wind"
	actionSurge = "feature:action-surge-1-use"
)

// featureError is the failed_precondition for a reason a feature action is
// disabled with. The master has the last word on the economy only: a spent
// resource stops him too (he corrects the uses).
func featureError(r *rulesv1.DisabledReason, master bool) error {
	switch r.GetCode() {
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED:
		if !master {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
		}
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_USED:
		if !master {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED, "the bonus action of this turn is used")
		}
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_USED:
		if !master {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_USED, "the reaction is used")
		}
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ALREADY_USED_THIS_TURN:
		if !master {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ALREADY_USED_THIS_TURN, "already used in this turn")
		}
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ATTACK_ACTION_FIRST:
		if !master {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ATTACK_ACTION_FIRST, "Flurry of Blows comes after the Attack action")
		}
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES, "no uses left",
			func(b *playv1.EncounterBlocked) { b.Recharge = r.GetRecharge() })
	}
	return nil
}

// mustReactNowOrOnTurn checks that a combatant may take a reaction now: the
// combat is running and it is not out of the fight or down; whether it is its
// own turn does not matter.
func (s *Service) mustReactNowOrOnTurn(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	switch {
	case c.enc.Status != statusActive:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE)
	case who.Defeated:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED)
	}
	down, err := s.isDown(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return s.mustNotBeSurprised(ctx, c, who)
}

// secondWind is Retomar o fôlego: 1d10 plus the fighter's level of hit points,
// rolled like a damage (RN-18) and healed through the vitals (or the NPC's hit
// points). It fills the event with the roll and what changed, and returns the
// vitals of a player's character.
func (s *Service) secondWind(ctx context.Context, c *combatTx, m authz.Membership, v combatViewer, who playdb.Combatant, fighterLevel int, in rollInput, rolled bool, ev *actionEvent) (*playv1.CharacterVitals, error) {
	if !rolled {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_sum for Retomar o fôlego"))
	}
	if !v.master {
		if err := s.mustRollThisWay(ctx, c.tx, m, in); err != nil {
			return nil, err
		}
	}
	expr := dice.Expr{Count: 1, Sides: 10, Modifier: fighterLevel}
	var roll dice.Result
	var err error
	if in.inApp {
		if roll, err = dice.Roll(s.roller, expr); err != nil {
			return nil, fmt.Errorf("roll the d10: %w", err)
		}
	} else if roll, err = dice.Physical(expr, in.typed); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_sum must be 1 to 10"))
	}
	hit, vit, err := s.healCombatant(ctx, c, who, clamp32(max(roll.Total, 0), 0, math.MaxInt32))
	if err != nil {
		return nil, err
	}
	ev.Heal, ev.DiceCount, ev.DiceSides, ev.Faces = true, 1, 10, faces32(roll.Faces)
	ev.Modifier, ev.Total, ev.Physical = clamp32(fighterLevel, 0, 100), clamp32(roll.Total, 0, math.MaxInt32), roll.Physical
	ev.Amount, ev.Before, ev.After, ev.DeathBefore = hit.Amount, hit.Before, hit.After, hit.DeathBefore
	return vit, nil
}
