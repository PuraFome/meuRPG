package leaktest

import (
	"bytes"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A monk's Slow Fall (SRD 5.1, Monk 4): Toren falls into a pit, and the question of the feature is his and
// the master's. The other player reads no window, no word of the feature and no number of the fall; the
// master reads them all (the positive controls). The fall of 37 is what the pit rolls (a flat damage) and
// 20 is what Slow Fall takes off at level 4.

// slowFallFall makes Toren a monk of level 4, digs a pit on the fog map and has the master fire it on him.
func (w *world) slowFallFall() {
	w.t.Helper()
	ctx := w.t.Context()
	toren := w.toren
	sheet := w.fullSheet("class:monk", "race:human", 4)
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: toren.GetId(), Revision: toren.GetRevision(), Name: toren.GetName(), Sheet: sheet,
	})))
	pit := w.point(w.fogMap, mapsv1.MapPointKind_MAP_POINT_KIND_TRAP, "Fosso do teste", "um fosso fundo", 10, 14, func(r *mapsv1.CreateMapPointRequest) {
		r.Trap = &mapsv1.TrapSpec{
			NoticeDc: trapNotice, FindDc: trapFind, AreaSize: 1, Trigger: rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER, PresetKey: "trap:simple-pit",
			Effect: &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{{Dice: "37", DamageTypeKey: "damage-type:bludgeoning"}}},
		}
	})
	must(w.master.play.FireTrap(ctx, rq(&playv1.FireTrapRequest{
		CampaignId: w.campaign, MapId: w.fogMap, PointId: pit.GetId(), TargetIds: []string{w.combatant(w.toren).GetId()}, IdempotencyKey: newKey(),
	})))
}

// slowFallNeedles are what no other player reads: the window, the kind, and the numbers of the line.
func (w *world) slowFallNeedles(windowID string) {
	w.secrets.add(&canary{needle: windowID, kind: "reaction-window-id", readers: names([]*person{w.caio})})
	w.secrets.add(&canary{needle: "REACTION_KIND_SLOW_FALL", kind: "slow-fall-kind", readers: names([]*person{w.caio})})
	w.secrets.add(&canary{needle: `"slowFall"`, kind: "slow-fall-prompt", readers: names([]*person{w.caio})})
	w.secrets.add(&canary{needle: "no dano da queda", kind: "slow-fall-numbers", readers: names([]*person{w.caio})})
	w.secrets.add(&canary{needle: "de 37 para 17", kind: "slow-fall-damage"})
	// The close message: only the reactor and the master read that Toren used the feature.
	w.secrets.add(&canary{needle: "usou Queda Lenta: -20", kind: "slow-fall-answer", readers: names([]*person{w.caio})})
}

func TestASlowFallsWindowIsTheMonksAndTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	w.slowFallFall()
	window := windowOfKind(w.caio, w, playv1.ReactionKind_REACTION_KIND_SLOW_FALL)
	if window == nil || window.GetSlowFall().GetReduction() != 20 {
		t.Fatalf("Caio reads %v, want Toren's Slow Fall window (20 points)", window)
	}
	first := len(w.secrets.list)
	w.slowFallNeedles(window.GetId())

	got := newAnswers()
	check := func(when string) {
		t.Helper()
		// Ana reads none of it, and Caio, the monk, reads all but the damage around the cut (the master's).
		for _, p := range []*person{w.ana, w.caio} {
			for _, r := range w.reads(p) {
				got.keep(p, r)
				for _, f := range w.inspect(p, r, nil) {
					t.Errorf("%s %s: %s", when, p.name, f)
				}
			}
		}
		for _, r := range w.reads(w.master) {
			got.keep(w.master, r)
		}
	}
	check("while Toren may use Slow Fall")
	if got := windowOfKind(w.ana, w, playv1.ReactionKind_REACTION_KIND_SLOW_FALL); got != nil {
		t.Errorf("Ana reads the window %v", got)
	}
	if wait := w.ana.currentEncounter(w).GetReactionWait().GetTitlePt(); wait != "Esperando o mestre" && !strings.HasPrefix(wait, "Esperando a reação de Toren") {
		t.Errorf("Ana reads the wait %q, want \"Esperando o mestre\" or Toren's name", wait)
	}
	// The positive control: the master's reads hold the window, the kind and the prompt.
	for _, word := range []string{window.GetId(), "REACTION_KIND_SLOW_FALL", `"slowFall"`} {
		if !anyBody(w.reads(w.master), word) {
			t.Errorf("the master's reads do not hold %q: the needle is unreachable", word)
		}
	}
	// Ana cannot answer it, and asking cannot tell a real window from an invented one.
	asked := w.ana.call("/meurpg.play.v1.CombatService/AnswerReaction", &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})
	fake := w.ana.call("/meurpg.play.v1.CombatService/AnswerReaction", &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: newKey(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})
	if asked.ok() || fake.ok() || asked.status != fake.status || !bytes.Equal(asked.body, fake.body) {
		t.Errorf("a real window is refused with %d %s, an invented one with %d %s: they must read the same", asked.status, asked.body, fake.status, fake.body)
	}

	used := must(w.caio.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})))
	if used.GetResult().GetSlowFall().GetReduction() != 20 || used.GetResult().GetSlowFall().DamageBefore != nil {
		t.Fatalf("Caio's result = %v, want 20 points off and no damage numbers", used.GetResult())
	}
	check("after Toren used Slow Fall")

	// The positive controls of the log: the master reads the fall around the cut, Caio the points, and
	// the line of Ana (when she gets one) has neither.
	line := func(p *person) string {
		out := ""
		for _, e := range w.logOfPerson(p) {
			if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
				out = e.GetReactionTextPt()
			}
		}
		return out
	}
	if got := line(w.master); !strings.Contains(got, "-20 no dano da queda (de 37 para 17)") {
		t.Errorf("the master's line = %q, want the cut and the damage around it", got)
	}
	if got := line(w.caio); !strings.HasSuffix(got, "-20 no dano da queda.") {
		t.Errorf("Caio's line = %q, want the points taken off", got)
	}
	if got := line(w.ana); strings.Contains(got, "20") || strings.Contains(got, "37") || got != "Toren usou Queda Lenta." {
		t.Errorf("Ana's line = %q, want the use without a number", got)
	}
	// Only the needles of this table are checked here (the matrix reads the rest of the world's).
	w.secrets.list = w.secrets.list[first:]
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, got) })
}

// Toren's window is heard by Toren's player and the master alone.
func TestASlowFallsWindowIsHeardByItsMonkAndTheMasterAlone(t *testing.T) {
	w := newWorld(t)
	got := w.hiddenOnlyEvents(w.slowFallFall)
	opened := func(name string) int {
		n := 0
		for _, ev := range got[name] {
			if strings.HasPrefix(ev, "reaction_window_opened") {
				n++
			}
		}
		return n
	}
	if opened(w.master.name) != 1 || opened(w.caio.name) != 1 || opened(w.ana.name) != 0 {
		t.Errorf("openings heard: master %d, Caio %d, Ana %d; want 1, 1, 0 (%v)", opened(w.master.name), opened(w.caio.name), opened(w.ana.name), got)
	}
}
