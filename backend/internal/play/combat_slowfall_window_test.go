package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The question of Slow Fall (SRD 5.1, Monk 4): a monk that falls into a pit is asked whether to use its
// reaction; the fall damage waits for the answer, as it does for a Feather Fall.

var slowFallKind = playv1.ReactionKind_REACTION_KIND_SLOW_FALL

// use is the answer "Usar".
func use(r *playv1.AnswerReactionRequest) { r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE }

// slowFallRig is the cave with a pit on (9, 7) that deals dice, Toren a monk of the level, the combat
// begun and the dice of the fall queued. The wizard has no Feather Fall unless feather is set.
func slowFallRig(t *testing.T, level int32, dice string, feather bool, faces ...int) (*trapRig, *playv1.Encounter) {
	t.Helper()
	spells := []string{burningHands, magicMissileSpell, shieldSpell}
	if feather {
		spells = append(spells, featherFallSpell)
	}
	c := newCaveOf(t, 3, spells, func(a *armed) *charactersv1.Character {
		return a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", level, abilities(12, 16, 14, 14, 8), nil, nil)
	})
	r := newTrapRigOn(t, c)
	r.trap(t, "Fosso", 9, 7, pit(dice), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	e := r.fight(t)
	r.h.roller.queue(faces...)
	return r, e
}

// torenFalls has Toren walk into the pit.
func (r *trapRig) torenFalls(t *testing.T) {
	t.Helper()
	if _, err := r.moveResult(t, r.caio, "Toren", 12, 7); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
}

// trapPoint is the only trap point of the cave's map.
func (r *trapRig) trapPoint(t *testing.T) *mapsv1.MapPoint {
	t.Helper()
	var id string
	if err := r.h.pool.QueryRow(t.Context(), `SELECT id FROM map_points WHERE map_id = $1`, r.mapID).Scan(&id); err != nil {
		t.Fatalf("find the trap point: %v", err)
	}
	return &mapsv1.MapPoint{Id: id}
}

func fiveSixTimes() []int { return []int{5, 5, 5, 5, 5, 5} }

// windowRow reads the status and the closing reason of the window of the kind, from the database.
func (a *armed) windowRow(t *testing.T, kind, reactor string) (status, reason string) {
	t.Helper()
	var why *string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT status, closed_reason FROM reaction_windows WHERE kind = $1 AND reactor_id = $2 ORDER BY seq DESC LIMIT 1`, kind, a.id(t, reactor)).Scan(&status, &why); err != nil {
		t.Fatalf("read the %s window of %s: %v", kind, reactor, err)
	}
	if why != nil {
		reason = *why
	}
	return status, reason
}

// TestSlowFallAsksTheMonkAndNobodyElse: Toren, a monk of level 4, falls: the window is his (the master
// reads it too), says how deep the fall is and how many points the feature takes off, and holds the
// damage. The other players read only that somebody is deciding.
func TestSlowFallAsksTheMonkAndNobodyElse(t *testing.T) {
	t.Parallel()
	r, _ := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	watchers := map[string]*watcher{"master": r.master.watch(t, r.campaignID), "caio": r.caio.watch(t, r.campaignID), "ana": r.ana.watch(t, r.campaignID), "bia": r.bia.watch(t, r.campaignID)}
	for _, w := range watchers {
		w.ready(t)
	}
	r.torenFalls(t)
	r.master.markCurrentMap(t, r.campaignID, r.mapID, watchers["master"], watchers["caio"], watchers["ana"], watchers["bia"])

	w := r.windowOf(t, r.caio, slowFallKind)
	if w == nil || w.GetSlowFall().GetFallFt() != 10 || w.GetSlowFall().GetReduction() != 20 || w.GetReactorLabel() != "Toren" {
		t.Fatalf("Toren's window = %v, want Slow Fall: a fall of 10 ft, 20 points off", w)
	}
	if w.GetTrigger() != nil {
		t.Errorf("the player reads the trigger %v: its numbers are the master's", w.GetTrigger())
	}
	if m := r.windowOf(t, r.master, slowFallKind); m == nil || m.GetId() != w.GetId() {
		t.Errorf("the master reads %v, want the same window", m)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "awaiting_reaction" || amount != 30 {
		t.Errorf("Toren's fall = %s for %d, want it waiting for the window (30)", status, amount)
	}
	for name, u := range map[string]*user{"Ana": r.ana, "Bia": r.bia} {
		enc := r.get(t, u)
		if len(enc.GetReactionWindows()) != 0 || enc.GetReactionWait().GetTitlePt() != "Esperando a reação de Toren" {
			t.Errorf("%s reads %d windows and %q, want none and \"Esperando a reação de Toren\"", name, len(enc.GetReactionWindows()), enc.GetReactionWait().GetTitlePt())
		}
		if got := asJSON(t, enc); strings.Contains(got, w.GetId()) || strings.Contains(strings.ToLower(got), "slowfall") || strings.Contains(got, "SLOW_FALL") {
			t.Errorf("%s reads the window in the combat: %s", name, got)
		}
	}
	// The streams: the window is told to its reactor and to the master, nobody else.
	for name, want := range map[string]bool{"master": true, "caio": true, "ana": false, "bia": false} {
		opened := 0
		for _, ev := range watchers[name].beforeMarker(t) {
			if ev.GetReactionWindowOpened() != nil {
				opened++
			}
		}
		if (opened > 0) != want {
			t.Errorf("%s was told of %d opened windows, want some: %v", name, opened, want)
		}
	}
}

// TestSlowFallUsedTakesFiveTimesTheLevelOffAndSpendsTheReaction: 30 points become 10 and wait for the
// master; the player reads the points taken off and the master reads the damage around them.
func TestSlowFallUsedTakesFiveTimesTheLevelOffAndSpendsTheReaction(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	w := r.windowOf(t, r.caio, slowFallKind)
	res := r.mustAnswer(t, r.caio, e, w.GetId(), use)
	got := res.GetResult()
	if got.GetKind() != slowFallKind || !got.GetUsed() || got.GetSlowFall().GetReduction() != 20 || got.GetSlowFall().DamageBefore != nil || got.GetSlowFall().DamageAfter != nil {
		t.Errorf("the player's result = %v, want 20 points off and no damage numbers", got)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 10 {
		t.Errorf("Toren's fall = %s for %d, want it rolled for the master (10)", status, amount)
	}
	toren := byLabel(t, r.get(t, r.caio), "Toren")
	if !toren.GetReactionUsed() || toren.GetReactionKind() != slowFallKind {
		t.Errorf("Toren's reaction = used %v, %v; want Slow Fall spent", toren.GetReactionUsed(), toren.GetReactionKind())
	}
	if n := len(r.get(t, r.caio).GetReactionWindows()); n != 0 {
		t.Errorf("%d windows left after the answer", n)
	}
	lines := map[string]string{}
	for name, u := range map[string]*user{"master": r.master, "caio": r.caio, "ana": r.ana} {
		for _, round := range r.log(t, u, r.get(t, u)).GetRounds() {
			for _, entry := range round.GetEntries() {
				if entry.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
					lines[name] = entry.GetReactionTextPt()
				}
			}
		}
	}
	want := map[string]string{
		"master": "Toren usou Queda Lenta: -20 no dano da queda (de 30 para 10).",
		"caio":   "Toren usou Queda Lenta: -20 no dano da queda.",
		"ana":    "Toren usou Queda Lenta.",
	}
	for name, line := range want {
		if lines[name] != line {
			t.Errorf("%s's log line = %q, want %q", name, lines[name], line)
		}
	}
}

// TestSlowFallAnsweredByTheMasterReadsTheDamage: the master answers for the player and reads the damage
// before and after.
func TestSlowFallAnsweredByTheMasterReadsTheDamage(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	w := r.windowOf(t, r.master, slowFallKind)
	res := r.mustAnswer(t, r.master, e, w.GetId(), use).GetResult()
	if !res.GetByMaster() || res.GetSlowFall().GetDamageBefore() != 30 || res.GetSlowFall().GetDamageAfter() != 10 {
		t.Errorf("the master's result = %v, want the damage 30 then 10", res)
	}
}

// TestSlowFallOfASmallFallFloorsAtZero: a fall of 4 points loses 4, not 20, and the excess is not carried.
func TestSlowFallOfASmallFallFloorsAtZero(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "1d6", false, 4)
	r.torenFalls(t)
	w := r.windowOf(t, r.caio, slowFallKind)
	if got := w.GetSlowFall().GetReduction(); got != 20 {
		t.Errorf("the prompt offers %d points, want 20 (5 x 4)", got)
	}
	res := r.mustAnswer(t, r.caio, e, w.GetId(), use).GetResult()
	if got := res.GetSlowFall().GetReduction(); got != 4 {
		t.Errorf("the points taken off = %d, want 4: the fall had only 4", got)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 0 {
		t.Errorf("Toren's fall = %s for %d, want 0", status, amount)
	}
}

// TestSlowFallPassedLeavesTheFallAsItWas: "Deixar passar" keeps the damage and the reaction.
func TestSlowFallPassedLeavesTheFallAsItWas(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	w := r.windowOf(t, r.caio, slowFallKind)
	if res := r.mustAnswer(t, r.caio, e, w.GetId(), passAnswer).GetResult(); res.GetUsed() {
		t.Errorf("the result = %v, want a pass", res)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 30 {
		t.Errorf("Toren's fall = %s for %d, want it rolled for the master (30)", status, amount)
	}
	if byLabel(t, r.get(t, r.caio), "Toren").GetReactionUsed() {
		t.Error("passing spent the reaction")
	}
	// The button is still there for a fall that landed in the master's hands.
	if _, err := r.action(t, r.caio, r.get(t, r.caio), "Toren", slowFallAction); err != nil {
		t.Fatalf("TakeAction(Slow Fall) after the pass error = %v", err)
	}
	if _, amount := r.fallOf(t, "Toren"); amount != 10 {
		t.Errorf("the fall = %d after the button, want 10", amount)
	}
}

// TestSlowFallButtonClosesTheQuestion: the button spends the reaction, which
// closes the window of the question: a monk has one reaction.
func TestSlowFallButtonClosesTheQuestion(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	if _, err := r.action(t, r.caio, e, "Toren", slowFallAction); err != nil {
		t.Fatalf("TakeAction(Slow Fall) error = %v", err)
	}
	if status, reason := r.windowRow(t, "slow_fall", "Toren"); status != "closed" || reason != "reaction_spent" {
		t.Errorf("the window is %s (%s), want it closed: the reaction was spent", status, reason)
	}
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 10 {
		t.Errorf("Toren's fall = %s for %d, want it rolled (10)", status, amount)
	}
}

// TestSlowFallAndFeatherFallShareTheGroup: the monk is asked first (initiative), and the wizard's
// Feather Fall window waits for it; each has its own reaction.
func TestSlowFallAndFeatherFallShareTheGroup(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", true, fiveSixTimes()...)
	r.torenFalls(t)
	monk := r.windowOf(t, r.caio, slowFallKind)
	wizard := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL)
	if monk == nil || wizard == nil || monk.GetGroupId() != wizard.GetGroupId() {
		t.Fatalf("windows %v and %v, want the monk's and the wizard's in one group", monk, wizard)
	}
	if _, err := r.answerReaction(t, r.ana, e, wizard.GetId(), passAnswer); err == nil {
		t.Error("the wizard answered before the monk, who comes first in the initiative")
	}
	r.mustAnswer(t, r.caio, e, monk.GetId(), use)
	if status, amount := r.fallOf(t, "Toren"); status != "awaiting_reaction" || amount != 10 {
		t.Errorf("Toren's fall = %s for %d, want it still waiting for the wizard (10)", status, amount)
	}
	r.mustAnswer(t, r.ana, e, wizard.GetId(), func(q *playv1.AnswerReactionRequest) {
		q.Answer, q.Slot, q.CreatureIds = playv1.ReactionChoice_REACTION_CHOICE_USE, slotOfLevel(1), []string{r.id(t, "Toren")}
	})
	if status, _ := r.fallOf(t, "Toren"); status != "discarded" {
		t.Errorf("Toren's fall is %s, want it discarded by the Feather Fall", status)
	}
}

// TestSlowFallDoesNotAskWhomItCannotServe: a reaction already spent, a monk of level 3 and a creature
// of the table that is not a character open no window, and the fall lands as before.
func TestSlowFallDoesNotAskWhomItCannotServe(t *testing.T) {
	t.Parallel()
	t.Run("reaction spent", func(t *testing.T) {
		t.Parallel()
		r, _ := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
		if _, err := r.h.pool.Exec(t.Context(), `UPDATE combatants SET reaction_used = true WHERE id = $1`, r.id(t, "Toren")); err != nil {
			t.Fatalf("spend the reaction: %v", err)
		}
		r.torenFalls(t)
		if w := r.windowOf(t, r.caio, slowFallKind); w != nil {
			t.Errorf("a monk with no reaction is asked: %v", w)
		}
		if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 30 {
			t.Errorf("Toren's fall = %s for %d, want it rolled (30)", status, amount)
		}
	})
	t.Run("level 3", func(t *testing.T) {
		t.Parallel()
		r, _ := slowFallRig(t, 3, "6d6", false, fiveSixTimes()...)
		r.torenFalls(t)
		if w := r.windowOf(t, r.caio, slowFallKind); w != nil {
			t.Errorf("a monk of level 3 is asked: %v", w)
		}
		if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 30 {
			t.Errorf("Toren's fall = %s for %d, want it rolled (30)", status, amount)
		}
	})
	t.Run("an NPC", func(t *testing.T) {
		t.Parallel()
		r, _ := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
		pt := r.trapPoint(t)
		if _, err := r.moveResult(t, r.master, "Goblin 1", 10, 7); err != nil {
			t.Fatalf("MoveCombatant(Goblin 1) error = %v", err)
		}
		if _, err := r.fireByHand(t, pt, r.id(t, "Goblin 1")); err != nil {
			t.Fatalf("FireTrap() error = %v", err)
		}
		if n := len(r.get(t, r.master).GetReactionWindows()); n != 0 {
			t.Errorf("an NPC's fall opened %d windows, want none", n)
		}
		if status, _ := r.fallOf(t, "Goblin 1"); status != "applied" {
			t.Errorf("the NPC's fall is %s, want it applied at once", status)
		}
	})
}

// TestSlowFallUndoGivesTheReactionAndThePointsBack: after the undo the window is open, the fall waits
// with its 30 points and the reaction is free.
func TestSlowFallUndoGivesTheReactionAndThePointsBack(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	id := r.windowOf(t, r.caio, slowFallKind).GetId()
	w := newUndoWatch(t, r.armed)
	w.run(t, "Slow Fall window", nil, func() { r.mustAnswer(t, r.caio, e, id, use) })
	if status, amount := r.fallOf(t, "Toren"); status != "awaiting_reaction" || amount != 30 {
		t.Errorf("Toren's fall = %s for %d after the undo, want it waiting (30)", status, amount)
	}
	if got := r.windowOf(t, r.caio, slowFallKind); got == nil || got.GetId() != id {
		t.Errorf("the window after the undo = %v, want %s open again", got, id)
	}
	if byLabel(t, r.get(t, r.caio), "Toren").GetReactionUsed() {
		t.Error("the reaction is still spent after the undo")
	}
}

// TestSlowFallOfTwoMonksIsIndependent: each monk is asked about its own fall, and what one takes off is
// not taken off the other's.
func TestSlowFallOfTwoMonksIsIndependent(t *testing.T) {
	t.Parallel()
	c := newCaveFrom(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", 4, abilities(12, 16, 14, 14, 8), nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:monk", "race:human", 6, abilities(12, 16, 14, 14, 8), nil, nil)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:halfling", 2,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	r := newTrapRigOn(t, c)
	r.trap(t, "Fosso", 9, 7, pit("6d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	r.trap(t, "Fosso 2", 9, 8, pit("6d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	e := r.fight(t)
	r.h.roller.queue(fiveSixTimes()...)
	r.torenFalls(t)
	if w := r.windowOf(t, r.ana, slowFallKind); w != nil {
		t.Fatalf("Pensantus is asked about Toren's fall: %v", w)
	}
	r.mustAnswer(t, r.caio, e, r.windowOf(t, r.caio, slowFallKind).GetId(), use)

	r.h.roller.queue(fiveSixTimes()...)
	if _, err := r.moveResult(t, r.master, "Pensantus", 9, 8); err != nil {
		t.Fatalf("MoveCombatant(Pensantus) error = %v", err)
	}
	if w := r.windowOf(t, r.caio, slowFallKind); w != nil {
		t.Errorf("Toren is asked about Pensantus's fall: %v", w)
	}
	w := r.windowOf(t, r.ana, slowFallKind)
	if w == nil || w.GetSlowFall().GetReduction() != 30 {
		t.Fatalf("Pensantus's window = %v, want Slow Fall with 30 points (5 x 6)", w)
	}
	r.mustAnswer(t, r.ana, e, w.GetId(), use)
	if status, amount := r.fallOf(t, "Pensantus"); status != "rolled" || amount != 0 {
		t.Errorf("Pensantus's fall = %s for %d, want 0 (30 - 30)", status, amount)
	}
	var torenAmount int32
	if err := r.h.pool.QueryRow(t.Context(), `SELECT amount FROM pending_damages WHERE target_id = $1 AND trap_point_id IS NOT NULL`, r.id(t, "Toren")).Scan(&torenAmount); err != nil || torenAmount != 10 {
		t.Errorf("Toren's fall = %d (%v), want 10: only his own 20 came off", torenAmount, err)
	}
}

// multiclassRig is the cave with a pit on (9, 7), Toren a monk of level 4 and a wizard of level 3 with
// Feather Fall prepared, and the combat begun with the dice of the fall queued: one reactor with the two
// windows of one fall. CreateCharacter refuses the multiclass, so the sheet is the monk's rewritten.
func multiclassRig(t *testing.T) (*trapRig, *playv1.Encounter) {
	t.Helper()
	c := newCaveOf(t, 3, []string{burningHands, magicMissileSpell, shieldSpell}, func(a *armed) *charactersv1.Character {
		toren := a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", 4, abilities(12, 16, 14, 14, 8), nil, nil)
		if _, err := a.h.pool.Exec(t.Context(), `
			UPDATE characters SET sheet = sheet || jsonb_build_object('full', (sheet -> 'full')
				|| jsonb_build_object(
					'classes', (sheet #> '{full,classes}') || '[{"level": 3, "class_key": "class:wizard"}]'::jsonb,
					'known_spell_keys', '["spell:feather-fall"]'::jsonb,
					'prepared_spell_keys', '["spell:feather-fall"]'::jsonb))
			WHERE id = $1`, toren.GetId()); err != nil {
			t.Fatalf("make the multiclass sheet: %v", err)
		}
		return toren
	})
	r := newTrapRigOn(t, c)
	r.trap(t, "Fosso", 9, 7, pit("6d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	e := r.fight(t)
	r.h.roller.queue(fiveSixTimes()...)
	r.torenFalls(t)
	return r, e
}

// featherFor answers "Usar" with a slot of the first level, on Toren.
func featherFor(t *testing.T, r *trapRig) func(*playv1.AnswerReactionRequest) {
	t.Helper()
	return func(q *playv1.AnswerReactionRequest) {
		q.Answer, q.Slot, q.CreatureIds = playv1.ReactionChoice_REACTION_CHOICE_USE, slotOfLevel(1), []string{r.id(t, "Toren")}
	}
}

// TestSlowFallOfAMonkWhoCanAlsoCastFeatherFall: one reactor, one reaction, two windows of one fall.
func TestSlowFallOfAMonkWhoCanAlsoCastFeatherFall(t *testing.T) {
	t.Parallel()
	feather := playv1.ReactionKind_REACTION_KIND_FEATHER_FALL
	t.Run("the Slow Fall waits for the Feather Fall", func(t *testing.T) {
		t.Parallel()
		r, e := multiclassRig(t)
		slow := r.windowOf(t, r.caio, slowFallKind)
		if slow == nil || r.windowOf(t, r.caio, feather) == nil {
			t.Fatalf("Toren reads %v, want both windows", r.windows(t, r.caio))
		}
		_, err := r.answerReaction(t, r.caio, e, slow.GetId(), use)
		wantRefusal(t, "Slow Fall before Feather Fall", err, connect.CodeFailedPrecondition, "this reaction window cannot be answered now")
	})
	t.Run("Feather Fall first discards the fall and closes the Slow Fall", func(t *testing.T) {
		t.Parallel()
		r, e := multiclassRig(t)
		r.mustAnswer(t, r.caio, e, r.windowOf(t, r.caio, feather).GetId(), featherFor(t, r))
		if status, _ := r.fallOf(t, "Toren"); status != "discarded" {
			t.Errorf("Toren's fall is %s, want it discarded", status)
		}
		if status, reason := r.windowRow(t, "slow_fall", "Toren"); status != "closed" || reason != "reaction_spent" {
			t.Errorf("the Slow Fall window is %s (%s), want it closed: the reaction was spent", status, reason)
		}
	})
	t.Run("Feather Fall passed, Slow Fall used", func(t *testing.T) {
		t.Parallel()
		r, e := multiclassRig(t)
		r.mustAnswer(t, r.caio, e, r.windowOf(t, r.caio, feather).GetId(), passAnswer)
		r.mustAnswer(t, r.caio, e, r.windowOf(t, r.caio, slowFallKind).GetId(), use)
		if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 10 {
			t.Errorf("Toren's fall = %s for %d, want it rolled (30 - 20 = 10)", status, amount)
		}
	})
	t.Run("the button closes both and the undo opens both again", func(t *testing.T) {
		t.Parallel()
		r, e := multiclassRig(t)
		w := newUndoWatch(t, r.armed)
		w.run(t, "Slow Fall button", nil, func() {
			if _, err := r.action(t, r.caio, e, "Toren", slowFallAction); err != nil {
				t.Fatalf("TakeAction(Slow Fall) error = %v", err)
			}
		})
		if r.windowOf(t, r.caio, feather) == nil || r.windowOf(t, r.caio, slowFallKind) == nil {
			t.Errorf("Toren reads %v after the undo, want both windows open", r.windows(t, r.caio))
		}
	})
}

// TestSlowFallButtonUndoOpensTheQuestionAgain: the button while the window is open, then the undo.
func TestSlowFallButtonUndoOpensTheQuestionAgain(t *testing.T) {
	t.Parallel()
	r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
	r.torenFalls(t)
	w := newUndoWatch(t, r.armed)
	w.run(t, "Slow Fall button", nil, func() {
		if _, err := r.action(t, r.caio, e, "Toren", slowFallAction); err != nil {
			t.Fatalf("TakeAction(Slow Fall) error = %v", err)
		}
	})
	if status, amount := r.fallOf(t, "Toren"); status != "awaiting_reaction" || amount != 30 {
		t.Errorf("Toren's fall = %s for %d after the undo, want it waiting (30)", status, amount)
	}
	if r.windowOf(t, r.caio, slowFallKind) == nil {
		t.Error("the window is closed after the undo")
	}
	if byLabel(t, r.get(t, r.caio), "Toren").GetReactionUsed() {
		t.Error("the reaction is still spent after the undo")
	}
}

// A "Deixar passar" on a fall window is a reaction_answered event, which the master's undo does not take back: it is not the
// undoable event of the log and an undo that names it is refused, so the fall that went on stays with the master.
func TestAPassOnAFallWindowIsNotUndoable(t *testing.T) {
	t.Parallel()
	feather := playv1.ReactionKind_REACTION_KIND_FEATHER_FALL
	for name, tc := range map[string]struct {
		rig    func(t *testing.T) (*trapRig, *playv1.Encounter)
		kind   playv1.ReactionKind
		player func(r *trapRig) *user
	}{
		"Slow Fall": {func(t *testing.T) (*trapRig, *playv1.Encounter) {
			t.Helper()
			r, e := slowFallRig(t, 4, "6d6", false, fiveSixTimes()...)
			r.torenFalls(t)
			return r, e
		}, slowFallKind, func(r *trapRig) *user { return r.caio }},
		"Feather Fall": {fallingRig, feather, func(r *trapRig) *user { return r.ana }},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, e := tc.rig(t)
			r.mustAnswer(t, tc.player(r), e, r.windowOf(t, tc.player(r), tc.kind).GetId(), passAnswer)
			if status, _ := r.fallOf(t, "Toren"); status != "rolled" {
				t.Fatalf("Toren's fall = %s after the pass, want rolled", status)
			}
			if id := r.armed.log(t, r.master, e).GetUndoableEventId(); id != "" {
				t.Errorf("the log offers %s to undo after a pass, want nothing", id)
			}
			if err := r.undo(t, r.master, e, newKey()); err == nil {
				t.Error("an undo after a pass succeeded")
			}
			if status, _ := r.fallOf(t, "Toren"); status != "rolled" {
				t.Errorf("Toren's fall = %s after the refused undo, want rolled", status)
			}
		})
	}
}
