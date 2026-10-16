package play

import (
	"log/slog"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// effectLogView, frozen as it is: the effect part of a log line for each kind of viewer. No
// database is needed, the line is a function of the event, the combatants and the names.
func TestEffectLogViewFrozen(t *testing.T) {
	t.Parallel()
	owner, other := "user-owner", "user-other"
	byID := map[string]playdb.Combatant{
		"goblin": {ID: "goblin", Label: "Goblin"},
		"hidden": {ID: "hidden", Label: "Sombra", Hidden: true},
		"mine":   {ID: "mine", Label: "Pensantus", UserID: &owner},
		"myhide": {ID: "myhide", Label: "Furtivo", UserID: &owner, Hidden: true},
	}
	master := combatViewer{master: true}
	player := combatViewer{userID: other}
	ownerV := combatViewer{userID: owner}
	svc := &Service{roster: logRoster{}, logger: slog.New(slog.DiscardHandler), conditionNames: map[string]string{}}

	full := func() *lastingEvent {
		return &lastingEvent{
			Key: "effect:bless", Change: "failed", Targets: []string{"goblin", "hidden", "gone", "mine", "myhide"},
			Reason: "why", Ability: "dex", D20: 7, Total: 11, DC: 14, Amount: 5, DType: "fire",
		}
	}
	// withLucky is the same event after Lucky rolled the d20 again.
	withLucky := func(l *lastingEvent) *logEntry {
		return &logEntry{ev: actionEvent{Lasting: l, Lucky: &luckyReroll{Index: 0, From: 1, To: 7}}}
	}
	plain := func(l *lastingEvent) *logEntry { return &logEntry{ev: actionEvent{Lasting: l}} }
	d20, total, dc := int32(7), int32(11), int32(14)
	reroll := []*playv1.Reroll{{Index: 0, From: 1, To: 7, SourceKey: "trait:lucky"}}
	labels := func(l ...string) []string { return l }

	for _, c := range []struct {
		name string
		e    *logEntry
		kind playv1.CombatLogKind
		v    combatViewer
		want *playv1.CombatLogEffect // nil: the line is left as it was
	}{
		{name: "no effect in the event", e: &logEntry{}, kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: master},
		{
			name: "the master reads everything", e: withLucky(full()), kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: master,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_FAILED,
				Ability: "dex", AbilityNamePt: "Destreza", Amount: 5, DamageTypePt: "fogo", Reason: "why",
				TargetLabels: labels("Goblin", "Sombra", "Pensantus", "Furtivo"), D20: &d20, Total: &total, Dc: &dc, Rerolled: reroll,
			},
		},
		{
			name: "the owner reads the roll, not the DC, and not a hidden combatant", e: withLucky(full()), kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: ownerV,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_FAILED,
				Ability: "dex", AbilityNamePt: "Destreza", Amount: 5, DamageTypePt: "fogo", Reason: "why",
				TargetLabels: labels("Goblin", "Pensantus"), D20: &d20, Total: &total, Rerolled: reroll,
			},
		},
		{
			name: "another player reads neither the roll nor the DC", e: withLucky(full()), kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: player,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_FAILED,
				Ability: "dex", AbilityNamePt: "Destreza", Amount: 5, DamageTypePt: "fogo", Reason: "why",
				TargetLabels: labels("Goblin", "Pensantus"),
			},
		},
		{
			name: "an owner of a target with no d20 reads no roll", e: plain(&lastingEvent{Key: "effect:bless", Change: "added", Targets: []string{"mine"}, DC: 12}),
			kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: ownerV,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ADDED,
				TargetLabels: labels("Pensantus"),
			},
		},
		{
			name: "the master, no d20 and no DC, a key and a change nobody names", e: plain(&lastingEvent{Key: "mystery", Change: "unknown"}),
			kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: master,
			want: &playv1.CombatLogEffect{SourceKey: "mystery"},
		},
		{
			name: "the master, a roll without a DC", e: plain(&lastingEvent{Key: "effect:bless", Change: "saved", D20: 7, Total: 11}),
			kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: master,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVED, D20: &d20, Total: &total,
			},
		},
		{
			name: "the master, a DC without a roll", e: plain(&lastingEvent{Key: "effect:bless", Change: "damage", DC: 14, DType: "fire"}),
			kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT, v: master,
			want: &playv1.CombatLogEffect{
				SourceKey: "effect:bless", SourceNamePt: "Bênção", Change: playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_DAMAGE, DamageTypePt: "fogo", Dc: &dc,
			},
		},
		{
			// The exhaustion line is not filtered for the viewer: it names every target, hidden or
			// not, whoever reads it (the line itself is owner-only upstream, see effectLineVisible).
			name: "an exhaustion line names every target, an unknown one empty", e: plain(&lastingEvent{
				Key: "condition:exhaustion", Change: "added", Targets: []string{"goblin", "hidden", "gone"}, Level: 2, Before: 1, Reason: "r", D20: 5, DC: 9, Ability: "dex",
			}),
			kind: playv1.CombatLogKind_COMBAT_LOG_KIND_EXHAUSTION, v: player,
			want: &playv1.CombatLogEffect{SourceKey: "condition:exhaustion", ExhaustionLevel: 2, ExhaustionBefore: 1, Reason: "r", TargetLabels: labels("Goblin", "Sombra", "")},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			out := &playv1.CombatLogEntry{Kind: c.kind}
			svc.effectLogView(t.Context(), c.e, out, c.v, byID, "camp1")
			if !proto.Equal(out.GetEffect(), c.want) {
				t.Errorf("effect =\n%s\nwant\n%s", protojson.Format(out.GetEffect()), protojson.Format(c.want))
			}
			if rest := (&playv1.CombatLogEntry{Kind: c.kind, Effect: out.GetEffect()}); !proto.Equal(out, rest) {
				t.Errorf("the line changed beyond its effect: %s", protojson.Format(out))
			}
		})
	}
}
