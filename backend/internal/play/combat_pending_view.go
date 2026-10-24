package play

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The pending damage as the viewer reads it, and the event a write answers with.

// damageTypePT names the damage types in Portuguese for the log; the keys are
// the content's ("damage-type:fire").
var damageTypePT = map[string]string{
	"damage-type:acid": "ácido", "damage-type:bludgeoning": "concussão", "damage-type:cold": "frio",
	"damage-type:fire": "fogo", "damage-type:force": "energia", "damage-type:lightning": "elétrico",
	"damage-type:necrotic": "necrótico", "damage-type:piercing": "perfurante", "damage-type:poison": "veneno",
	"damage-type:psychic": "psíquico", "damage-type:radiant": "radiante", "damage-type:slashing": "cortante",
	"damage-type:thunder": "trovejante",
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
	if t, ok := owner(p.GetTargetId()); ok && dc > 0 && v.saveDCFor(t) {
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
