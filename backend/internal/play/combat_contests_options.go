package play

import (
	"context"
	"math"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What GetTurnOptions says of the contests (SRD 5.1, "Grappling", "Shoving a Creature",
// "Hide", "Help", "Surprise"): the special attacks under "Atacar", the facts of the
// combatant's turn (grappled, holding, hidden, surprised) and the Hide and Help actions.

// contestTargets lists who a grapple or a shove may aim at, as the viewer sees them: every
// combatant they see, not defeated, within the melee reach (the whole table without a map,
// where the master judges the reach); a creature more than one size larger is listed with
// its reason. The master has the last word: nothing is refused to him.
func contestTargets(enc playdb.Encounter, cs []playdb.Combatant, who playdb.Combatant, v combatViewer) []*playv1.ContestTarget {
	var out []*playv1.ContestTarget
	for _, t := range cs {
		if t.ID == who.ID || t.Defeated || !v.sees(t) {
			continue
		}
		target := &playv1.ContestTarget{CombatantId: t.ID, Size: sizeProto(t.Size), Eligible: true}
		if !isTheatre(enc) {
			dist, ok := distanceFt(who, t)
			switch {
			case !ok && !v.master:
				continue // a player cannot reach what has no square
			case ok && dist > combat.MeleeReachFt && !v.master:
				continue
			case ok:
				target.DistanceFt = dist
			}
		}
		if !v.master && !combat.CanGrappleOrShove(who.Size, t.Size) {
			target.Eligible, target.Reason = false, playv1.ContestTargetReason_CONTEST_TARGET_REASON_TOO_BIG
		}
		out = append(out, target)
	}
	return out
}

// contestOptionsFor works out the contest side of GetTurnOptions for a combatant: the
// special attacks that replace one attack of the Attack action, and the state of its turn. gate
// is why the combatant cannot act now (UNSPECIFIED when it can).
//
//nolint:gocyclo // one flat list of the reasons a special attack is off, as the turn options list the others
func (s *Service) contestOptionsFor(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer, who playdb.Combatant, opts *rulesv1.TurnOptions, gate rulesv1.DisabledReasonCode) ([]*playv1.ContestAttackOption, *playv1.ContestTurnState, error) {
	cd, err := s.loadContestData(ctx, s.queries, d.enc, d.cs)
	if err != nil {
		return nil, nil, err
	}
	r := &contestReader{s: s, m: m, v: v, d: cd}
	state := &playv1.ContestTurnState{Hidden: isHidden(cd.hiding, who.ID)}

	// The grapple: who holds this combatant (never a creature the viewer does not see), and
	// who it holds.
	hold, grappled := holdOn(holdsOf(cd.holds), who)
	if grappled {
		state.Grappled = true
		if grappler, ok := combatantByID(d.cs, hold.GrapplerID); ok && (v.master || v.sees(grappler)) {
			state.GrapplerId = grappler.ID
		}
		state.CanEscape, state.EscapeReason = true, gateReason(gate)
		if state.EscapeReason == nil && who.ActionUsed && !v.master {
			state.CanEscape, state.EscapeReason = false, &rulesv1.DisabledReason{Code: rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED}
		} else if state.EscapeReason != nil {
			state.CanEscape = false
		}
		for _, sk := range []string{skillAthletics, skillAcrobat} {
			n, notes, err := r.optionOf(ctx, who, sk)
			if err != nil {
				return nil, nil, err
			}
			state.EscapeOptions = append(state.EscapeOptions, &playv1.ContestSkillOption{
				Skill: skillEnum(sk), Modifier: clamp32(n.Bonus, math.MinInt32, math.MaxInt32), Known: n.Known || !v.master,
				Mode: checkModeProto(notesMode(notes)), Notes: notesProto(notes),
			})
		}
	}
	for _, h := range cd.holds {
		if h.GrapplerID != who.ID {
			continue
		}
		if g, ok := combatantByID(d.cs, h.GrappledID); ok && isGrappled(g) && (v.master || v.sees(g)) {
			state.HoldingId = g.ID
			break
		}
	}
	surprised, err := s.surprisedNow(ctx, nil, d.enc, who)
	if err != nil {
		return nil, nil, err
	}
	state.Surprised = surprised
	for _, h := range cd.helps {
		if h.Kind == helpAttack && h.AllyCharacterID == who.CharacterID {
			if t, ok := combatantByID(d.cs, deref(h.TargetID)); ok && (v.master || v.sees(t)) {
				state.HelpTargetId = t.ID
			}
		}
	}
	if err := s.hideAndHelpActions(ctx, m, who, opts, state, r); err != nil {
		return nil, nil, err
	}

	// The special attacks: they replace one attack of the Attack action.
	if mayAttack(who, false) != nil {
		return nil, state, nil //nolint:nilerr // a combatant that cannot attack has no special attack to list; that is no failure
	}
	sheet, err := s.sheetOf(ctx, nil, m.CampaignID, who)
	if err != nil {
		return nil, nil, err
	}
	perAction := max(sheet.AttacksPerAction, 1)
	last, hasLast := lastAttack(sheet, who)
	castCantrip := hasLast && last.Spell
	left := combat.AttacksLeft(perAction, combat.TurnState{ActionUsed: who.ActionUsed, AttacksMade: int(who.AttacksMade)})
	if castCantrip {
		left = 0
	}
	reason := gateReason(gate)
	if reason == nil && left == 0 && !v.master {
		code := rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED
		if !castCantrip && who.AttacksMade > 0 && perAction > 1 {
			code = rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ATTACKS_USED
		}
		reason = &rulesv1.DisabledReason{Code: code}
	}
	roll, _, err := s.contestRollOption(ctx, r, who)
	if err != nil {
		return nil, nil, err
	}
	targets := contestTargets(d.enc, d.cs, who, v)
	var out []*playv1.ContestAttackOption
	for _, kind := range []playv1.ContestAttackOptionKind{playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_SHOVE} {
		out = append(out, &playv1.ContestAttackOption{
			Kind: kind, ReplacesAttack: true, Enabled: reason == nil, Reason: reason, AttacksLeft: clamp32(left, 0, math.MaxInt32),
			Targets: targets, RollOption: roll,
		})
	}
	return out, state, nil
}

// contestRollOption is what the combatant's Strength (Athletics) check would roll.
func (s *Service) contestRollOption(ctx context.Context, r *contestReader, who playdb.Combatant) (*playv1.CheckOption, []rollNote, error) {
	n, notes, err := r.optionOf(ctx, who, skillAthletics)
	if err != nil {
		return nil, nil, err
	}
	return &playv1.CheckOption{Modifier: clamp32(n.Bonus, math.MinInt32, math.MaxInt32), Known: n.Known || !r.v.master, Mode: checkModeProto(notesMode(notes)), Notes: notesProto(notes)}, notes, nil
}

// hideAndHelpActions fills what the sheet's actions say of Hide and Help: the standard Hide
// and the features that give it (Cunning Action), the Help, each with its economy and
// whether it can be taken now, and what a Hide would roll.
func (s *Service) hideAndHelpActions(ctx context.Context, m authz.Membership, who playdb.Combatant, opts *rulesv1.TurnOptions, state *playv1.ContestTurnState, r *contestReader) error {
	ref := func(a *rulesv1.ActionOption) *playv1.ContestAction {
		return &playv1.ContestAction{Key: a.GetAction().GetKey(), Economy: a.GetAction().GetEconomy(), Enabled: a.GetEnabled(), Reason: a.GetReason()}
	}
	for _, a := range opts.GetStandardActions() {
		switch a.GetAction().GetKey() {
		case actionHide:
			state.HideActions = append(state.HideActions, ref(a))
		case actionHelp:
			state.HelpAction = ref(a)
		}
	}
	if len(opts.GetFeatureActions()) > 0 {
		sheet, err := s.sheetOf(ctx, nil, m.CampaignID, who)
		if err != nil {
			return err
		}
		for _, a := range opts.GetFeatureActions() {
			if i := slices.IndexFunc(sheet.FeatureActions, func(fa link.FeatureAction) bool { return fa.Key == a.GetAction().GetKey() }); i >= 0 && sheet.FeatureActions[i].Standard == actionHide {
				state.HideActions = append(state.HideActions, ref(a))
			}
		}
	}
	if len(state.HideActions) > 0 {
		n, notes, err := r.optionOf(ctx, who, skillStealth)
		if err != nil {
			return err
		}
		state.HideOption = &playv1.CheckOption{Modifier: clamp32(n.Bonus, math.MinInt32, math.MaxInt32), Known: n.Known || !r.v.master, Mode: checkModeProto(notesMode(notes)), Notes: notesProto(notes)}
	}
	return nil
}

// gateReason is the reason a gate code gives for a disabled option, nil when the combatant can act.
func gateReason(code rulesv1.DisabledReasonCode) *rulesv1.DisabledReason {
	if code == rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return nil
	}
	return &rulesv1.DisabledReason{Code: code}
}
