package leaktest

import (
	"bytes"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// Effects that last (RN-22, RN-10): what the master hides from the players reaches them through
// no channel: not in the combat, the log, the order's labels, the turn options, the rolls' sources
// or the list of the effects on the characters.

// putHiddenEffect puts a Hold Person the players may not read on the combatant, with a label that
// is a canary.
func (w *world) putHiddenEffect(target *playv1.Combatant) (label string) {
	w.t.Helper()
	label = w.secrets.public("effect-label")
	w.secrets.add(&canary{needle: label, kind: "effect-label"})
	hidden := false
	must(w.master.lasting.AddLastingEffect(w.t.Context(), rq(&playv1.AddLastingEffectRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:hold-person",
		TargetIds: []string{target.GetId()}, CasterId: w.combatant(w.toren).GetId(),
		Duration:      &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 10, AnchorCombatantId: w.combatant(w.toren).GetId()},
		PlayerVisible: &hidden, PlayerLabel: label,
	})))
	return label
}

func TestAnEffectTheMasterHidesReachesNoPlayer(t *testing.T) {
	w := newWorld(t)
	mage := w.reactionFight()
	label := w.putHiddenEffect(mage)

	reads := func(p *person) []reply {
		out := w.reads(p)
		return append(out,
			p.call(playv1connect.LastingEffectServiceListCharacterEffectsProcedure, &playv1.ListCharacterEffectsRequest{CampaignId: w.campaign}),
			p.call(playv1connect.CombatServiceGetTurnOptionsProcedure, &playv1.GetTurnOptionsRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: w.combatant(w.toren).GetId(),
			}))
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	// The positive control: the master's own panel holds the label.
	panel := w.master.call(playv1connect.LastingEffectServiceListLastingEffectsProcedure, &playv1.ListLastingEffectsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()})
	if !bytes.Contains(panel.body, []byte(label)) {
		t.Error("the master's panel does not hold the label: the needle is unreachable")
	}
}
