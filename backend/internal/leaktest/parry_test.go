package leaktest

import (
	"bytes"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// A Knight's Parry (SRD 5.1, RN-20, RN-31): a melee hit on it waits for the master, who answers for the
// creature. A player reads "Esperando o mestre" and, after the answer, an ordinary miss: no window, no
// prompt, no word of the reaction, no bonus and no armor class. The master reads them (the positive controls).

// parryWords are what no player reads of a Parry, before or after it is used.
var parryWords = []string{"Parry", "parry", "PARRY", "Aparar", "aparou", "monster:knight"}

// parryFight puts a visible Knight next to Toren, on Toren's turn, and arms the needles.
func (w *world) parryFight() *playv1.Combatant {
	w.t.Helper()
	knight := w.reactionFightOf("monster:knight", w.secrets.public("knight-name"))
	for _, word := range parryWords {
		w.secrets.add(&canary{needle: word, kind: "parry"})
	}
	return knight
}

// daggerHit is Toren's dagger attack on the target as Caio: +4 to hit, so a d20 of 14 makes 18, the Knight's armor class.
func (w *world) daggerHit(target *playv1.Combatant, face int32) reply {
	w.t.Helper()
	return w.caio.call(playv1connect.CombatServiceRollAttackProcedure, &playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: w.combatant(w.toren).GetId(), AttackKey: "equipment:dagger", TargetId: target.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: face},
	})
}

func TestAParrysWindowIsTheMastersAloneAndItsUseReadsAsAMiss(t *testing.T) {
	w := newWorld(t)
	knight := w.parryFight()
	hit := w.daggerHit(knight, 14)
	if !hit.ok() {
		t.Fatalf("Toren's attack: status %d %s", hit.status, hit.body)
	}
	window := windowOfKind(w.master, w, playv1.ReactionKind_REACTION_KIND_PARRY)
	if window == nil || window.GetParry().GetAcBonus() != 2 || window.GetTrigger().GetTargetArmorClass() != 18 {
		t.Fatalf("the master reads %v, want the Knight's Parry (+2) against armor class 18", window)
	}
	w.secrets.add(&canary{needle: window.GetId(), kind: "reaction-window-id"})

	check := func(when string) {
		t.Helper()
		for _, p := range []*person{w.ana, w.caio} {
			rs := w.reads(p)
			if p == w.caio {
				rs = append(rs, hit)
			}
			for _, r := range rs {
				for _, f := range w.inspect(p, r, nil) {
					t.Errorf("%s %s: %s", when, p.name, f)
				}
			}
		}
	}
	check("while the Knight may parry")
	for _, p := range []*person{w.ana, w.caio} {
		if got := windowOfKind(p, w, playv1.ReactionKind_REACTION_KIND_PARRY); got != nil {
			t.Errorf("%s reads the Parry window %v", p.name, got)
		}
		if wait := p.currentEncounter(w).GetReactionWait().GetTitlePt(); wait != "Esperando o mestre" {
			t.Errorf("%s reads the wait %q, want \"Esperando o mestre\"", p.name, wait)
		}
	}
	// The positive control: the master's reads hold the window and the word.
	if !anyBody(w.reads(w.master), window.GetId()) || !anyBody(w.reads(w.master), "REACTION_KIND_PARRY") {
		t.Error("the master's reads do not hold the window: the needles are unreachable")
	}
	// A player cannot answer it, and asking cannot tell a real window from an invented one.
	asked := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})
	fake := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: newKey(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})
	if asked.ok() || fake.ok() || asked.status != fake.status || !bytes.Equal(asked.body, fake.body) {
		t.Errorf("a real window is refused with %d %s, an invented one with %d %s: they must read the same", asked.status, asked.body, fake.status, fake.body)
	}

	used := must(w.master.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})))
	if !used.GetResult().GetParry().GetStopped() {
		t.Fatalf("the answer = %v, want the hit stopped", used.GetResult())
	}
	check("after the Knight parried")

	// The positive controls of the log: the master reads the line and the reason; Caio, who attacked, reads a plain
	// miss (Ana stands out of the Knight's sight on the fog map and reads no line of it at all).
	masterLine, masterAttack := "", false
	for _, e := range w.logOfPerson(w.master) {
		if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
			masterLine = e.GetReactionTextPt()
		}
		masterAttack = masterAttack || (e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && e.GetStoppedByReaction())
	}
	if !strings.Contains(masterLine, "aparou (+2 CA)") || !masterAttack {
		t.Errorf("the master's log: line %q, stopped attack %v; want \"aparou (+2 CA)\" and the attack stopped by a reaction", masterLine, masterAttack)
	}
	miss := false
	for _, e := range w.logOfPerson(w.caio) {
		miss = miss || (e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && e.GetActorId() == w.combatant(w.toren).GetId() && e.GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_MISS && !e.GetStoppedByReaction())
	}
	if !miss {
		t.Error("Caio's log has no plain miss of Toren's attack: the control is missing")
	}
}

// The Knight's window is heard by the master alone: the players' streams get the combat's ordinary change events
// and neither of the window's two, with Parry used or passed (the master's stream gets both: the control).
func TestAParrysWindowIsHeardByTheMasterAlone(t *testing.T) {
	w := newWorld(t)
	knight := w.parryFight()
	got := w.hiddenOnlyEvents(func() {
		must(w.caio.combat.RollAttack(t.Context(), rq(&playv1.RollAttackRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: w.combatant(w.toren).GetId(), AttackKey: "equipment:dagger", TargetId: knight.GetId(),
			IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 14},
		})))
		enc := must(w.master.combat.GetEncounter(t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		must(w.master.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: enc.GetReactionWindows()[0].GetId(),
			Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
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
			t.Errorf("%s heard %d openings and %d closings of the Knight's window", p.name, o, c)
		}
	}
}

// anyBody says one of the replies holds the word.
func anyBody(rs []reply, word string) bool {
	for _, r := range rs {
		if contains(r.body, word) {
			return true
		}
	}
	return false
}
