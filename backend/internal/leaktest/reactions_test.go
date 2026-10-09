package leaktest

import (
	"bytes"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The reaction window (PM-04, RN-10, RN-20): what an NPC could do in answer to a player's
// action is the master's alone until it happens. A player reads "Esperando o mestre" and
// nothing of the window, the reactor, the reaction or the numbers it holds.

// reactionFight puts a visible Mago (Shield, Counterspell) next to Toren, on Toren's turn,
// and returns the Mago's combatant. Toren carries a dagger.
func (w *world) reactionFight() *playv1.Combatant {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	sheet := w.fullSheet("class:wizard", "race:human", 1)
	sheet.GetFull().WeaponKeys = []string{"equipment:dagger"}
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(), Sheet: sheet,
	})))
	hidden := false
	must(m.combat.AddMonsters(ctx, rq(&playv1.AddMonstersRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CreatureKey: "monster:mage", Count: 1, Name: w.secrets.public("mage-name"), Hidden: &hidden,
	})))
	e := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	var mage *playv1.Combatant
	for _, c := range e.GetCombatants() {
		if c.GetBestiaryCreatureKey() == "monster:mage" {
			mage = c
		}
	}
	if mage == nil {
		w.t.Fatal("the Mago is not in the combat")
	}
	for id, sq := range map[string][2]int32{w.combatant(w.toren).GetId(): {12, 14}, mage.GetId(): {13, 14}} {
		must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Col: sq[0], Row: sq[1], Forced: true})))
	}
	for range 40 {
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == w.combatant(w.toren).GetId() {
			w.encounter = cur
			return mage
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	w.t.Fatal("Toren's turn never came")
	return nil
}

// reactionNeedles are what the players must not read before the reaction happens: the
// reaction's names and the Mago's spells, and the id of its window.
func (w *world) reactionNeedles(windowID string) {
	w.secrets.add(&canary{needle: "Escudo Arcano", kind: "reaction-name"})
	w.secrets.add(&canary{needle: "spell:shield", kind: "reaction-spell"})
	w.secrets.add(&canary{needle: "spell:counterspell", kind: "reaction-spell"})
	w.secrets.add(&canary{needle: windowID, kind: "reaction-window-id"})
}

func (w *world) reads(p *person) []reply {
	w.t.Helper()
	enc := p.call(playv1connect.CombatServiceGetEncounterProcedure, &playv1.GetEncounterRequest{CampaignId: w.campaign})
	log := p.call(playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()})
	return []reply{enc, log}
}

// A hit that an NPC's Shield could change waits for the master: the players read the same
// words as for a "Sempre" check, and no read, of the attack or of the combat, holds a word of the
// window, the reaction, the Mago's spells or the numbers of the trigger. The master's read has
// them (the positive control: the needles are reachable).
func TestAnNPCsReactionIsTheMastersAloneUntilItHappens(t *testing.T) {
	w := newWorld(t)
	mage := w.reactionFight()
	toren := w.combatant(w.toren)

	hit := w.caio.call(playv1connect.CombatServiceRollAttackProcedure, &playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: toren.GetId(), AttackKey: "equipment:dagger", TargetId: mage.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 10},
	})
	if !hit.ok() {
		t.Fatalf("Toren's attack: status %d %s", hit.status, hit.body)
	}
	enc := must(w.master.combat.GetEncounter(t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	if len(enc.GetReactionWindows()) != 1 || enc.GetReactionWindows()[0].GetKind() != playv1.ReactionKind_REACTION_KIND_SHIELD {
		t.Fatalf("the master reads %v, want the Mago's Shield window", enc.GetReactionWindows())
	}
	window := enc.GetReactionWindows()[0]
	w.reactionNeedles(window.GetId())

	check := func(when string) {
		t.Helper()
		replies := map[*person][]reply{}
		for _, p := range []*person{w.ana, w.caio} {
			replies[p] = append(w.reads(p), hit)
			if p != w.caio {
				replies[p] = replies[p][:2] // the attack's answer is Caio's own
			}
			for _, r := range replies[p] {
				for _, f := range w.inspect(p, r, nil) {
					t.Errorf("%s %s: %s", when, p.name, f)
				}
			}
		}
	}
	check("while the Mago may react")
	// The positive control: the master's own read holds the needles the players' do not.
	masterReads := w.reads(w.master)
	found := false
	for _, r := range masterReads {
		if len(r.body) > 0 && contains(r.body, window.GetId()) {
			found = true
		}
	}
	if !found {
		t.Error("the master's reads do not hold the window: the needle is unreachable")
	}
	// A player may not answer it, and asking cannot tell a real window from an invented one.
	real := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS, IdempotencyKey: newKey(),
	})
	fake := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: newKey(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS, IdempotencyKey: newKey(),
	})
	if real.ok() || fake.ok() || real.status != fake.status || string(real.body) != string(fake.body) {
		t.Errorf("a real window is refused with %d %s, an invented one with %d %s: they must read the same", real.status, real.body, fake.status, fake.body)
	}
	// The master uses Shield for the Mago.
	must(w.master.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE,
		Slot: &playv1.SpellSlot{Level: 1}, IdempotencyKey: newKey(),
	})))
	// What happened is no secret any more, but the window and its numbers still are.
	w.secrets.allowNeedle("Escudo Arcano", w.ana, w.caio)
	w.secrets.allowNeedle("spell:shield", w.ana, w.caio)
	check("after the Mago reacted")
}

func contains(body []byte, s string) bool { return bytes.Contains(body, []byte(s)) }

// An NPC's window is heard by the master alone: the players' streams get the combat's
// ordinary change events and neither of the window's two (RN-10), while the master's
// stream gets both (the positive control).
func TestAnNPCsReactionWindowIsHeardByTheMasterAlone(t *testing.T) {
	w := newWorld(t)
	mage := w.reactionFight()
	toren := w.combatant(w.toren)
	got := w.hiddenOnlyEvents(func() {
		must(w.caio.combat.RollAttack(t.Context(), rq(&playv1.RollAttackRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: toren.GetId(), AttackKey: "equipment:dagger", TargetId: mage.GetId(),
			IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 10},
		})))
		enc := must(w.master.combat.GetEncounter(t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		must(w.master.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: enc.GetReactionWindows()[0].GetId(),
			Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS, IdempotencyKey: newKey(),
		})))
	})
	count := func(name string) (opened, closed int) {
		for _, ev := range got[name] {
			switch {
			case strings.HasPrefix(ev, "reaction_window_opened"):
				opened++
			case strings.HasPrefix(ev, "reaction_window_closed"):
				closed++
			}
		}
		return opened, closed
	}
	if o, c := count(w.master.name); o != 1 || c != 1 {
		t.Errorf("the master heard %d openings and %d closings, want one of each (%v)", o, c, got[w.master.name])
	}
	for _, p := range []*person{w.ana, w.caio} {
		if o, c := count(p.name); o != 0 || c != 0 {
			t.Errorf("%s heard %d openings and %d closings of an NPC's window", p.name, o, c)
		}
	}
}
