package play

import (
	"context"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// audienceOwner is the audience of an effect only its target's player reads.
const audienceOwner = "owner"

// The lines of the log for effects that last (RN-22). A line the master hides is the master's
// alone (the event is Secret); one for the owner alone is the owner's and the master's; the roll
// of a saving throw is the master's and the target's player's, the DC the master's (RN-10).

// effectLogKinds are the events that make an effect line.
var effectLogKinds = map[string]playv1.CombatLogKind{
	eventLastingAdded:      playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventLastingChanged:    playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventLastingEnded:      playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventLastingSaved:      playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventLastingTriggered:  playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventLastingVisibility: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT,
	eventExhaustion:        playv1.CombatLogKind_COMBAT_LOG_KIND_EXHAUSTION,
}

var effectChangeToProto = map[string]playv1.CombatLogEffectChange{
	"added":    playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ADDED,
	"ended":    playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ENDED,
	"duration": playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_DURATION_CHANGED,
	"saved":    playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVED,
	"failed":   playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_FAILED,
	"damage":   playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_DAMAGE,
	"skipped":  playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_SKIPPED,
	"temp_hp":  playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_TEMP_HP,
}

// kindKeyOf is the event kind that makes a line of the kind, "" for a kind that is no effect's.
func kindKeyOf(k playv1.CombatLogKind) string {
	if k == playv1.CombatLogKind_COMBAT_LOG_KIND_EXHAUSTION {
		return eventExhaustion
	}
	if k == playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT {
		return eventLastingAdded
	}
	return ""
}

// effectLineVisible says whether the viewer gets the line: the master always, a player the
// lines that are not for the owner alone, and those only for a target they own.
func effectLineVisible(l *lastingEvent, v combatViewer, byID map[string]playdb.Combatant) bool {
	if v.master {
		return true
	}
	if l.Change == "visibility" {
		return false // the master's switch is his
	}
	if !l.OwnerOnly {
		return true
	}
	return slices.ContainsFunc(l.Targets, func(id string) bool { return v.owns(byID[id]) })
}

// effectLogView writes the effect part of a line for the viewer.
func (s *Service) effectLogView(ctx context.Context, e *logEntry, out *playv1.CombatLogEntry, v combatViewer, byID map[string]playdb.Combatant, campaignID string) {
	l := e.ev.Lasting
	if l == nil {
		return
	}
	names := s.namesFor(ctx, campaignID)
	if out.Kind == playv1.CombatLogKind_COMBAT_LOG_KIND_EXHAUSTION {
		out.Effect = &playv1.CombatLogEffect{SourceKey: l.Key, SourceNamePt: names(l.Key), ExhaustionLevel: l.Level, ExhaustionBefore: l.Before}
		for _, id := range l.Targets {
			out.Effect.TargetLabels = append(out.Effect.TargetLabels, byID[id].Label)
		}
		return
	}
	le := &playv1.CombatLogEffect{
		SourceKey: l.Key, SourceNamePt: names(l.Key), Change: effectChangeToProto[l.Change],
		Ability: l.Ability, Amount: l.Amount, Reason: l.Reason,
	}
	if l.Ability != "" {
		le.AbilityNamePt = names("ability:" + l.Ability)
	}
	if l.DType != "" {
		le.DamageTypePt = names("damage-type:" + l.DType)
	}
	for _, id := range l.Targets {
		if t, ok := byID[id]; ok && (v.master || !t.Hidden) {
			le.TargetLabels = append(le.TargetLabels, t.Label)
		}
	}
	// The roll is the master's and the target's player's; the DC the master's alone (RN-10:
	// the DC of an NPC's effect never reaches a player).
	if v.master || slices.ContainsFunc(l.Targets, func(id string) bool { return v.owns(byID[id]) }) {
		if l.D20 != 0 {
			d20, total := l.D20, l.Total
			le.D20, le.Total = &d20, &total
		}
	}
	if v.master && l.DC != 0 {
		dc := l.DC
		le.Dc = &dc
	}
	out.Effect = le
}
