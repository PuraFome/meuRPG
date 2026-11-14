package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied action (ReadyService, RN-10): the trigger the player wrote and the action held are the master's and the
// readier's player's. Another player reads that the combatant readied an action, never what or when; a hidden NPC's
// readied action is the master's alone, line and all. The master's and the owner's reads are the positive controls; the
// TV (the table's spectator) is inspected with the other players.

// readiedWords are the fields that only the master's and the owner's copies carry: the readied action of the combatant
// and the detail of its line.
var readiedWords = []string{`"triggerText"`, "READIED_KIND_ATTACK", "READIED_STATUS_WAITING"}

// readiedLinesOf counts the readied lines of a person's log whose actor is id, and those with the detail.
func readiedLinesOf(entries []*playv1.CombatLogEntry, id string) (lines, detailed int) {
	return readiedKindLinesOf(entries, id, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED)
}

// readiedKindLinesOf counts the lines of the kind in a person's log whose actor is id, and those with the detail.
func readiedKindLinesOf(entries []*playv1.CombatLogEntry, id string, kind playv1.CombatLogKind) (lines, detailed int) {
	for _, e := range entries {
		if e.GetKind() != kind || e.GetActorId() != id {
			continue
		}
		lines++
		if e.GetReadied() != nil || e.GetReadiedLost() != nil || e.GetReadiedFired() != nil {
			detailed++
		}
	}
	return lines, detailed
}

// inspectPlayers inspects every read of the people: none may hold a canary they are not a reader of.
func (w *world) inspectPlayers(t *testing.T, people ...*person) {
	t.Helper()
	for _, p := range people {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
}

// readsHold says whether any of the person's reads holds the needle.
func (w *world) readsHold(p *person, needle string) bool {
	for _, r := range w.reads(p) {
		if contains(r.body, needle) {
			return true
		}
	}
	return false
}

func TestAReadiedActionIsTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	toren := w.combatant(w.toren)
	trigger := w.secrets.marker("readied-trigger", w.caio)
	for _, n := range readiedWords {
		w.secrets.add(&canary{needle: n, kind: "readied-detail", readers: names([]*person{w.caio})})
	}
	must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: "equipment:dagger",
	})))

	for _, p := range []*person{w.master, w.caio} { // the positive controls
		if lines, detailed := readiedLinesOf(w.logOfPerson(p), toren.GetId()); lines != 1 || detailed != 1 {
			t.Errorf("%s reads %d readied lines, %d with the trigger; want 1 and 1", p.name, lines, detailed)
		}
		found := false
		for _, r := range w.reads(p) {
			found = found || contains(r.body, trigger)
		}
		if !found {
			t.Errorf("%s's reads hold no trigger: the canary is unreachable", p.name)
		}
	}
	if lines, detailed := readiedLinesOf(w.logOfPerson(w.ana), toren.GetId()); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d readied lines, %d with the trigger; want 1 and 0", lines, detailed)
	}
	for _, p := range []*person{w.ana, w.pending} {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	w.readScreen(t, "after Toren readied", w.tvReader(), nil)
}

// The master readies an action for the hidden NPC on its turn: no player reads a word of it, not even the line.
func TestAHiddenNPCsReadiedActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	hidden := w.combatant(w.hiddenNPC)
	w.onTurnOf(hidden.GetId())
	trigger := w.secrets.marker("readied-hidden-trigger")
	must(w.master.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hidden.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_OTHER, ActionKey: "standard:dodge",
	})))

	if lines, detailed := readiedLinesOf(w.logOfPerson(w.master), hidden.GetId()); lines != 1 || detailed != 1 {
		t.Errorf("the master reads %d readied lines of the hidden NPC, %d with the trigger; want 1 and 1", lines, detailed)
	}
	found := false
	for _, r := range w.reads(w.master) {
		found = found || contains(r.body, trigger)
	}
	if !found {
		t.Error("the master's reads hold no trigger: the canary is unreachable")
	}
	for _, p := range []*person{w.ana, w.caio} {
		if lines, _ := readiedLinesOf(w.logOfPerson(p), hidden.GetId()); lines != 0 {
			t.Errorf("%s reads %d readied lines of the hidden NPC, want none", p.name, lines)
		}
	}
	for _, p := range []*person{w.ana, w.caio, w.pending} { // a pending member has no log at all: the reads are the check
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	w.readScreen(t, "after the hidden NPC readied", w.tvReader(), nil)
}

// onTurnOf plays the combat until the combatant's turn.
func (w *world) onTurnOf(id string) {
	w.t.Helper()
	ctx := w.t.Context()
	for range 40 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == id {
			w.encounter = cur
			return
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	w.t.Fatal("it never became the combatant's turn")
}

// A readied action that ends: lost at the start of Toren's next turn, then readied again and given up (CancelReadied). Each
// end is a READIED_LOST line whose trigger, and the cancel's flag, are the master's and Caio's; Ana reads the line alone.
func TestALostReadiedActionIsTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	toren := w.combatant(w.toren)
	lostTrigger := w.secrets.marker("readied-lost-trigger", w.caio)
	canceledTrigger := w.secrets.marker("readied-canceled-trigger", w.caio)
	for _, n := range append([]string{`"canceled"`}, readiedWords...) {
		w.secrets.add(&canary{needle: n, kind: "readied-detail", readers: names([]*person{w.caio})})
	}
	ready := func(trigger string) *playv1.Encounter {
		return must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
			TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: "equipment:dagger",
		}))).GetEncounter()
	}
	ready(lostTrigger)
	w.comeBackTo(toren) // the loss
	readied := ready(canceledTrigger)
	must(w.caio.ready.CancelReadied(t.Context(), rq(&playv1.CancelReadiedRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: w.combatantOf(readied, w.toren.GetId()).GetReadied().GetId(), IdempotencyKey: newKey(),
	})))

	for _, p := range []*person{w.master, w.caio} { // the positive controls
		var lost, canceled int
		for _, e := range w.logOfPerson(p) {
			if d := e.GetReadiedLost(); e.GetActorId() == toren.GetId() && d != nil {
				if d.GetTriggerText() == lostTrigger && !d.GetCanceled() {
					lost++
				}
				if d.GetTriggerText() == canceledTrigger && d.GetCanceled() {
					canceled++
				}
			}
		}
		if lost != 1 || canceled != 1 {
			t.Errorf("%s reads %d lost lines and %d canceled ones with the trigger; want 1 and 1", p.name, lost, canceled)
		}
		if !w.readsHold(p, lostTrigger) || !w.readsHold(p, `"canceled"`) {
			t.Errorf("%s's reads hold no lost trigger or no cancel: the canaries are unreachable", p.name)
		}
	}
	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.ana), toren.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST); lines != 2 || detailed != 0 {
		t.Errorf("Ana reads %d lost lines, %d with the detail; want 2 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren's readied actions ended", w.tvReader(), nil)
}

// The hidden NPC's readied action is lost at the start of its next turn: the lost line is the master's alone too.
func TestAHiddenNPCsLostReadiedActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	hidden := w.combatant(w.hiddenNPC)
	w.onTurnOf(hidden.GetId())
	trigger := w.secrets.marker("readied-hidden-lost-trigger")
	must(w.master.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hidden.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_OTHER, ActionKey: "standard:dodge",
	})))
	w.comeBackTo(hidden)

	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.master), hidden.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST); lines != 1 || detailed != 1 {
		t.Errorf("the master reads %d lost lines of the hidden NPC, %d with the trigger; want 1 and 1", lines, detailed)
	}
	if !w.readsHold(w.master, trigger) {
		t.Error("the master's reads hold no trigger: the canary is unreachable")
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, kind := range []playv1.CombatLogKind{playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST} {
			if lines, _ := readiedKindLinesOf(w.logOfPerson(p), hidden.GetId(), kind); lines != 0 {
				t.Errorf("%s reads %d %v lines of the hidden NPC, want none", p.name, lines, kind)
			}
		}
	}
	w.inspectPlayers(t, w.ana, w.caio, w.pending)
	w.readScreen(t, "after the hidden NPC's readied action was lost", w.tvReader(), nil)
}

// The held action's name is a canary too: the NPC in Ana's plain sight gets an attack whose name is the master's, and
// readies it. The master reads the name on the combatant and on the line (the positive controls); Ana reads the line and
// not the name, and no player reads the name anywhere.
func TestAReadiedActionsNameIsTheMasters(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	name := w.secrets.marker("readied-attack-name")
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
	got := must(w.master.ready.ReadyAction(ctx, rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: seen.GetId(), IdempotencyKey: newKey(),
		TriggerText: w.secrets.marker("readied-seen-trigger"), Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: attacks[0].GetAttack().GetKey(),
	}))).GetEncounter()

	if n := w.combatantOf(got, w.seenNPC.GetId()).GetReadied().GetActionNamePt(); n != name { // the positive controls
		t.Errorf("the master reads the NPC's readied action named %q, want %q", n, name)
	}
	var line string
	for _, e := range w.logOfPerson(w.master) {
		if e.GetActorId() == seen.GetId() && e.GetReadied() != nil {
			line = e.GetReadied().GetActionNamePt()
		}
	}
	if line != name {
		t.Errorf("the master's readied line of the NPC names %q, want %q", line, name)
	}
	if lines, detailed := readiedLinesOf(w.logOfPerson(w.ana), seen.GetId()); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d readied lines of the NPC, %d with the detail; want 1 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.caio, w.pending)
	w.readScreen(t, "after the NPC in sight readied", w.tvReader(), nil)
}
