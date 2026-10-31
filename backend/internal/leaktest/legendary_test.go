package leaktest

import (
	"bytes"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// An adult red dragon in Ana's sight: the master reads its legendary state (3 actions, 3 uses
// of Legendary Resistance, 3 options), and no player reads the field in the combat, the log or
// anywhere the matrix looks (RN-20, RN-29). The matrix flags the field as a master-only field
// in every player's answer; this test is the positive control that the field is populated.
func TestTheDragonsLegendaryStateIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	enc := w.master.currentEncounter(w)
	var dragon *playv1.Combatant
	for _, c := range enc.GetCombatants() {
		if c.GetBestiaryCreatureKey() == dragonKey {
			dragon = c
		}
	}
	l := dragon.GetLegendary()
	if l == nil || l.GetActionsPerRound() != 3 || l.GetActionsLeft() != 3 || l.GetResistancePerDay() != 3 || l.GetResistanceLeft() != 3 || len(l.GetOptions()) != 3 {
		t.Fatalf("the master reads the dragon's legendary state as %v, want 3/3 actions, 3/3 resistance and 3 options", l)
	}
	for _, p := range []*person{w.ana, w.caio} {
		seen := false
		mustSee := p == w.ana // the dragon stands in Ana's sight, not in Caio's
		for _, c := range p.currentEncounter(w).GetCombatants() {
			if c.GetId() != dragon.GetId() {
				continue
			}
			seen = true
			if c.GetLegendary() != nil {
				t.Errorf("%s reads the dragon's legendary state %v", p.name, c.GetLegendary())
			}
		}
		if mustSee && !seen {
			t.Errorf("%s does not read the dragon, which is in plain sight: the control is missing", p.name)
		}
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
}

// legendaryWords are what no player reads of a legendary spend: the option names (an attack, a text option),
// their keys and the word itself. The master's answers hold them (the positive control).
var legendaryWords = []string{"Wing Attack", "wing-attack", "Tail Attack", "tail-attack", "legendary"}

// offTurn passes the turns until the combatant is not the one on turn: it may spend its legendary actions.
func (w *world) offTurn(c *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	for range 40 {
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() != c.GetId() {
			w.encounter = cur
			return
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(), DiscardPendingDamage: true})))
	}
	w.t.Fatalf("the turn of %s never passed", c.GetLabel())
}

// The master spends the dragon's legendary actions at the end of another's turn: on a text option (Wing
// Attack, 2 actions) and on an attack (Tail Attack, through RollAttack). No player reads the option, the
// cost, the count or the word in the combat, the log or the answers they hold; the attack is the ordinary
// line of the dragon's tail on Toren. The master reads them (the positive control), and a player's own try
// to spend is refused.
func TestALegendarySpendIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	for _, word := range legendaryWords {
		w.secrets.add(&canary{needle: word, kind: "legendary-spend"})
	}
	var dragon *playv1.Combatant
	for _, c := range w.master.currentEncounter(w).GetCombatants() {
		if c.GetBestiaryCreatureKey() == dragonKey {
			dragon = c
		}
	}
	w.offTurn(dragon)

	text := w.master.call(playv1connect.CombatServiceSpendLegendaryActionProcedure, &playv1.SpendLegendaryActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: dragon.GetId(), OptionKey: "wing-attack", IdempotencyKey: newKey(),
	})
	if !text.ok() {
		t.Fatalf("the text spend: status %d %s", text.status, text.body)
	}
	strike := w.master.call(playv1connect.CombatServiceRollAttackProcedure, &playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: dragon.GetId(), AttackKey: "basic:2", TargetId: w.combatant(w.toren).GetId(),
		IdempotencyKey: newKey(), D20Faces: []int32{20, 20}, LegendaryOption: "tail-attack",
	})
	if !strike.ok() {
		t.Fatalf("the legendary attack: status %d %s", strike.status, strike.body)
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.readsOf(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
		refused := p.call(playv1connect.CombatServiceSpendLegendaryActionProcedure, &playv1.SpendLegendaryActionRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: dragon.GetId(), OptionKey: "detect", IdempotencyKey: newKey(),
		})
		if refused.ok() {
			t.Errorf("%s spent the dragon's legendary action", p.name)
		}
	}
	// The positive controls: the master's answers and log hold each needle, and Ana reads the attack as a line.
	master := append(w.reads(w.master), text, strike)
	for _, word := range []string{"Wing Attack", "wing-attack", "legendary"} {
		found := false
		for _, r := range master {
			found = found || bytes.Contains(r.body, []byte(word))
		}
		if !found {
			t.Errorf("the master's answers do not hold %q: the needle is unreachable", word)
		}
	}
	var state *playv1.LegendaryState
	for _, c := range w.master.currentEncounter(w).GetCombatants() {
		if c.GetId() == dragon.GetId() {
			state = c.GetLegendary()
		}
	}
	if state.GetActionsLeft() != 0 {
		t.Errorf("the master reads the dragon with %v, want 0 actions left (2 and 1 spent)", state)
	}
	sawAttack := false
	for _, e := range w.logOfPerson(w.ana) {
		sawAttack = sawAttack || (e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && e.GetActorId() == dragon.GetId())
	}
	if !sawAttack {
		t.Error("Ana's log has no attack line of the dragon: the control is missing")
	}
}

// resistanceWords are what no player reads of a Legendary Resistance: its name in both languages and the key.
// The master's answers hold the English ones (the positive control).
var resistanceWords = []string{"Legendary Resistance", "legendary-resistance", "Resistência Lendária"}

// onDragonsTurn passes the turns until the dragon is on turn.
func (w *world) onDragonsTurn(c *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	for range 40 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == c.GetId() {
			w.encounter = cur
			return
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(), DiscardPendingDamage: true})))
	}
	w.t.Fatalf("the turn of %s never came", c.GetLabel())
}

// Hold Person holds the dragon and its turn ends, which asks the master its Wisdom save. The master rolls a
// 1 and turns the failure into a success with Legendary Resistance. No player reads the trait, the failed
// roll, the count or the word in the combat, the log or the answers they hold: the line they read is a save
// that passed. The master reads them (the positive controls), and a player's own try to use the trait is
// refused.
func TestALegendaryResistanceIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	for _, word := range resistanceWords {
		w.secrets.add(&canary{needle: word, kind: "legendary-resistance"})
	}
	var dragon *playv1.Combatant
	for _, c := range w.master.currentEncounter(w).GetCombatants() {
		if c.GetBestiaryCreatureKey() == dragonKey {
			dragon = c
		}
	}
	ctx := t.Context()
	visible := true
	must(w.master.lasting.AddLastingEffect(ctx, rq(&playv1.AddLastingEffectRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:hold-person",
		TargetIds: []string{dragon.GetId()}, CasterId: w.combatant(w.toren).GetId(), PlayerVisible: &visible,
		Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 10, AnchorCombatantId: dragon.GetId()},
	})))
	w.onDragonsTurn(dragon)
	must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: dragon.GetId(), DiscardPendingDamage: true,
	})))
	enc := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	var window string
	for _, rw := range enc.GetReactionWindows() {
		if rw.GetKind() == playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE {
			window = rw.GetId()
		}
	}
	if window == "" {
		t.Fatal("the dragon's save is not asked")
	}
	save := func(p *person) reply {
		return p.call(playv1connect.LastingEffectServiceRollEffectSaveProcedure, &playv1.RollEffectSaveRequest{
			CampaignId: w.campaign, EncounterId: enc.GetId(), WindowId: window, IdempotencyKey: newKey(),
			Roll: &playv1.RollEffectSaveRequest_D20Face{D20Face: 1}, UseLegendaryResistance: true,
		})
	}
	for _, p := range []*person{w.ana, w.caio} {
		if refused := save(p); refused.ok() {
			t.Errorf("%s used the dragon's Legendary Resistance", p.name)
		}
	}
	roll := save(w.master)
	if !roll.ok() {
		t.Fatalf("the master's save: status %d %s", roll.status, roll.body)
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	// The positive controls: the master's answer and log hold the choice and the word, and Ana, who sees the
	// dragon, reads the ordinary line of a save that passed.
	master := append(w.reads(w.master), roll)
	for _, word := range []string{"resisted", "resistedLeft"} {
		found := false
		for _, r := range master {
			found = found || bytes.Contains(r.body, []byte(word))
		}
		if !found {
			t.Errorf("the master's answers do not hold %q: the control is missing", word)
		}
	}
	passed := false
	for _, e := range w.logOfPerson(w.ana) {
		fx := e.GetEffect()
		passed = passed || (fx.GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVED && fx.GetD20() == 0 && !fx.GetResisted())
	}
	if !passed {
		t.Error("Ana's log has no line of a save that passed: the control is missing")
	}
}
