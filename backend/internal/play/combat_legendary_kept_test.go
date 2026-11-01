package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// keptFight is a cast at the dragon Draco and a co-target that does not have the trait, both east of the wizard:
// the dragon fails (a 1), the co-target's dice are the test's. It answers the window with Draco's failure turned
// and says the dice the replay rolled (none, or the test fails).
func keptFight(t *testing.T, race, coTarget string, prepare func(a *armed, e *playv1.Encounter), faces ...int) (*armed, *playv1.Encounter) {
	t.Helper()
	a, e := resistFightOf(t, race, "Draco")
	a.put(t, "Draco", 8, 4)
	a.put(t, coTarget, 8, 5)
	if prepare != nil {
		prepare(a, e)
	}
	a.h.roller.queue(faces...)
	if _, err := a.holdAt(t, "Draco", coTarget); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	asked := a.h.roller.taken()
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("no window: the dragon's save did not fail (%v)", a.windows(t, a.master))
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if got := a.h.roller.taken(); got != asked {
		t.Errorf("the replay rolled %d dice more, want none", got-asked)
	}
	return a, e
}

// A co-target that saves with disadvantage keeps both d20 and the lower one counts, as it was rolled.
func TestLegendaryResistance_ACoTargetWithDisadvantageKeepsBothDice(t *testing.T) {
	t.Parallel()
	a, _ := keptFight(t, "", "Goblin", func(a *armed, e *playv1.Encounter) { a.setConditions(t, e, "Goblin", "condition:restrained") }, 20, 3, 1)
	save := a.castSaveOf(t, a.master, a.get(t, a.master), "Goblin")
	if save.GetRoll().GetFaces()[0] != 20 || save.GetRoll().GetFaces()[1] != 3 || save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Errorf("the goblin's save = %v, want the 20 and the 3 kept, the 3 counting, and failed", save)
	}
	if got := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco"); !got.GetResisted() || got.GetRoll().GetFaces()[0] != 1 {
		t.Errorf("the dragon's save = %v, want the 1 kept and turned", got)
	}
}

// A halfling player's character that rolls a 1 keeps the reroll: the replay does not roll Lucky again.
func TestLegendaryResistance_ALuckyCoTargetKeepsItsReroll(t *testing.T) {
	t.Parallel()
	a, _ := keptFight(t, "race:halfling", "Toren", nil, 1, 15, 1)
	save := a.castSaveOf(t, a.master, a.get(t, a.master), "Toren")
	if len(save.GetRoll().GetRerolled()) == 0 || save.GetRoll().GetFaces()[0] != 15 {
		t.Errorf("Toren's save = %v, want the reroll kept: a 1 rolled again to 15", save)
	}
}

// Bane's d4 on a co-target's save is rolled once: the die is kept and takes the 15 under the DC.
func TestLegendaryResistance_ACoTargetsBaneDieIsKept(t *testing.T) {
	t.Parallel()
	a, _ := keptFight(t, "", "Goblin", func(a *armed, e *playv1.Encounter) {
		a.mustAddEffect(t, e, "spell:bane", []string{"Goblin"}, a.rounds(t, 10, "Goblin"))
	}, 15, 4, 1)
	save := a.castSaveOf(t, a.master, a.get(t, a.master), "Goblin")
	if save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || save.GetRoll().GetTotal() != 11 {
		t.Errorf("the goblin's save = %v, want a total of 11 (15 - 4) and failed", save)
	}
}

// keptDice lands the roll the hold kept and refuses a roll that does not fit the save.
func TestKeptDice(t *testing.T) {
	t.Parallel()
	d4 := effectDie{Key: "spell:bane", Faces: 4, Sign: -1}
	tests := []struct {
		name    string
		kept    *saveRoll
		mode    combat.RollMode
		effects []effectDie
		ok      bool
		index   int
		total   int
		extra   int
	}{
		{"a normal d20", &saveRoll{D20: 12}, combat.ModeNormal, nil, true, 0, 14, 0},
		{"disadvantage keeps the pair and the counted die", &saveRoll{D20: 3, D20B: 20, Counted: 0}, combat.ModeDisadvantage, nil, true, 0, 5, 0},
		{"the second die counted", &saveRoll{D20: 15, D20B: 2, Counted: 1}, combat.ModeDisadvantage, nil, true, 1, 17, 0},
		{"an effect die", &saveRoll{D20: 15, Extra: []effectDie{{Face: 4}}}, combat.ModeNormal, []effectDie{d4}, true, 0, 17, -4},
		{"nothing kept", nil, combat.ModeNormal, nil, false, 0, 0, 0},
		{"an automatic failure", &saveRoll{Auto: true}, combat.ModeNormal, nil, false, 0, 0, 0},
		{"one die for a pair", &saveRoll{D20: 9}, combat.ModeAdvantage, nil, false, 0, 0, 0},
		{"a pair for one die", &saveRoll{D20: 9, D20B: 4}, combat.ModeNormal, nil, false, 0, 0, 0},
		{"an effect die missing", &saveRoll{D20: 9}, combat.ModeNormal, []effectDie{d4}, false, 0, 0, 0},
	}
	for _, tt := range tests {
		got, ok := keptDice(tt.kept, 2, tt.mode, tt.effects)
		if ok != tt.ok {
			t.Errorf("%s: ok = %v, want %v", tt.name, ok, tt.ok)
			continue
		}
		if ok && (got.d20.Index != tt.index || got.d20.Total != tt.total || got.extraTotal != tt.extra) {
			t.Errorf("%s: dice = %+v, want index %d, total %d, extra %d", tt.name, got, tt.index, tt.total, tt.extra)
		}
	}
}

// The window an NPC-caster's cast would not open, and the closing text of the master's own window.
func TestLegendaryResistance_AClosedWindowNamesNoReactor(t *testing.T) {
	t.Parallel()
	n := reactionNote{window: playdb.ReactionWindow{Kind: string(reaction.LegendaryResistance)}, reason: reaction.ReasonTriggerGone, byItself: true}
	if got, want := closedText(n, playdb.Combatant{}, false, true), "A Resistência Lendária fechou: o gatilho não vale mais."; got != want {
		t.Errorf("closedText() = %q, want %q", got, want)
	}
}

// A spell the master casts for an NPC wizard at the dragon is no player's cast: the dragon's failed save stands
// and nothing waits.
func TestLegendaryResistance_AnNPCsCastOpensNoWindow(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	mago := a.master.fullNPC(t, a.campaignID, "Mago", "race:human", "class:wizard", 3, abilities(10, 14, 12, 16, 10), nil)
	spells := []string{burningHands}
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(10, 14, 12, 16, 10), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 3}},
		KnownSpellKeys: spells, PreparedSpellKeys: spells,
	}}}
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: mago.GetId(), Revision: mago.GetRevision(), Name: mago.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: mago.GetId()}}, npcRolls: []int{20},
		players: map[string]int32{"Pensantus": 1, "Toren": 1, "Brisa": 1},
		reveal:  []string{"Mago"}, setup: true,
		at: map[string][2]int32{"Mago": {5, 5}, "Pensantus": {1, 1}, "Toren": {6, 1}, "Brisa": {4, 1}},
	})
	a.h.roller.queue(1)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = adultRedDragon, 1, "Draco", new(false)
	})
	a.begin(t, a.get(t, a.master))
	a.put(t, "Draco", 8, 5)
	a.h.roller.queue(1)
	if _, err := a.cast(t, a.master, a.get(t, a.master), "Mago", burningHands, slotOfLevel(1), a.at(t, "Draco"), noCastRoll); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if w := a.windowOf(t, a.master, resistKind); w != nil {
		t.Errorf("an NPC's cast opened %v, want no window", w)
	}
	if save := a.castSaveOf(t, a.master, a.get(t, a.master), "Draco"); save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || save.GetResisted() {
		t.Errorf("the dragon's save = %v, want failed and not turned", save)
	}
}

// A hold whose kept save no longer fits (here its d20 is gone) cannot be replayed: the master's answer drops the
// hold with nothing spent, the cast leaves no trace and the window is closed, so he is not stuck.
func TestLegendaryResistance_AHoldThatCannotBeReplayedIsDropped(t *testing.T) {
	t.Parallel()
	a, e := resistFight(t, "Draco")
	a.h.roller.queue(1)
	if _, err := a.holdAt(t, "Draco"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE reaction_holds SET data = jsonb_set(data::JSONB, '{saves,0,roll,d20}', '0')::JSONB WHERE state = 'held'`); err != nil {
		t.Fatalf("corrupt the hold: %v", err)
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v, want the hold dropped and the answer taken", err)
	}
	var held, dropped, open int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM reaction_holds WHERE state = 'held'), (SELECT count(*) FROM reaction_holds WHERE state = 'dropped'), (SELECT count(*) FROM reaction_windows WHERE status = 'open')`).Scan(&held, &dropped, &open); err != nil || held != 0 || dropped != 1 || open != 0 {
		t.Errorf("held = %d, dropped = %d, open windows = %d (%v); want 0, 1, 0", held, dropped, open, err)
	}
	if a.resistSpentBy(t, "Draco") != 0 || usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Error("the dropped cast left a spend")
	}
	if _, found := a.damageTo(t, "Draco"); found {
		t.Error("the dropped cast left a damage")
	}
	if a.windowOf(t, a.master, resistKind) != nil {
		t.Error("the window is still open: the master is stuck")
	}
}

// The save of a creature a spell zone catches is rolled by the server at once, with no choice for the master: a
// dragon inside the web when it is cast that fails it is restrained, no window opens and no use is spent.
func TestLegendaryResistance_AZoneSaveIsNotOffered(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.h.roller.queue(1)
	c.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = adultRedDragon, 1, "Draco", new(false)
	})
	c.mustMove(t, c.master, "Draco", 21, 6) // outside the cube of columns 17 to 20, rows 2 to 5
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", webKey, slotOfLevel(2), at(18, 3), nil)
	c.passTo(t, c.get(t, c.master), "Draco")
	c.h.roller.queue(1)
	if _, err := c.move(t, c.master, "Draco", 20, 5); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	if ws := c.windows(t, c.master); len(ws) != 0 {
		t.Errorf("the master reads the windows %v, want none: the zone's save is rolled at once", ws)
	}
	if cardOf(t, byLabel(t, c.get(t, c.master), "Draco"), "effect:web-restrained") == nil || c.resistSpentBy(t, "Draco") != 0 {
		t.Errorf("the dragon is not restrained or spent %d uses, want the failure to stand", c.resistSpentBy(t, "Draco"))
	}
}
