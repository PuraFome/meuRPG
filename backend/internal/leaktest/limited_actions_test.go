package leaktest

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// What a creature of a stat block has spent of its limited actions (a Recharge action, an action with uses a day,
// one that recharges after a rest) is the master's, and the player's who controls the creature (RN-20, RN-29): the
// state, the uses left, the d6 the server rolls at the start of the creature's turn and the line of that roll.
// Every other player reads the ordinary action and nothing of the limit, in the combat, the log, the stream's hints
// and the revision. These tests use the hell hound (Recharge 5-6) and the deva (3/day) of the fixture's combat,
// which no player controls; limited_controller_test.go has the creature of a player.

// limitedWords are what no player reads of a creature's limits: the field names, the SRD's words of the limit
// and the keys of the spent state. The master's answers hold the field names (the positive control).
var limitedWords = []string{"limitedActions", "usesLeft", "usesAllowed", "Recharge 5-6", "3/day", "recharge:", "use:", "rechargeRolled", "COMBAT_LOG_KIND_RECHARGE_ROLLED"}

// comeBackTo ends the turn of the combatant on turn and passes the turns until c is on turn again.
func (w *world) comeBackTo(c *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	for range 40 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == c.GetId() && w.encounter != nil && cur.GetRound() > w.encounter.GetRound() {
			w.encounter = cur
			return
		}
		for _, c := range cur.GetCombatants() { // a character that is down (the fixture's Pensantus) rolls its death save on its turn
			if c.GetId() == cur.GetCurrentCombatantId() && c.GetDeathSaveDue() {
				must(w.master.combat.RollDeathSave(ctx, rq(&playv1.RollDeathSaveRequest{
					CampaignId: w.campaign, EncounterId: cur.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(), Roll: &playv1.RollDeathSaveRequest_D20Face{D20Face: 15},
				})))
			}
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(), DiscardPendingDamage: true})))
	}
	w.t.Fatalf("the turn of %s never came back", c.GetLabel())
}

// useAction is UseCreatureAction as the master (or a player, to see the refusal).
func (w *world) useAction(p *person, who *playv1.Combatant, action string, targets ...string) reply {
	w.t.Helper()
	req := &playv1.UseCreatureActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: who.GetId(), ActionKey: action, TargetIds: targets, IdempotencyKey: newKey(),
	}
	if len(targets) > 0 {
		req.Area = &playv1.UseCreatureActionRequest_Direction{Direction: &playv1.SpellDirection{Dx: -1}}
	}
	return p.call(playv1connect.CombatServiceUseCreatureActionProcedure, req)
}

// limitedOf is the limited action of the combatant as the person reads it.
func (w *world) limitedOf(p *person, who *playv1.Combatant, key string) *playv1.LimitedAction {
	w.t.Helper()
	for _, c := range p.currentEncounter(w).GetCombatants() {
		if c.GetId() == who.GetId() {
			for _, l := range c.GetLimitedActions() {
				if l.GetActionKey() == key {
					return l
				}
			}
		}
	}
	return nil
}

func (w *world) limitedNeedles() []*canary {
	var out []*canary
	for _, word := range limitedWords {
		out = append(out, w.secrets.add(&canary{needle: word, kind: "limited-action"}))
	}
	return out
}

// A spent Recharge action and an action with uses a day, spent once of three: the master reads the state (the
// positive control), and no player reads the field, the words of the limit or the keys, in the combat or the log.
func TestALimitedActionsStateIsTheMastersAlone(t *testing.T) {
	for _, tc := range []struct {
		name, monster, action string
		targets               bool
		available             bool
		left                  int32
	}{
		{"recharge spent", "monster:hell-hound", "fire-breath", true, false, 0},
		{"three a day, one used", "monster:deva", "healing-touch", false, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			needles := w.limitedNeedles()
			w.statBlockNeedles(tc.monster, tc.action)
			creature := w.monsterOnTurn(tc.monster, w.secrets.public("limited-name"), false)
			var targets []string
			if tc.targets {
				targets = []string{w.combatant(w.toren).GetId()}
			}
			use := w.useAction(w.master, creature, tc.monster+"#"+tc.action, targets...)
			if !use.ok() {
				t.Fatalf("the use: status %d %s", use.status, use.body)
			}
			l := w.limitedOf(w.master, creature, tc.monster+"#"+tc.action)
			if l == nil || l.GetAvailable() != tc.available || l.GetUsesLeft() != tc.left {
				t.Fatalf("the master reads %v, want available %v with %d left", l, tc.available, tc.left)
			}
			w.settleAll(creature)
			for _, p := range []*person{w.ana, w.caio} {
				for _, c := range p.currentEncounter(w).GetCombatants() {
					if len(c.GetLimitedActions()) != 0 {
						t.Errorf("%s reads %v on %s", p.name, c.GetLimitedActions(), c.GetLabel())
					}
				}
			}
			w.checkNoStatBlock(t, []reply{use}, nil, needles[0], needles[1], w.canaryNamed("creature-action-key"))
		})
	}
}

// A refused try is the same to a player whatever the creature has spent: only the master may use an NPC's
// action, and the answer a player gets does not tell whether it was spent. The master's refusal names the reason,
// and nothing changed. (The player who controls a creature reads the reason too:
// TestTheControllerOfACreatureIsToldWhyItsSpentActionIsRefused.)
func TestARefusedSpentActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	hound := w.monsterOnTurn("monster:hell-hound", w.secrets.public("limited-name"), false)
	action := "monster:hell-hound#fire-breath"
	target := w.combatant(w.toren).GetId()
	before := map[*person]reply{}
	for _, p := range []*person{w.ana, w.caio} {
		before[p] = w.useAction(p, hound, action, target)
		if before[p].ok() {
			t.Fatalf("%s used the hound's breath", p.name)
		}
	}
	if use := w.useAction(w.master, hound, action, target); !use.ok() {
		t.Fatalf("the breath: status %d %s", use.status, use.body)
	}
	w.settleAll(hound)
	w.scriptRecharge(1)
	w.comeBackTo(hound)
	refused := w.useAction(w.master, hound, action, target)
	if refused.ok() || !bytes.Contains(refused.body, []byte("CREATURE_ACTION_RECHARGING")) {
		t.Errorf("the master's try on a spent breath: status %d %s, want the recharging reason", refused.status, refused.body)
	}
	for _, p := range []*person{w.ana, w.caio} {
		after := w.useAction(p, hound, action, target)
		if after.status != before[p].status || !bytes.Equal(after.body, before[p].body) {
			t.Errorf("%s: the refusal went from %d %s to %d %s once the breath was spent, want the same", p.name, before[p].status, before[p].body, after.status, after.body)
		}
	}
}

// scriptRecharge makes the next d6 the server rolls come out as face.
func (w *world) scriptRecharge(face int) {
	w.t.Helper()
	w.dice.ScriptFor(w.t, 6, face)
}

// turnStart is what the hound's turn starting again left, as the players and the master read it.
type turnStart struct {
	rise     map[string]int32    // how much each player's revision went up
	hints    map[string][]string // the kinds of hint each player's stream got
	entries  map[string]int      // the entries of each player's log
	findings []string            // what the matrix finds in the players' reads
	rolls    []*playv1.CombatLogRecharge
	players  []*playv1.CombatLogEntry // the roll lines in the players' logs
	breath   *playv1.LimitedAction
}

// turnStartOf plays the combat until the hound's turn starts again after it breathed. When spendBreath is false
// the breath is made available by hand before the turn, so the fight is the same and the server has nothing to
// roll; otherwise the server rolls the face given.
func turnStartOf(t *testing.T, spendBreath bool, face int) turnStart {
	t.Helper()
	w := newWorld(t)
	w.limitedNeedles()
	hound := w.monsterOnTurn("monster:hell-hound", w.secrets.public("limited-name"), false)
	if use := w.useAction(w.master, hound, "monster:hell-hound#fire-breath", w.combatant(w.toren).GetId()); !use.ok() {
		t.Fatalf("the breath: status %d %s", use.status, use.body)
	}
	w.settleAll(hound)
	if spendBreath {
		w.scriptRecharge(face)
	} else if _, err := w.pool.Exec(t.Context(), `UPDATE combatants SET creature_spent = '{}' WHERE id = $1`, hound.GetId()); err != nil {
		t.Fatalf("make the breath available: %v", err) // the same fight, with nothing for the server to roll
	}
	players := []*person{w.ana, w.caio}
	before, logged := map[string]int32{}, map[string]int{}
	for _, p := range players {
		before[p.name], logged[p.name] = p.currentEncounter(w).GetRevision(), len(w.logOfPerson(p))
	}
	out := turnStart{rise: map[string]int32{}, hints: map[string][]string{}, entries: map[string]int{}}
	got := w.hiddenOnlyEvents(func() {
		w.comeBackTo(hound)
		for _, p := range players {
			out.rise[p.name] = p.currentEncounter(w).GetRevision() - before[p.name]
			out.entries[p.name] = len(w.logOfPerson(p)) - logged[p.name]
			for _, e := range w.logOfPerson(p) {
				if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RECHARGE_ROLLED || e.GetRecharge() != nil {
					out.players = append(out.players, e)
				}
			}
			for _, r := range w.reads(p) {
				for _, f := range w.inspect(p, r, nil) {
					out.findings = append(out.findings, p.name+": "+f)
				}
			}
		}
		for _, e := range w.logOfPerson(w.master) {
			if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RECHARGE_ROLLED {
				out.rolls = append(out.rolls, e.GetRecharge())
			}
		}
		out.breath = w.limitedOf(w.master, hound, "monster:hell-hound#fire-breath")
	})
	for _, p := range players {
		for _, h := range got[p.name] {
			out.hints[p.name] = append(out.hints[p.name], strings.SplitN(h, " ", 2)[0])
		}
	}
	return out
}

// The server rolls the breath's d6 at the start of the hound's turn: the master reads the line (the positive
// control), and a player's revision, hints and log are those of a turn start with nothing to roll.
func TestARechargeRollAtTheTurnStartOfAnNPCIsTheMastersAlone(t *testing.T) {
	control := turnStartOf(t, false, 0)
	rolled := turnStartOf(t, true, 6)
	for name := range rolled.rise {
		if rolled.rise[name] != control.rise[name] {
			t.Errorf("%s: the revision went up by %d with a roll and by %d without, want the same", name, rolled.rise[name], control.rise[name])
		}
		if !slices.Equal(rolled.hints[name], control.hints[name]) {
			t.Errorf("%s: the stream's hints were %v with a roll and %v without, want the same", name, rolled.hints[name], control.hints[name])
		}
		if rolled.entries[name] != control.entries[name] {
			t.Errorf("%s: the log has %d entries with a roll and %d without, want the same", name, rolled.entries[name], control.entries[name])
		}
	}
	for _, f := range slices.Concat(rolled.findings, control.findings) {
		t.Error(f)
	}
	if len(rolled.players) != 0 {
		t.Errorf("a player reads the roll in the log: %v", rolled.players)
	}
	// The positive controls: the master reads the roll and the breath available again.
	if len(rolled.rolls) != 1 || rolled.rolls[0].GetFace() != 6 || !rolled.rolls[0].GetRecharged() || len(control.rolls) != 0 {
		t.Errorf("the master's log has %v (control %v), want one roll of 6 that recharged", rolled.rolls, control.rolls)
	}
	if rolled.breath == nil || !rolled.breath.GetAvailable() {
		t.Errorf("the master reads %v after a 6, want the breath available", rolled.breath)
	}
}

// The undo of the use gives the breath back, and a player's reads stay neutral before and after.
func TestUndoingALimitedActionLeavesThePlayersNothing(t *testing.T) {
	w := newWorld(t)
	needles := w.limitedNeedles()
	w.statBlockNeedles("monster:hell-hound", "fire-breath")
	hound := w.monsterOnTurn("monster:hell-hound", w.secrets.public("limited-name"), false)
	action := "monster:hell-hound#fire-breath"
	use := w.useAction(w.master, hound, action, w.combatant(w.toren).GetId())
	if !use.ok() {
		t.Fatalf("the breath: status %d %s", use.status, use.body)
	}
	if l := w.limitedOf(w.master, hound, action); l == nil || l.GetAvailable() {
		t.Fatalf("the master reads %v after the use, want it spent", l)
	}
	log := must(w.master.combat.ListCombatLog(t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()})))
	undo := w.master.call(playv1connect.CombatServiceUndoLastActionProcedure, &playv1.UndoLastActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ExpectedEventId: log.GetUndoableEventId(), IdempotencyKey: newKey(),
	})
	if !undo.ok() {
		t.Fatalf("the undo: status %d %s", undo.status, undo.body)
	}
	if l := w.limitedOf(w.master, hound, action); l == nil || !l.GetAvailable() {
		t.Errorf("the master reads %v after the undo, want the breath available", l)
	}
	w.checkNoStatBlock(t, []reply{use, undo}, nil, needles[0], needles[1])
}
