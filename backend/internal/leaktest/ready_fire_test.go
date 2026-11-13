package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A fired readied action (FireReadied, the READIED window, RN-10): the window, its trigger and the held action are the
// reactor's and the master's. Another player reads only the wait ("Esperando a reação de Toren", "Esperando o mestre") and,
// once answered, a readied_fired line with the actor alone. A hidden NPC's window and line are the master's alone. The
// master's and the reactor's reads are the positive controls.

// firedWords are the needles of a READIED window and of the detail of its line.
var firedWords = []string{"REACTION_KIND_READIED", "READIED_STATUS_FIRED", "READIED_STATUS_IGNORED"}

// readyDodge readies the combatant's Dodge with the trigger, as p, ends the turn and answers the row.
func (w *world) readyDodge(p *person, c *playv1.Combatant, trigger string) string {
	w.t.Helper()
	got := must(p.ready.ReadyAction(w.t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_OTHER, ActionKey: "standard:dodge",
	}))).GetEncounter()
	w.endTurnOf(c)
	return w.combatantOf(got, c.GetCharacterId()).GetReadied().GetId()
}

// endTurnOf ends the turn of the combatant on turn, as the master.
func (w *world) endTurnOf(c *playv1.Combatant) {
	w.t.Helper()
	cur := w.master.currentEncounter(w)
	if cur.GetCurrentCombatantId() != c.GetId() {
		w.t.Fatalf("%s is not on turn", c.GetLabel())
	}
	w.encounter = must(w.master.combat.EndTurn(w.t.Context(), rq(&playv1.EndTurnRequest{
		CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(),
	}))).GetEncounter()
}

// fireAndAnswer fires the row as the master, calls whileOpen with the window as the master reads it, and answers it as p:
// "Usar" or "Deixar passar".
func (w *world) fireAndAnswer(p *person, rowID string, use bool, whileOpen func(*playv1.ReactionWindow)) {
	w.t.Helper()
	ctx := w.t.Context()
	must(w.master.ready.FireReadied(ctx, rq(&playv1.FireReadiedRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: rowID, IdempotencyKey: newKey()})))
	windows := w.master.currentEncounter(w).GetReactionWindows()
	if len(windows) != 1 || windows[0].GetKind() != playv1.ReactionKind_REACTION_KIND_READIED {
		w.t.Fatalf("the master's windows = %v, want the READIED one", windows)
	}
	whileOpen(windows[0])
	answer := playv1.ReactionChoice_REACTION_CHOICE_PASS
	if use {
		answer = playv1.ReactionChoice_REACTION_CHOICE_USE
	}
	must(p.combat.AnswerReaction(ctx, rq(&playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: windows[0].GetId(), Answer: answer, IdempotencyKey: newKey(),
	})))
}

func TestAFiredReadiedActionIsTheReactorsAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	toren := w.combatant(w.toren)
	ignored := w.secrets.marker("readied-fired-trigger", w.caio)
	fired := w.secrets.marker("readied-fired-trigger", w.caio)
	for _, n := range append(append([]string{}, firedWords...), readiedWords[0]) {
		w.secrets.add(&canary{needle: n, kind: "readied-fired-detail", readers: names([]*person{w.caio})})
	}
	whileOpen := func(rw *playv1.ReactionWindow) {
		w.secrets.add(&canary{needle: rw.GetId(), kind: "readied-window-id", readers: names([]*person{w.caio})})
		if mine := w.caio.currentEncounter(w).GetReactionWindows(); len(mine) != 1 || !mine[0].GetForYou() { // the positive control
			t.Errorf("Caio's windows = %v, want Toren's, for him", mine)
		}
		ana := w.ana.currentEncounter(w)
		if len(ana.GetReactionWindows()) != 0 || ana.GetReactionWait().GetTitlePt() != "Esperando a reação de Toren" {
			t.Errorf("Ana reads the windows %v and the wait %q", ana.GetReactionWindows(), ana.GetReactionWait().GetTitlePt())
		}
		w.inspectPlayers(t, w.ana, w.pending)
		w.readScreen(t, "while Toren's READIED window waits", w.tvReader(), nil)
	}
	w.fireAndAnswer(w.caio, w.readyDodge(w.caio, toren, ignored), false, whileOpen)
	w.comeBackTo(toren)
	w.fireAndAnswer(w.caio, w.readyDodge(w.caio, toren, fired), true, whileOpen)

	for _, p := range []*person{w.master, w.caio} { // the positive controls
		got := map[string]playv1.ReadiedStatus{}
		for _, e := range w.logOfPerson(p) {
			if d := e.GetReadiedFired(); e.GetActorId() == toren.GetId() && d != nil {
				got[d.GetTriggerText()] = d.GetStatus()
			}
		}
		if len(got) != 2 || got[ignored] != playv1.ReadiedStatus_READIED_STATUS_IGNORED || got[fired] != playv1.ReadiedStatus_READIED_STATUS_FIRED {
			t.Errorf("%s reads the fired lines %v, want one ignored and one fired", p.name, got)
		}
		if !w.readsHold(p, fired) || !w.readsHold(p, "READIED_STATUS_FIRED") {
			t.Errorf("%s's reads hold no trigger or no status: the canaries are unreachable", p.name)
		}
	}
	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.ana), toren.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 2 || detailed != 0 {
		t.Errorf("Ana reads %d fired lines, %d with the detail; want 2 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren's readied actions were answered", w.tvReader(), nil)
}

// The NPC in Ana's sight readies its attack, named by the master, and the master lets it pass: the window, the name and
// the trigger are the master's; Ana reads that the master is awaited, then the line alone.
func TestAnNPCsFiredReadiedActionIsTheMasters(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	name := w.secrets.marker("readied-fired-attack-name")
	ch := must(w.master.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.seenNPC.GetId()}))).GetCharacter()
	sheet := ch.GetSheet()
	sheet.GetBasic().Attacks = []*charactersv1.BasicAttack{{Name: name, AttackBonus: 3, DamageDiceCount: 1, DamageDiceSides: 6, DamageType: charactersv1.DamageType_DAMAGE_TYPE_SLASHING}}
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: ch.GetId(), Revision: ch.GetRevision(), Name: ch.GetName(), Sheet: sheet,
	})))
	seen := w.combatant(w.seenNPC)
	w.onTurnOf(seen.GetId())
	attacks := must(w.master.combat.GetTurnOptions(ctx, rq(&playv1.GetTurnOptionsRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: seen.GetId(),
	}))).GetOptions().GetAttacks()
	if len(attacks) == 0 || attacks[0].GetAttack().GetNamePt() != name {
		t.Fatalf("the NPC's attacks = %v, want the one named %q", attacks, name)
	}
	trigger := w.secrets.marker("readied-fired-seen-trigger")
	got := must(w.master.ready.ReadyAction(ctx, rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: seen.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: attacks[0].GetAttack().GetKey(),
	}))).GetEncounter()
	w.endTurnOf(seen)
	for _, n := range firedWords {
		w.secrets.add(&canary{needle: n, kind: "readied-fired-detail"})
	}
	whileOpen := func(rw *playv1.ReactionWindow) {
		w.secrets.add(&canary{needle: rw.GetId(), kind: "readied-window-id"})
		if rw.GetTrigger().GetActionNamePt() != name { // the positive control
			t.Errorf("the master's trigger names %q, want %q", rw.GetTrigger().GetActionNamePt(), name)
		}
		if got := w.ana.currentEncounter(w).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
			t.Errorf("Ana's wait = %q", got)
		}
		w.inspectPlayers(t, w.ana, w.caio, w.pending)
		w.readScreen(t, "while the NPC's READIED window waits", w.tvReader(), nil)
	}
	w.fireAndAnswer(w.master, w.combatantOf(got, w.seenNPC.GetId()).GetReadied().GetId(), false, whileOpen)

	var line string
	for _, e := range w.logOfPerson(w.master) {
		if e.GetActorId() == seen.GetId() && e.GetReadiedFired() != nil {
			line = e.GetReadiedFired().GetActionNamePt()
		}
	}
	if line != name { // the positive control
		t.Errorf("the master's fired line names %q, want %q", line, name)
	}
	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.ana), seen.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d fired lines of the NPC, %d with the detail; want 1 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.caio, w.pending)
	w.readScreen(t, "after the NPC's readied action passed", w.tvReader(), nil)
}

// The hidden NPC's readied Dodge, fired and used: no player reads the window or the line.
func TestAHiddenNPCsFiredReadiedActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	hidden := w.combatant(w.hiddenNPC)
	w.onTurnOf(hidden.GetId())
	trigger := w.secrets.marker("readied-fired-hidden-trigger")
	for _, n := range firedWords {
		w.secrets.add(&canary{needle: n, kind: "readied-fired-detail"})
	}
	whileOpen := func(rw *playv1.ReactionWindow) {
		w.secrets.add(&canary{needle: rw.GetId(), kind: "readied-window-id"})
		for _, p := range []*person{w.ana, w.caio} {
			if got := p.currentEncounter(w).GetReactionWindows(); len(got) != 0 {
				t.Errorf("%s reads the windows %v", p.name, got)
			}
		}
		w.inspectPlayers(t, w.ana, w.caio, w.pending)
		w.readScreen(t, "while the hidden NPC's READIED window waits", w.tvReader(), nil)
	}
	w.fireAndAnswer(w.master, w.readyDodge(w.master, hidden, trigger), true, whileOpen)

	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.master), hidden.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 1 || detailed != 1 {
		t.Errorf("the master reads %d fired lines of the hidden NPC, %d with the detail; want 1 and 1", lines, detailed)
	}
	if !w.readsHold(w.master, trigger) { // the positive control
		t.Error("the master's reads hold no trigger: the canary is unreachable")
	}
	for _, p := range []*person{w.ana, w.caio} {
		if lines, _ := readiedKindLinesOf(w.logOfPerson(p), hidden.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 0 {
			t.Errorf("%s reads %d fired lines of the hidden NPC, want none", p.name, lines)
		}
	}
	w.inspectPlayers(t, w.ana, w.caio, w.pending)
	w.readScreen(t, "after the hidden NPC's readied action was used", w.tvReader(), nil)
}
