package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Legendary Resistance on the saves a player's action makes: a cast whose saving throw a dragon fails
// waits in a window of the master's alone, with the d20 kept; the master turns the failure into a success or
// not, and the cast goes on with the very same dice. These tests need the database
// (MEURPG_TEST_DATABASE_URL).

var resistKind = playv1.ReactionKind_REACTION_KIND_LEGENDARY_RESISTANCE

// resistFight is a combat on the map with the party (Pensantus plays first), a goblin and the named adult red
// dragons, in plain sight, side by side east of the wizard.
func resistFight(t *testing.T, dragons ...string) (*armed, *playv1.Encounter) {
	t.Helper()
	return resistFightOf(t, "", dragons...)
}

// resistFightOf is resistFight with the three heroes of one race ("" keeps each one's own).
func resistFightOf(t *testing.T, race string, dragons ...string) (*armed, *playv1.Encounter) {
	t.Helper()
	return resistFightWith(t, adultRedDragon, race, dragons...)
}

// resistFightWith is resistFightOf with creatures of the named stat block instead of the adult red dragon.
func resistFightWith(t *testing.T, creature, race string, dragons ...string) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCastersOf(t, 3, race)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
		reveal:  []string{"Goblin"}, setup: true,
		at: map[string][2]int32{"Pensantus": {5, 5}, "Toren": {6, 1}, "Brisa": {4, 1}, "Goblin": {2, 5}},
	})
	for _, name := range dragons {
		a.h.roller.queue(1)
		a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
			r.CreatureKey, r.Count, r.Name, r.Hidden = creature, 1, name, new(false)
		})
	}
	e = a.begin(t, a.get(t, a.master))
	for i, name := range dragons { // one east of the wizard, the next south: a cone reaches one of them
		a.put(t, name, [2]int32{8, 5}[i], [2]int32{5, 8}[i])
	}
	if cur := byLabel(t, e, "Pensantus"); e.GetCurrentCombatantId() != cur.GetId() {
		t.Fatalf("on turn = %s, want Pensantus", e.GetCurrentCombatantId())
	}
	combat := a.get(t, a.master)
	return a, combat
}

// resistSpent is the uses of Legendary Resistance the named dragon spent.
func (a *armed) resistSpentBy(t *testing.T, label string) int {
	t.Helper()
	return a.spentOf(t, a.id(t, label))[spentLegendaryResistance]
}

// holdAt casts Burning Hands at the creatures (a Dexterity save, half damage on a success): the dragon's
// bonus is +6 against DC 14, so a 7 or less fails.
func (a *armed) holdAt(t *testing.T, labels ...string) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return a.cast(t, a.ana, a.get(t, a.master), "Pensantus", burningHands, slotOfLevel(1), a.at(t, labels...), noCastRoll)
}

// damageTo is the pending damage the cast opened for the creature, as the master reads it: whether it is half
// (the save passed), and false when there is none.
func (a *armed) damageTo(t *testing.T, label string) (half, found bool) {
	t.Helper()
	for _, p := range a.mustOptions(t, a.master, a.get(t, a.master), "Pensantus").GetPendingDamages() {
		if p.GetTargetId() == a.id(t, label) {
			return p.GetHalf(), true
		}
	}
	return false, false
}

// resistUse answers the window: USE with the creatures, or PASS when none is named.
func (a *armed) resistUse(t *testing.T, u *user, e *playv1.Encounter, w *playv1.ReactionWindow, labels ...string) (*playv1.AnswerReactionResponse, error) {
	t.Helper()
	return a.answerReaction(t, u, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		if len(labels) == 0 {
			r.Answer = playv1.ReactionChoice_REACTION_CHOICE_PASS
			return
		}
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		for _, l := range labels {
			r.CreatureIds = append(r.CreatureIds, a.id(t, l))
		}
	})
}

// A failed save of the dragon holds the cast in a window of the master's: the cast has done nothing yet (the
// slot is not spent, no damage is opened), no player reads the window, and the master reads the roll that
// failed with the uses left.
func TestLegendaryResistance_AFailedSaveHoldsTheCastForTheMaster(t *testing.T) {
	t.Parallel()
	a, _ := resistFight(t, "Draco")
	a.h.roller.queue(1) // the dragon's Wisdom save: a 1 and 7 against DC 14
	before := a.events(t)
	res, err := a.holdAt(t, "Draco")
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if res.GetCast() != nil {
		t.Errorf("the held cast answers %v, want the combat only", res.GetCast())
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
	}
	offers := w.GetLegendaryResistance().GetOffers()
	if len(offers) != 1 || offers[0].GetCombatantId() != a.id(t, "Draco") || offers[0].GetD20() != 1 || offers[0].GetTotal() != 7 || offers[0].GetDc() != 14 || offers[0].GetUsesLeft() != 3 || offers[0].GetUsesPerDay() != 3 {
		t.Errorf("the offers = %v, want Draco's 1 (7 against 14) with 3 of 3 uses", offers)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 0 {
		t.Errorf("the dragon spent %d uses before the master answered", got)
	}
	if _, found := a.damageTo(t, "Draco"); found || usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Errorf("the held cast opened damage (%v) or spent %d slots", found, usedSlots(a.vitals(t, a.pens), 1))
	}
	if n := a.events(t); n != before+1 {
		t.Errorf("the held cast wrote %d events, want 1 (the hold's)", n-before)
	}
	for _, u := range []*user{a.ana, a.caio, a.bia} {
		if a.windowOf(t, u, resistKind) != nil || len(a.windows(t, u)) != 0 {
			t.Errorf("a player reads the windows %v, want none of them", a.windows(t, u))
		}
		if wait := a.get(t, u).GetReactionWait(); wait.GetTitlePt() != "Esperando o mestre" {
			t.Errorf("a player's wait = %q, want \"Esperando o mestre\"", wait.GetTitlePt())
		}
	}
	if _, err := a.holdAt(t, "Goblin"); err == nil {
		t.Error("another cast went through while the window waits")
	}
}

// The master uses the trait: the failed 1 becomes a success with the very same d20 (the roller is not asked
// again), one use is spent, the cast goes on (the damage is the half of a save that passed) and the master reads
// the roll that failed with the choice, while a player reads a save that passed.
func TestLegendaryResistance_TheMasterTurnsTheFailureIntoASuccessWithTheSameD20(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1, 20, 20, 20) // the 1 is the save; the 20s would show a second roll
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	asked := a.h.roller.taken()
	w := a.windowOf(t, a.master, resistKind)
	res, err := a.resistUse(t, a.master, e, w, "Draco")
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if r := res.GetResult(); !r.GetUsed() || r.GetKind() != resistKind || len(r.GetLegendaryResistance().GetCombatantIds()) != 1 {
		t.Errorf("the result = %v, want the trait used on Draco", r)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none: the d20 is the one that was kept", again-asked)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 1 {
		t.Errorf("the dragon spent %d uses, want 1", got)
	}
	if a.windowOf(t, a.master, resistKind) != nil {
		t.Error("the window is still open after the answer")
	}
	master := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco")
	if master.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || !master.GetResisted() || master.GetResistedLeft() != 2 || master.GetRoll().GetFaces()[0] != 1 || master.GetRoll().GetTotal() != 7 {
		t.Errorf("the master reads %v, want the failed 1 (7) made a success with 2 uses left", master)
	}
	player := a.castSaveOf(t, a.ana, a.get(t, a.ana), "Draco")
	if player.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || player.GetResisted() || player.GetResistedLeft() != 0 || player.GetRoll() != nil {
		t.Errorf("the caster's player reads %v, want a save that passed and nothing else", player)
	}
	if half, found := a.damageTo(t, "Draco"); !found || !half {
		t.Errorf("the dragon's damage is half = %v (found %v), want the half of a save that passed", half, found)
	}
	if l := a.legendaryOf(t, a.master); l.GetResistanceLeft() != 2 {
		t.Errorf("the master reads %v, want 2 uses left", l)
	}
}

// The master declines: every save stays as it was rolled, the dragon is held, nothing is spent, and the cast
// lands with the same d20 (the roller is not asked again).
func TestLegendaryResistance_DecliningLeavesTheSaveFailedAndSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1, 20)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	asked := a.h.roller.taken()
	res, err := a.resistUse(t, a.master, e, a.windowOf(t, a.master, resistKind))
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if res.GetResult().GetUsed() {
		t.Errorf("the result = %v, want the trait declined", res.GetResult())
	}
	if got := a.h.roller.taken(); got != asked {
		t.Errorf("the replay rolled %d dice more, want none", got-asked)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 0 {
		t.Errorf("the dragon spent %d uses on a declined window", got)
	}
	save := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco")
	if save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || save.GetResisted() || save.GetRoll().GetFaces()[0] != 1 {
		t.Errorf("the master reads %v, want the 1 as it failed", save)
	}
	if half, found := a.damageTo(t, "Draco"); !found || half {
		t.Errorf("the dragon's damage is half = %v (found %v), want the full damage of a failed save", half, found)
	}
}

// A cast at no creature that can use the trait opens no window and changes nothing for the table: the
// creature has no trait, or it has no use left.
func TestLegendaryResistance_NoWindowWithoutACreatureThatCan(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]struct {
		target string
		face   int
		spent  int
	}{
		"a creature with no trait": {"Goblin", 1, 0},
		"no use left":              {"Draco", 1, 3},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := resistFight(t, "Draco")
			if set.spent > 0 {
				if _, err := bumpCreatureSpent(t.Context(), playdb.New(a.h.pool), a.id(t, "Draco"), spentLegendaryResistance, set.spent); err != nil {
					t.Fatal(err)
				}
			}
			a.h.roller.queue(set.face)
			res, err := a.holdAt(t, set.target)
			if err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			if res.GetCast() == nil || len(a.windows(t, a.master)) != 0 {
				t.Errorf("the cast = %v, windows = %v; want it resolved at once with no window", res.GetCast(), a.windows(t, a.master))
			}
			if got := a.resistSpentBy(t, "Draco"); got != set.spent {
				t.Errorf("the dragon spent %d uses, want %d", got, set.spent)
			}
		})
	}
}

// A save that passed against a creature that can use the trait holds the cast all the same, so the wait says
// nothing about the roll: the window offers nothing to spend, PASS lets the cast land with the save as rolled,
// and no use is spent.
func TestLegendaryResistance_APassedSaveHoldsTheCastWithNothingToOffer(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(20, 20) // the dragon's save passes; a second roll would show
	before := a.events(t)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	asked := a.h.roller.taken()
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
	}
	if offers := w.GetLegendaryResistance().GetOffers(); len(offers) != 0 {
		t.Errorf("the offers = %v, want none: the save passed", offers)
	}
	if n := a.events(t); n != before+1 {
		t.Errorf("the held cast wrote %d events, want 1 (the hold's)", n-before)
	}
	if _, found := a.damageTo(t, "Draco"); found || usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Error("the held cast opened damage or spent a slot")
	}
	for _, u := range []*user{a.ana, a.caio, a.bia} {
		if len(a.windows(t, u)) != 0 {
			t.Errorf("a player reads the windows %v, want none of them", a.windows(t, u))
		}
		if wait := a.get(t, u).GetReactionWait(); wait.GetTitlePt() != "Esperando o mestre" {
			t.Errorf("a player's wait = %q, want \"Esperando o mestre\"", wait.GetTitlePt())
		}
	}
	// Nothing was offered, so a USE is refused whoever it names, with nothing spent and the window still open.
	events := a.events(t)
	for _, labels := range [][]string{{"Draco"}, {"Pensantus"}} {
		if _, err := a.resistUse(t, a.master, e, w, labels...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("a USE naming %v error = %v, want it refused", labels, err)
		}
	}
	if a.events(t) != events || a.resistSpentBy(t, "Draco") != 0 || a.windowOf(t, a.master, resistKind) == nil {
		t.Error("the refused answer changed something")
	}
	res, err := a.resistUse(t, a.master, e, w)
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if res.GetResult().GetUsed() {
		t.Errorf("the result = %v, want the trait declined", res.GetResult())
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 0 {
		t.Errorf("the dragon spent %d uses, want none", got)
	}
	if a.windowOf(t, a.master, resistKind) != nil {
		t.Error("the window is still open after the answer")
	}
	save := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco")
	if save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || save.GetResisted() || save.GetRoll().GetFaces()[0] != 20 {
		t.Errorf("the master reads %v, want the 20 as it passed", save)
	}
	if half, found := a.damageTo(t, "Draco"); !found || !half {
		t.Errorf("the dragon's damage is half = %v (found %v), want the half of a save that passed", half, found)
	}
	if usedSlots(a.vitals(t, a.pens), 1) != 1 {
		t.Error("the cast did not spend its slot after the answer")
	}
}

// The window may be answered once: a retry of the answer (the same key) spends once and answers the same, and
// the cast request sent again while it waits is the same cast (one hold, one window).
func TestLegendaryResistance_RetriesSpendOnce(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	castKey := newKey()
	for range 2 {
		if _, err := a.castKey(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Draco"), noCastRoll, castKey); err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
	}
	var held int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM reaction_holds`).Scan(&held); err != nil || held != 1 {
		t.Fatalf("holds = %d, %v; want the cast kept once", held, err)
	}
	if n := len(a.windows(t, a.master)); n != 1 {
		t.Fatalf("windows = %d, want 1", n)
	}
	w := a.windowOf(t, a.master, resistKind)
	req := &playv1.AnswerReactionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(),
		Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, CreatureIds: []string{a.id(t, "Draco")},
	}
	for range 2 {
		if _, err := a.master.combat.AnswerReaction(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("AnswerReaction() error = %v", err)
		}
	}
	if got := a.resistSpentBy(t, "Draco"); got != 1 {
		t.Errorf("after the same answer twice the dragon spent %d uses, want 1", got)
	}
	// Another answer to the same window, with a key of its own, lost the race.
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err == nil {
		t.Error("the window was answered twice")
	}
	if got := a.resistSpentBy(t, "Draco"); got != 1 {
		t.Errorf("the dragon spent %d uses, want 1", got)
	}
}

// A player cannot answer the window, and cannot tell it from one that does not exist: the refusal is the
// same, nothing changes, and the master's answer afterwards works (the positive control).
func TestRN20_APlayerCannotAnswerTheLegendaryResistanceWindow(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	events := a.events(t)
	for _, u := range []*user{a.ana, a.caio, a.bia} {
		_, own := a.resistUse(t, u, e, w, "Draco")
		_, fake := a.answerReaction(t, u, e, newKey(), func(r *playv1.AnswerReactionRequest) {
			r.Answer, r.CreatureIds = playv1.ReactionChoice_REACTION_CHOICE_USE, []string{a.id(t, "Draco")}
		})
		if connect.CodeOf(own) != connect.CodePermissionDenied || connect.CodeOf(fake) != connect.CodePermissionDenied || own.Error() != fake.Error() {
			t.Errorf("a player's answers = %v and %v, want the same refusal", own, fake)
		}
	}
	if a.events(t) != events || a.resistSpentBy(t, "Draco") != 0 || a.windowOf(t, a.master, resistKind) == nil {
		t.Error("a refused answer changed something")
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Errorf("the master's answer error = %v", err)
	}
}

// A use is refused when the creature has none left by the time the master answers (another save spent it
// meanwhile), and nothing is spent; the window stays open for the master to answer again.
func TestLegendaryResistance_AnAnswerWithNoUseLeftIsRefused(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if _, err := bumpCreatureSpent(t.Context(), playdb.New(a.h.pool), a.id(t, "Draco"), spentLegendaryResistance, 3); err != nil {
		t.Fatal(err)
	}
	w := a.windowOf(t, a.master, resistKind)
	events := a.events(t)
	_, err := a.resistUse(t, a.master, e, w, "Draco")
	wantBlockedBy(t, "the fourth use", err, blockedNoResistance)
	if a.events(t) != events || a.resistSpentBy(t, "Draco") != 3 || a.windowOf(t, a.master, resistKind) == nil {
		t.Error("the refused answer changed something")
	}
	for _, labels := range [][]string{{"Pensantus"}, {"Draco", "Draco"}} {
		if _, err := a.resistUse(t, a.master, e, w, labels...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("an answer naming %v error = %v, want it refused", labels, err)
		}
	}
	if _, err := a.answerReaction(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) { r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a USE naming no creature error = %v, want it refused", err)
	}
	if _, err := a.resistUse(t, a.master, e, w); err != nil {
		t.Errorf("declining the window error = %v", err)
	}
}

// A cast at several creatures: two dragons fail and the goblin passes. The window offers the two dragons only
// (the goblin has no trait and passed); the master turns one of them, the other stays failed, and each save
// lands with its own d20, rolled once.
func TestLegendaryResistance_AManyTargetCastConvertsOnlyTheChosenFailures(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco", "Drago")
	a.put(t, "Draco", 8, 4)
	a.put(t, "Drago", 8, 5)
	a.put(t, "Goblin", 8, 6)
	a.h.roller.queue(20, 1, 1)
	if _, err := a.holdAt(t, "Draco", "Drago", "Goblin"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	asked := a.h.roller.taken()
	w := a.windowOf(t, a.master, resistKind)
	if w == nil || len(w.GetLegendaryResistance().GetOffers()) != 2 {
		t.Fatalf("the window = %v, want the offers of the two dragons", w)
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if got := a.h.roller.taken(); got != asked {
		t.Errorf("the replay rolled %d dice more, want none", got-asked)
	}
	if a.resistSpentBy(t, "Draco") != 1 || a.resistSpentBy(t, "Drago") != 0 {
		t.Errorf("spent Draco %d, Drago %d; want 1 and 0", a.resistSpentBy(t, "Draco"), a.resistSpentBy(t, "Drago"))
	}
	want := map[string]struct {
		outcome playv1.SaveOutcome
		half    bool
		resist  bool
	}{
		"Draco":  {playv1.SaveOutcome_SAVE_OUTCOME_SAVED, true, true},
		"Drago":  {playv1.SaveOutcome_SAVE_OUTCOME_FAILED, false, false},
		"Goblin": {playv1.SaveOutcome_SAVE_OUTCOME_SAVED, true, false},
	}
	for label, w := range want {
		save := a.castSaveOf(t, a.master, a.get(t, a.master), label)
		half, found := a.damageTo(t, label)
		if save.GetOutcome() != w.outcome || save.GetResisted() != w.resist || !found || half != w.half {
			t.Errorf("%s: the master reads %v with half = %v (found %v); want %v, half %v, resisted %v", label, save, half, found, w.outcome, w.half, w.resist)
		}
		if p := a.castSaveOf(t, a.ana, a.get(t, a.ana), label); p.GetOutcome() != w.outcome || p.GetResisted() || p.GetRoll() != nil {
			t.Errorf("%s: the caster's player reads %v, want the outcome only", label, p)
		}
	}
}

// A dragon that is incapacitated still has the trait: Hold Person on it, then a cast it fails without a roll
// (a paralyzed creature fails Dexterity saves) is offered, and the master turns it.
func TestLegendaryResistance_AnIncapacitatedDragonIsStillOffered(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.mustAddEffect(t, e, holdPerson, []string{"Draco"}, a.rounds(t, 10, "Draco"))
	if a.legendaryOf(t, a.master).GetUsableNow() {
		t.Fatal("the held dragon can take legendary actions: the test needs it incapacitated")
	}
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil || w.GetLegendaryResistance().GetOffers()[0].GetD20() != 0 {
		t.Fatalf("the window = %v, want the offer of a save that failed without a roll", w)
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if save := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco"); save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || !save.GetResisted() || !save.GetAutoFailed() {
		t.Errorf("the master reads %v, want the automatic failure made a success", save)
	}
	if p := a.castSaveOf(t, a.ana, a.get(t, a.ana), "Draco"); p.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || p.GetAutoFailed() || p.GetResisted() {
		t.Errorf("the caster's player reads %v, want a save that passed", p)
	}
}

// The undo of the cast gives the use back with the slot and the damage it opened.
func TestLegendaryResistance_TheUndoOfTheCastGivesTheUseBack(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if _, err := a.resistUse(t, a.master, e, a.windowOf(t, a.master, resistKind), "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if a.resistSpentBy(t, "Draco") != 1 {
		t.Fatalf("the dragon spent %d uses, want 1", a.resistSpentBy(t, "Draco"))
	}
	undoable := a.log(t, a.master, e).GetUndoableEventId()
	if undoable == "" {
		t.Fatal("the master has nothing to undo after the cast")
	}
	if err := a.undo(t, a.master, e, undoable); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 0 {
		t.Errorf("after the undo the dragon spent %d uses, want 0", got)
	}
	if l := a.legendaryOf(t, a.master); l.GetResistanceLeft() != 3 {
		t.Errorf("the master reads %v, want 3 uses left", l)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 0 {
		t.Errorf("after the undo Pensantus spent %d slots, want 0", got)
	}
}

// A cast that waited for another window first (the master's check of "Reações dos inimigos") is held again
// by the dragon's failed save when the first answer replays it: the second hold is made inside the answer's
// transaction, the d20 is rolled once, and the cast lands when the master has answered both.
func TestLegendaryResistance_ACastHeldByAnotherWindowIsHeldAgain(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.setRules(t, enemyReactionsAlways)
	before := a.h.roller.taken()
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	check := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if check == nil || a.windowOf(t, a.master, resistKind) != nil || a.h.roller.taken() != before {
		t.Fatalf("after the cast the windows are %v and %d dice were rolled; want the master's check only, before any save", a.windows(t, a.master), a.h.roller.taken()-before)
	}
	if _, err := a.resistUse(t, a.master, e, check); err != nil {
		t.Fatalf("the master's check error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil || len(a.windows(t, a.master)) != 1 {
		t.Fatalf("after the check the windows are %v, want the Legendary Resistance one", a.windows(t, a.master))
	}
	if usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Error("the cast spent its slot while it waits")
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if got := a.h.roller.taken() - before; got != 1 {
		t.Errorf("the cast rolled %d dice, want the one save", got)
	}
	if a.resistSpentBy(t, "Draco") != 1 || usedSlots(a.vitals(t, a.pens), 1) != 1 {
		t.Errorf("spent %d uses and %d slots, want 1 and 1", a.resistSpentBy(t, "Draco"), usedSlots(a.vitals(t, a.pens), 1))
	}
	if save := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco"); !save.GetResisted() || save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
		t.Errorf("the master reads %v, want the failure made a success", save)
	}
}

// The combat ends while the window waits: the held cast is dropped with it and nothing was spent.
func TestLegendaryResistance_ACombatThatEndsDropsTheHeldCast(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var holds, windows int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM reaction_holds), (SELECT count(*) FROM reaction_windows)`).Scan(&holds, &windows); err != nil || holds != 0 || windows != 0 {
		t.Errorf("holds = %d, windows = %d (%v); want none", holds, windows, err)
	}
	if a.resistSpentBy(t, "Draco") != 0 || usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Error("the dropped cast left a spend")
	}
}
