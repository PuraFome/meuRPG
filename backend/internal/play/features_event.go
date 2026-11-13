package play

import (
	"context"
	"fmt"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// featureUseEvent is what a feature_used event keeps of the use: IDs, keys and numbers only (the choice's text and the
// feature's name are read from the sheet when the log is shown). The event's own fields hold the rest: Actor, Target (the
// only target, when it is not the user), Key (the feature), the economy before, FeatureBefore (the feature state before),
// Hits (what each target's hit points were before, for the undo), Lasting (the effect rows made), Resource and PointsSpent.
type featureUseEvent struct {
	// Scope is the scope the use was counted in ("" for a feature with no limit), Choice the key picked.
	Scope  string `json:"scope,omitempty"`
	Choice string `json:"choice,omitempty"`
	// Targets are the creatures the use was aimed at (the user alone when it names none, and then Targets is empty),
	// each with what it got.
	Targets []featureTargetUse `json:"targets,omitempty"`
	// Healed and TempHP are the totals over the targets, Effect says a lasting effect was put on them.
	Healed int32 `json:"healed,omitempty"`
	TempHP int32 `json:"temp_hp,omitempty"`
	Effect bool  `json:"effect,omitempty"`
	// Faces and Total are the dice the server rolled for the heal (one roll, given to every target).
	Faces []int32 `json:"faces,omitempty"`
	Total int32   `json:"total,omitempty"`
}

// featureTargetUse is what one target got.
type featureTargetUse struct {
	ID     string `json:"id"`
	Healed int32  `json:"healed,omitempty"`
	TempHP int32  `json:"temp_hp,omitempty"`
}

// featureEffectsBack takes away the effects the use put on its targets, and works the targets out again.
func (u *undoer) featureEffectsBack(ctx context.Context, ev actionEvent) error {
	if ev.Lasting == nil {
		return nil
	}
	for _, id := range ev.Lasting.Effects {
		if err := u.c.q.DeleteLastingEffect(ctx, id); err != nil {
			return fmt.Errorf("take the effect of the use away: %w", err)
		}
	}
	return u.s.refreshCombatants(ctx, u.c, ev.Lasting.Targets...)
}

// featureUsed puts back what a use did: the economy, the feature's state, the use of the resource, the hit points of
// each target and the effects the use put on them. The turn's other changes after it are not there: a later event closes the
// chain (the undo takes only the last action).
func (u *undoer) featureUsed(ctx context.Context, ev actionEvent) error {
	who, ok := u.find(ev.Actor)
	if !ok {
		return nil
	}
	if err := u.setEconomy(ctx, who, ev.ActionBefore, ev.BonusBefore, ev.ReactionBefore, ev.DashedBefore); err != nil {
		return err
	}
	if err := u.setRun(ctx, who, ev.RunBefore); err != nil {
		return err
	}
	if err := u.featureEffectsBack(ctx, ev); err != nil {
		return err
	}
	for _, h := range ev.Hits {
		if err := u.castHitTargetBack(ctx, h); err != nil {
			return err
		}
	}
	if ev.Resource != "" && who.Kind == kindPlayer {
		if err := u.spendResourcesBack(ctx, who, ev.Resource, max(ev.PointsSpent, 1)); err != nil {
			return err
		}
	}
	return u.onceMarksBack(ctx, who, ev)
}

// featureUsedLine is the handler of a use of a feature: a line of the feature action's audience (the table, unless the user or
// a target is hidden: hitsVisible), whose numbers are the master's and the user's player's (fillFeatureUsed).
func featureUsedLine(_ *logBuilder, _ playdb.ListEncounterEventsRow, entry *logEntry) bool {
	entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED
	return true
}

// fillFeatureUsed is the line's body: the feature, the choice and what the use did. The hit points are the master's and the
// user's player's, as in the other lines that heal; that a lasting effect was put on is the table's.
func (e *logEntry) fillFeatureUsed(c *logViewing, out *playv1.CombatLogEntry) bool {
	f := e.ev.Feature
	if f == nil {
		return false
	}
	used := &playv1.CombatLogFeatureUsed{
		FeatureKey: e.ev.Key, NamePt: c.names.of(c.ctx, c.actor, e.ev.Key),
		ChoiceTextPt: c.names.choiceText(c.ctx, c.actor, e.ev.Key, f.Choice), EffectApplied: f.Effect,
	}
	if c.v.master || c.v.owns(c.actor) {
		used.Healed, used.TempHp = f.Healed, f.TempHP
	}
	out.FeatureUsed = used
	return true
}
