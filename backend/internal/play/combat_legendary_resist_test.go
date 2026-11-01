package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Legendary Resistance (SRD 5.1: "If the creature fails a saving throw, it can choose to succeed
// instead", 3/Day) on the saves the master rolls for the creature: an effect's (RollEffectSave) and a
// concentration's (ResolveConcentrationSave). A roll that fails becomes a success that spends one use, in
// the same change; a roll that succeeds spends nothing. These tests need the database
// (MEURPG_TEST_DATABASE_URL).

var (
	blockedNoResistance  = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_RESISTANCE_NONE_LEFT
	blockedNoResistTrait = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_RESISTANCE_UNAVAILABLE
)

// resistanceSpent is the uses of Legendary Resistance the dragon has spent.
func (a *armed) resistanceSpent(t *testing.T) int {
	t.Helper()
	return a.spentOf(t, a.id(t, "Draco"))[spentLegendaryResistance]
}

// resisting is a save request with the master's choice made.
func resisting(set func(*playv1.RollEffectSaveRequest)) func(*playv1.RollEffectSaveRequest) {
	return func(r *playv1.RollEffectSaveRequest) {
		set(r)
		r.UseLegendaryResistance = true
	}
}

// resistingConcentration is a concentration save request with the master's choice made.
func resistingConcentration(set func(*playv1.ResolveConcentrationSaveRequest)) func(*playv1.ResolveConcentrationSaveRequest) {
	return func(r *playv1.ResolveConcentrationSaveRequest) {
		set(r)
		r.UseLegendaryResistance = true
	}
}

// heldDragon has Hold Person on the dragon (a Wisdom save at the end of its turn, DC 10) and its turn end:
// the master has the save to roll. The hold paralyzes the dragon, so it is incapacitated for the rest.
func heldDragon(t *testing.T) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a, e, _ := dragonFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Draco"}, a.rounds(t, 10, "Draco"))
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the dragon's windows = %v, want the save of Hold Person", a.windows(t, a.master))
	}
	return a, e, w
}

// holdsTheDragon says the dragon still has Hold Person.
func (a *armed) holdsTheDragon(t *testing.T) bool {
	t.Helper()
	return cardOf(t, byLabel(t, a.get(t, a.master), "Draco"), holdPerson) != nil
}

// A failed save of an effect becomes a success that spends one use: the hold ends, the master reads the
// roll that failed and the uses left, and the dragon is incapacitated all the while (it is a trait, not an
// action).
func TestLegendaryResistance_AFailedEffectSaveBecomesASuccessAndSpendsOne(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	if a.legendaryOf(t, a.master).GetUsableNow() {
		t.Fatal("the held dragon can take legendary actions: the test needs it incapacitated")
	}
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(1)))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	r := res.GetResult()
	if !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 || r.GetD20() != 1 || !r.GetEffectEnded() || r.GetTotal() >= r.GetDc() {
		t.Errorf("the master's result = %v, want the failed 1 made a success with 2 uses left, ending the hold", r)
	}
	if got := a.resistanceSpent(t); got != 1 {
		t.Errorf("the dragon spent %d uses, want 1", got)
	}
	if a.holdsTheDragon(t) {
		t.Error("the dragon is still held after the save was made a success")
	}
	if l := a.legendaryOf(t, a.master); l.GetResistanceLeft() != 2 {
		t.Errorf("the master reads %v, want 2 uses left", l)
	}
}

// A request sent twice spends once, and the retry answers what the first did.
func TestLegendaryResistance_ARetryOfTheSameRequestSpendsOnce(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	req := &playv1.RollEffectSaveRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(), UseLegendaryResistance: true,
		Roll: &playv1.RollEffectSaveRequest_D20Face{D20Face: 2},
	}
	var results []*playv1.EffectSaveResult
	for range 2 {
		res, err := a.master.lasting.RollEffectSave(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("RollEffectSave() error = %v", err)
		}
		results = append(results, res.Msg.GetResult())
	}
	if got := a.resistanceSpent(t); got != 1 {
		t.Errorf("after the same request twice the dragon spent %d uses, want 1", got)
	}
	if r := results[1]; !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 || r.GetD20() != 2 {
		t.Errorf("the retry answers %v, want the first answer (a failed 2 made a success, 2 left)", r)
	}
}

// A roll that succeeds spends nothing, and the choice is ignored: the master need not guess before the dice.
func TestLegendaryResistance_ARollThatSucceedsSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(20)))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r := res.GetResult(); !r.GetSaved() || r.GetResisted() || r.GetResistedLeft() != 0 {
		t.Errorf("the result = %v, want a save that passed by itself", r)
	}
	if got := a.resistanceSpent(t); got != 0 {
		t.Errorf("the dragon spent %d uses on a save it passed, want none", got)
	}
}

// Without the choice a failed save stays failed and spends nothing.
func TestLegendaryResistance_WithoutTheChoiceAFailureStaysAFailure(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(1))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r := res.GetResult(); r.GetSaved() || r.GetResisted() {
		t.Errorf("the result = %v, want a failure", r)
	}
	if got := a.resistanceSpent(t); got != 0 || !a.holdsTheDragon(t) {
		t.Errorf("spent %d uses, held = %v; want none spent and the hold kept", got, a.holdsTheDragon(t))
	}
}

// The third use is the last: the fourth failed save is refused, and nothing changes (the window stays open
// for the master to roll it again, the hold stays, no use is spent, no event is written).
func TestLegendaryResistance_TheFourthUseIsRefusedAndNothingChanges(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	if _, err := bumpCreatureSpent(t.Context(), playdb.New(a.h.pool), a.id(t, "Draco"), spentLegendaryResistance, 2); err != nil {
		t.Fatal(err)
	}
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(1)))
	if err != nil || res.GetResult().GetResistedLeft() != 0 || !res.GetResult().GetResisted() {
		t.Fatalf("the third use = %v, %v; want it used with none left", res.GetResult(), err)
	}
	if got := a.resistanceSpent(t); got != 3 {
		t.Fatalf("after the third use the dragon spent %d, want 3", got)
	}

	// Another save of the same dragon: Hold Person again, its turn comes back round and ends.
	a.mustAddEffect(t, e, holdPerson, []string{"Draco"}, a.rounds(t, 10, "Draco"))
	again := a.windowAfterTheDragonsNextTurn(t, e)
	events := a.events(t)
	_, err = a.rollEffectSave(t, a.master, e, again.GetId(), resisting(effectSaveFace(1)))
	wantBlockedBy(t, "the fourth use", err, blockedNoResistance)
	if got, n := a.resistanceSpent(t), a.events(t); got != 3 || n != events || !a.holdsTheDragon(t) {
		t.Errorf("after the refusal spent = %d, events = %d (were %d), held = %v; want 3, the same, held", got, n, events, a.holdsTheDragon(t))
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) == nil {
		t.Error("the refused save closed its window")
	}
	// Rolled without the choice, the same window is answered as a failure.
	if res, err := a.rollEffectSave(t, a.master, e, again.GetId(), effectSaveFace(1)); err != nil || res.GetResult().GetSaved() {
		t.Errorf("the window rolled without the choice = %v, %v; want a failure", res.GetResult(), err)
	}
}

// windowAfterTheDragonsNextTurn passes the turns until the dragon's turn ends again and answers the
// master's open save window the turns left in between (none), returning the dragon's new save window.
func (a *armed) windowAfterTheDragonsNextTurn(t *testing.T, e *playv1.Encounter) *playv1.ReactionWindow {
	t.Helper()
	e = a.passTo(t, e, "Draco")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the dragon's windows = %v, want the save of Hold Person", a.windows(t, a.master))
	}
	return w
}

// A combatant without the trait is refused before the roll, and nothing changes: here a player's character
// (the master rolls its save).
func TestLegendaryResistance_ACreatureWithoutTheTraitIsRefused(t *testing.T) {
	t.Parallel()
	a, e, _ := dragonFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the windows = %v, want Toren's save", a.windows(t, a.master))
	}
	events := a.events(t)
	_, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(1)))
	wantBlockedBy(t, "Toren's save with the choice", err, blockedNoResistTrait)
	if n := a.events(t); n != events || cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), holdPerson) == nil {
		t.Errorf("events = %d (were %d) or the hold ended; want nothing changed", n, events)
	}
	if got := a.resistanceSpent(t); got != 0 {
		t.Errorf("the dragon spent %d uses on a refusal about Toren", got)
	}
}

// A player is refused the choice, on a save of their own and on the dragon's, and nothing changes.
func TestRN20_APlayerCannotUseLegendaryResistance(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	events := a.events(t)
	_, err := a.rollEffectSave(t, a.caio, e, w.GetId(), resisting(effectSaveFace(1)))
	wantCode(t, "a player's choice on the dragon's save", err, connect.CodePermissionDenied)
	if n := a.events(t); n != events || a.resistanceSpent(t) != 0 || !a.holdsTheDragon(t) {
		t.Errorf("events = %d (were %d), spent = %d; want nothing changed", n, events, a.resistanceSpent(t))
	}
}

// concentratingDragon has the dragon concentrate on Teia and a second adult red dragon, Rival, bite it
// (a critical hit): the master has the concentration save to roll against DC 24 (the dragon's
// Constitution save is +13, so a 1 fails).
func concentratingDragon(t *testing.T) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a, e, _ := dragonFight(t)
	a.h.roller.queue(3)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey = adultRedDragon
		r.Count = 1
		r.Name = "Rival"
		r.Hidden = new(false)
	})
	e = a.passTo(t, e, "Rival")
	a.concentrate(t, "Draco", webSpell)
	hit := a.mustAttack(t, a.master, e, "Rival", dragonBite, "Draco", d20(20))
	a.h.roller.queue(10, 10, 10, 10)
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the dragon's concentration save", a.windows(t, a.master))
	}
	return a, e, w
}

func (a *armed) dragonConcentratesOn(t *testing.T) string {
	t.Helper()
	return byLabel(t, a.get(t, a.master), "Draco").GetConcentrationSpell()
}

// A failed concentration save becomes a success that spends one use: the dragon keeps concentrating.
func TestLegendaryResistance_AFailedConcentrationSaveIsKept(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	res, err := a.resolveConcentration(t, a.master, e, w.GetId(), resistingConcentration(concentrationFace(1)))
	if err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	r := res.GetResult()
	if !r.GetKept() || !r.GetResisted() || r.GetResistedLeft() != 2 || r.GetSave().GetTotal() >= r.GetDc() {
		t.Errorf("the master's result = %v, want a failed roll made a success with 2 uses left", r)
	}
	if got, spell := a.resistanceSpent(t), a.dragonConcentratesOn(t); got != 1 || spell != webSpell {
		t.Errorf("spent = %d, concentrating on %q; want 1 use and Teia kept", got, spell)
	}
}

// The same refusals as the effect's: no use left changes nothing and the concentration waits; a roll that
// passes spends nothing; a player is refused.
func TestLegendaryResistance_ConcentrationRefusalsAndAPassedRoll(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	_, err := a.resolveConcentration(t, a.caio, e, w.GetId(), resistingConcentration(concentrationFace(1)))
	wantCode(t, "a player's choice", err, connect.CodePermissionDenied)
	if _, err := bumpCreatureSpent(t.Context(), playdb.New(a.h.pool), a.id(t, "Draco"), spentLegendaryResistance, 3); err != nil {
		t.Fatal(err)
	}
	events := a.events(t)
	_, err = a.resolveConcentration(t, a.master, e, w.GetId(), resistingConcentration(concentrationFace(1)))
	wantBlockedBy(t, "no use left", err, blockedNoResistance)
	if n := a.events(t); n != events || a.dragonConcentratesOn(t) != webSpell || a.resistanceSpent(t) != 3 {
		t.Errorf("after the refusal events = %d (were %d), concentrating on %q, spent %d; want nothing changed", n, events, a.dragonConcentratesOn(t), a.resistanceSpent(t))
	}
	// A roll that passes needs no use: with none left, the choice is ignored.
	res, err := a.resolveConcentration(t, a.master, e, w.GetId(), resistingConcentration(concentrationFace(20)))
	if err != nil {
		t.Fatalf("ResolveConcentrationSave(20) error = %v", err)
	}
	if r := res.GetResult(); !r.GetKept() || r.GetResisted() || a.resistanceSpent(t) != 3 {
		t.Errorf("the result = %v, spent %d; want a save that passed by itself, nothing spent", r, a.resistanceSpent(t))
	}
}

// A concentration save the master keeps without a roll is no failure: the choice spends nothing.
func TestLegendaryResistance_AKeptConcentrationSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	if _, err := a.resolveConcentration(t, a.master, e, w.GetId(), resistingConcentration(keepConcentration)); err != nil {
		t.Fatalf("ResolveConcentrationSave(keep) error = %v", err)
	}
	if got := a.resistanceSpent(t); got != 0 {
		t.Errorf("a kept concentration spent %d uses, want none", got)
	}
}

// The saves of a dragon that failed and was not helped by the trait still end the concentration.
func TestLegendaryResistance_WithoutTheChoiceTheConcentrationEnds(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	if _, err := a.resolveConcentration(t, a.master, e, w.GetId(), concentrationFace(1)); err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	if got := a.dragonConcentratesOn(t); got != "" || a.resistanceSpent(t) != 0 {
		t.Errorf("concentrating on %q, spent %d; want the concentration lost and nothing spent", got, a.resistanceSpent(t))
	}
}

// The save events are not among the undoable kinds, so a use cannot be given back by "Desfazer última
// ação": the use stays spent and the undo has nothing to take back.
func TestLegendaryResistance_ASaveIsNotUndoable(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	if _, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(1))); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if id := a.log(t, a.master, e).GetUndoableEventId(); id != "" {
		t.Errorf("the log offers to undo %q after the save, want nothing to undo", id)
	}
	if got := a.resistanceSpent(t); got != 1 {
		t.Errorf("spent = %d, want 1", got)
	}
}

// RN-20, RN-29: the players read a save that succeeded and nothing else: the log line is the ordinary
// success (no roll, no word, no count), their view of the combat has no trace, and the master reads the
// roll, the choice and the uses left.
func TestRN20_AResistedEffectSaveReadsAsASuccessToThePlayers(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	if _, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(1))); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	var master *playv1.CombatLogEffect
	for _, l := range logEntries(a.log(t, a.master, e)) {
		if l.GetEffect().GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVED {
			master = l.GetEffect()
		}
	}
	if master == nil || !master.GetResisted() || master.GetResistedLeft() != 2 || master.GetD20() != 1 || master.GetTotal() >= master.GetDc() {
		t.Fatalf("the master's line = %v, want a save with the failed roll, the choice and 2 uses left (the positive control)", master)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		var line *playv1.CombatLogEffect
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetEffect().GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVED {
				line = l.GetEffect()
			}
		}
		if line == nil || line.Resisted || line.ResistedLeft != 0 || line.D20 != nil || line.Total != nil || line.Dc != nil {
			t.Errorf("%s reads the save as %v, want the ordinary line of a save that passed", who, line)
		}
		for what, js := range map[string]string{"the combat": asJSON(t, a.get(t, u)), "the log": asJSON(t, a.log(t, u, e))} {
			if low := strings.ToLower(js); strings.Contains(low, "legendary") || strings.Contains(low, "lendária") || strings.Contains(low, `"resisted`) {
				t.Errorf("%s reads Legendary Resistance in %s:\n%s", who, what, js)
			}
		}
	}
}

// The concentration line: the master reads the failed roll and the choice, a player the line of a
// concentration that was kept.
func TestRN20_AResistedConcentrationSaveReadsAsKeptToThePlayers(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	if _, err := a.resolveConcentration(t, a.master, e, w.GetId(), resistingConcentration(concentrationFace(1))); err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	line := func(u *user) string {
		var out string
		for _, r := range a.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
					out = en.GetReactionTextPt()
				}
			}
		}
		return out
	}
	want := "Draco falhou no teste de concentração em Teia (rolou 14 contra CD 24) e usou Resistência Lendária: sucesso (restam 2)."
	if got := line(a.master); got != want {
		t.Errorf("the master reads %q, want %q", got, want)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		if got := line(u); got != "Draco manteve a concentração em Teia." {
			t.Errorf("%s reads %q, want the line of a concentration kept", who, got)
		}
		if js := asJSON(t, a.get(t, u)); strings.Contains(strings.ToLower(js), "legendary") {
			t.Errorf("%s reads the trait in the combat:\n%s", who, js)
		}
	}
}

// The hints: a save the master made a success sends the players the hints of a save that passed, no more
// and no other, and the stream says nothing of the trait.
func TestRN20_AResistedSaveHintsThePlayersAsAPassedSaveDoes(t *testing.T) {
	t.Parallel()
	hints := func(set func(*playv1.RollEffectSaveRequest)) []*playv1.WatchGameSessionResponse {
		a, e, w := heldDragon(t)
		pw := a.caio.watch(t, a.campaignID)
		pw.ready(t)
		if _, err := a.rollEffectSave(t, a.master, e, w.GetId(), set); err != nil {
			t.Fatalf("RollEffectSave() error = %v", err)
		}
		a.master.markCurrentMap(t, a.campaignID, a.mapID, pw)
		return pw.beforeMarker(t)
	}
	resisted := hints(resisting(effectSaveFace(1)))
	passed := hints(effectSaveFace(20))
	if len(passed) == 0 {
		t.Fatal("a passed save sent the player no hint: the control is missing")
	}
	if len(resisted) != len(passed) {
		t.Errorf("a resisted save sent the player %d hints, a passed one %d", len(resisted), len(passed))
	}
	if js := strings.ToLower(asJSONAll(t, resisted)); strings.Contains(js, "legendary") || strings.Contains(js, "resist") {
		t.Errorf("the player's stream carries the trait: %s", js)
	}
}

// A concentration save sent twice spends once, and the retry answers what the first did: the failed roll made a
// success, with the uses left.
func TestLegendaryResistance_AConcentrationRetrySpendsOnceAndAnswersTheSame(t *testing.T) {
	t.Parallel()
	a, e, w := concentratingDragon(t)
	req := &playv1.ResolveConcentrationSaveRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(), UseLegendaryResistance: true,
		Roll: &playv1.ResolveConcentrationSaveRequest_D20Face{D20Face: 1},
	}
	var results []*playv1.ConcentrationSaveResult
	for range 2 {
		res, err := a.master.combat.ResolveConcentrationSave(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("ResolveConcentrationSave() error = %v", err)
		}
		results = append(results, res.Msg.GetResult())
	}
	if got := a.resistanceSpent(t); got != 1 {
		t.Errorf("after the same request twice the dragon spent %d uses, want 1", got)
	}
	if r := results[1]; !r.GetKept() || !r.GetResisted() || r.GetResistedLeft() != 2 {
		t.Errorf("the retry answers %v, want the first answer (kept by the trait, 2 left)", r)
	}
}

// A save the creature fails without a roll (a paralyzed creature fails Strength and Dexterity saves) is turned
// into a success too.
func TestLegendaryResistance_AnAutoFailedSaveBecomesASuccess(t *testing.T) {
	t.Parallel()
	a, e, w := heldDragon(t)
	a.execSQL(t, `UPDATE combatant_states SET end_save_ability = 'dex' WHERE encounter_id = $1 AND kind = 'effect'`, e.GetId())
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(effectSaveFace(20)))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r := res.GetResult(); !r.GetAutoFail() || !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 {
		t.Errorf("the result = %v, want an automatic failure made a success with 2 left", r)
	}
	if got := a.resistanceSpent(t); got != 1 || a.holdsTheDragon(t) {
		t.Errorf("spent = %d, held = %v; want 1 use and the hold ended", got, a.holdsTheDragon(t))
	}
}

// With no roll to resist (a skipped effect save, a concentration the master keeps) the choice is ignored, also
// for a creature without the trait: nothing is refused and nothing is spent.
func TestLegendaryResistance_TheChoiceIsIgnoredWhereThereIsNoRoll(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t) // Toren has no trait
	skip := func(r *playv1.RollEffectSaveRequest) { r.Roll = &playv1.RollEffectSaveRequest_Skip{Skip: true} }
	if _, err := a.rollEffectSave(t, a.master, e, w.GetId(), resisting(skip)); err != nil {
		t.Errorf("a skipped save with the choice error = %v, want it ignored", err)
	}
	b, f := concentratingHit(t, 6)
	cw := b.windowOf(t, b.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if cw == nil {
		t.Fatal("no concentration window")
	}
	if _, err := b.resolveConcentration(t, b.master, f, cw.GetId(), resistingConcentration(keepConcentration)); err != nil {
		t.Errorf("a kept concentration with the choice error = %v, want it ignored", err)
	}
}
