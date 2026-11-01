package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The saving throw of a spell zone against a creature with Legendary Resistance (SRD 5.1, "Legendary Resistance"):
// a creature the area catches that has the trait and a use left is asked of the master, as a player's character is
// asked of its player, and the master's answer may name the trait. The lich (Constitution save +10, Dexterity +3,
// Legendary Resistance 3/Day) stands in Moonbeam (Constitution save DC 13, 2d10 radiant, half on a pass) and in
// Grease (Dexterity save DC 15 from the wizard, prone on a failure). These tests need the database.

const lichKey = "monster:lich"

// bossFight is the cave with the creature of the key, named label, revealed at (8,7), between Toren and the ogre. The ogre
// and the goblin stand far to the east, out of the way of the areas.
func (c *cave) bossFight(t *testing.T, key, label string) *playv1.Encounter {
	t.Helper()
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.ogre.GetId()}, {CharacterId: c.goblins.GetId()}},
		npcRolls: []int{2, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		reveal:   []string{"Ogro", "Goblin"},
		at:       map[string][2]int32{"Toren": {5, 7}, "Pensantus": {12, 7}, "Brisa": {3, 7}, "Ogro": {16, 7}, "Goblin": {20, 7}},
		setup:    true,
	})
	c.h.roller.queue(3)
	c.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = key, 1, label, new(false)
	})
	c.begin(t, c.get(t, c.master))
	return c.mustMove(t, c.master, label, 8, 7)
}

// lichFight is bossFight with the lich, "Lich".
func (c *cave) lichFight(t *testing.T) *playv1.Encounter { return c.bossFight(t, lichKey, "Lich") }

// spentBy is the uses of Legendary Resistance the creature has spent.
func (c *cave) spentBy(t *testing.T, label string) int {
	t.Helper()
	return c.spentOf(t, c.id(t, label))[spentLegendaryResistance]
}

// lichSpent is the uses of Legendary Resistance the lich has spent.
func (c *cave) lichSpent(t *testing.T) int { t.Helper(); return c.spentBy(t, "Lich") }

// lichInMoonbeam has Toren cast Moonbeam on the lich and the turn pass to the lich: the zone catches it at the start
// of its turn. It returns the master's question for the lich's save.
func (c *cave) lichInMoonbeam(t *testing.T) *playv1.ReactionWindow {
	t.Helper()
	e := c.lichFight(t)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(8, 7), nil)
	c.declineCounterspell(t)
	c.endTurnsUntilAsked(t, e)
	w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the lich in the beam is not asked: windows = %v", c.windows(t, c.master))
	}
	return w
}

// endTurnsUntilAsked ends the turns, as the master, until a saving throw is asked: the turn that begins waits for it.
func (c *cave) endTurnsUntilAsked(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	for range 6 {
		if c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) != nil {
			return
		}
		if _, err := c.endTurnRaw(t, c.master, e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
	}
}

// declineCounterspell has the master leave the lich's Counterspell (a spell of its stat block) unused.
func (c *cave) declineCounterspell(t *testing.T) {
	t.Helper()
	w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
	if w == nil {
		t.Fatal("the lich is not offered Counterspell")
	}
	if _, err := c.answerReaction(t, c.master, c.get(t, c.master), w.GetId(), func(r *playv1.AnswerReactionRequest) { r.Answer = playv1.ReactionChoice_REACTION_CHOICE_PASS }); err != nil {
		t.Fatalf("AnswerReaction(Counterspell) error = %v", err)
	}
}

// answerBoss answers a boss's save as the master with a typed d20 and the choice of the trait.
func (c *cave) answerBoss(t *testing.T, w *playv1.ReactionWindow, face int32, use bool) (*playv1.EffectSaveResult, error) {
	t.Helper()
	res, err := c.rollEffectSave(t, c.master, c.get(t, c.master), w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		effectSaveFace(face)(r)
		r.UseLegendaryResistance = use
	})
	if err != nil {
		return nil, err
	}
	return res.GetResult(), nil
}

// The lich the beam catches is asked of the master and nothing is rolled by the server: no player reads the question, the
// hit points and the uses stay as they were until the master answers.
func TestAZoneAsksTheMasterForACreatureWithLegendaryResistance(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	if w.GetReactorId() != c.id(t, "Lich") || w.GetEffectSave().GetZoneId() == "" || w.GetEffectSave().GetZoneReason() != zoneWhyStart {
		t.Errorf("the master's window = %v, want the save of the zone for the lich at the start of its turn", w)
	}
	for _, u := range []*user{c.ana, c.caio, c.bia} {
		if got := c.windows(t, u); len(got) != 0 {
			t.Errorf("a player reads the windows %v: the lich's question is the master's alone", got)
		}
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135 {
		t.Errorf("the lich's hit points = %d, want 135 until the master answers", cur)
	}
	if got := c.lichSpent(t); got != 0 {
		t.Errorf("the lich spent %d uses before the answer, want 0", got)
	}
}

// The master turns the failed roll into a success: one use is spent, the lich takes half of the beam, and the
// master reads the roll that failed with the uses left.
func TestAMasterUsesLegendaryResistanceOnTheFailedSaveOfAZone(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	c.h.roller.queue(5, 5) // 2d10 = 10, half 5
	r, err := c.answerBoss(t, w, 1, true)
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 || r.GetD20() != 1 || r.GetTotal() >= r.GetDc() {
		t.Errorf("the master's result = %v, want the failed 1 made a success with 2 uses left", r)
	}
	if got := c.lichSpent(t); got != 1 {
		t.Errorf("the lich spent %d uses, want 1", got)
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135-5 {
		t.Errorf("the lich's hit points = %d, want 130: a save made takes half of 2d10 = 10", cur)
	}
	if w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE); w != nil {
		t.Errorf("the question is still open after the answer: %v", w)
	}
}

// A failed roll with no choice of the trait is a failure: the whole beam, nothing spent.
func TestAFailedSaveOfAZoneWithoutTheChoiceTakesTheWholeDamage(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	c.h.roller.queue(5, 5)
	r, err := c.answerBoss(t, w, 1, false)
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r.GetSaved() || r.GetResisted() {
		t.Errorf("the result = %v, want a failed save", r)
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135-10 {
		t.Errorf("the lich's hit points = %d, want 125: the whole 2d10 = 10", cur)
	}
	if got := c.lichSpent(t); got != 0 {
		t.Errorf("the lich spent %d uses without the master's choice, want 0", got)
	}
}

// A roll that passes spends nothing, and the choice is ignored.
func TestARollThatPassesAZoneSaveSpendsNothing(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	c.h.roller.queue(5, 5)
	r, err := c.answerBoss(t, w, 20, true)
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if !r.GetSaved() || r.GetResisted() {
		t.Errorf("the result = %v, want a save that passed by itself", r)
	}
	if got := c.lichSpent(t); got != 0 {
		t.Errorf("the lich spent %d uses on a save it passed, want 0", got)
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135-5 {
		t.Errorf("the lich's hit points = %d, want 130: half of 10", cur)
	}
}

// The answer sent twice spends once, and the retry answers what the first did.
func TestARetryOfTheAnswerToAZoneSaveSpendsOnce(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	e := c.get(t, c.master)
	c.h.roller.queue(5, 5)
	req := &playv1.RollEffectSaveRequest{
		CampaignId: c.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(), UseLegendaryResistance: true,
		Roll: &playv1.RollEffectSaveRequest_D20Face{D20Face: 2},
	}
	var results []*playv1.EffectSaveResult
	for range 2 {
		res, err := c.master.lasting.RollEffectSave(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("RollEffectSave() error = %v", err)
		}
		results = append(results, res.Msg.GetResult())
	}
	if got := c.lichSpent(t); got != 1 {
		t.Errorf("after the same request twice the lich spent %d uses, want 1", got)
	}
	if r := results[1]; !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 || r.GetD20() != 2 {
		t.Errorf("the retry answers %v, want the first answer (a failed 2 made a success, 2 left)", r)
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135-5 {
		t.Errorf("the lich's hit points = %d, want 130: the damage landed once", cur)
	}
}

// A creature with the trait and no use left is rolled by the server at once, as any NPC is: nothing is asked.
func TestAZoneRollsAtOnceForACreatureWithNoUseLeft(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	e := c.lichFight(t)
	c.execSQL(t, `UPDATE combatants SET creature_spent = '{"legendary_resistance":3}' WHERE id = $1`, c.id(t, "Lich"))
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(8, 7), nil)
	c.declineCounterspell(t)
	c.h.roller.queue(1, 5, 5) // the lich's Constitution save: 1 + 10 < 13, then 2d10 = 10
	c.passTo(t, e, "Lich")
	if w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE); w != nil {
		t.Fatalf("the master is asked the save of a lich with no use left: %v", w)
	}
	if cur, _, _ := c.hp(t, "Lich"); cur != 135-10 {
		t.Errorf("the lich's hit points = %d, want 125: it failed the save the server rolled", cur)
	}
}

// A creature without the trait is rolled at once too: the question of the master is the trait's alone.
func TestAZoneRollsAtOnceForACreatureWithoutTheTrait(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	e := c.lichFight(t)
	c.mustMove(t, c.master, "Ogro", 14, 7)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(14, 7), nil)
	c.declineCounterspell(t)
	c.h.roller.queue(1, 5, 5) // the ogre's Constitution save, then 2d10 = 10
	c.passTo(t, e, "Ogro")
	if w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE); w != nil {
		t.Fatalf("the master is asked the save of an ogre: %v", w)
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-10 {
		t.Errorf("the ogre's hit points = %d, want 49", cur)
	}
}

// The vampire in the grease at the cast is asked inside the caster's request; the master uses the trait, the vampire stays on
// its feet, and the undo of the cast gives the use back with the zone.
func TestGreaseAtTheCastAsksTheMasterAndTheUndoGivesTheUseBack(t *testing.T) {
	t.Parallel()
	c := greaseCave(t)
	c.passTo(t, c.bossFight(t, vampireKey, "Vampiro"), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", greaseKey, slotOfLevel(1), at(8, 7), nil)
	w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the vampire in the grease is not asked at the cast: windows = %v", c.windows(t, c.master))
	}
	if got := c.windows(t, c.ana); len(got) != 0 {
		t.Errorf("the caster reads the windows %v: the vampire's question is the master's alone", got)
	}
	r, err := c.answerBoss(t, w, 1, true)
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if !r.GetSaved() || !r.GetResisted() || r.GetResistedLeft() != 2 {
		t.Errorf("the master's result = %v, want the failed 1 made a success with 2 left", r)
	}
	if c.isProne(t, "Vampiro") {
		t.Error("the vampire fell although the master made its save a success")
	}
	if got := c.spentBy(t, "Vampiro"); got != 1 {
		t.Fatalf("the vampire spent %d uses, want 1", got)
	}
	c.undoLast(t)
	if got := c.spentBy(t, "Vampiro"); got != 0 {
		t.Errorf("after the undo of the cast the vampire spent %d uses, want 0: the use goes back with the cast", got)
	}
	if zs := c.get(t, c.master).GetZones(); len(zs) != 0 {
		t.Errorf("zones after the undo = %v, want none", zs)
	}
}

// The same cast with no choice of the trait: the vampire falls prone, nothing is spent, and the undo stands it up.
func TestGreaseAtTheCastKnocksDownTheLichTheMasterLetsFail(t *testing.T) {
	t.Parallel()
	c := greaseCave(t)
	c.passTo(t, c.bossFight(t, vampireKey, "Vampiro"), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", greaseKey, slotOfLevel(1), at(8, 7), nil)
	w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatal("the vampire in the grease is not asked at the cast")
	}
	if _, err := c.answerBoss(t, w, 1, false); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if !c.isProne(t, "Vampiro") || c.spentBy(t, "Vampiro") != 0 {
		t.Errorf("vampire prone = %v, spent = %d, want prone with nothing spent", c.isProne(t, "Vampiro"), c.spentBy(t, "Vampiro"))
	}
	c.undoLast(t)
	if c.isProne(t, "Vampiro") || c.spentBy(t, "Vampiro") != 0 {
		t.Errorf("after the undo: vampire prone = %v, spent = %d, want standing with nothing spent", c.isProne(t, "Vampiro"), c.spentBy(t, "Vampiro"))
	}
}

// A player may not answer the lich's question, and is refused as for a window that does not exist.
func TestAPlayerCannotAnswerTheLichsZoneSave(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	w := c.lichInMoonbeam(t)
	e := c.get(t, c.master)
	_, ask := c.rollEffectSave(t, c.caio, e, w.GetId(), effectSaveFace(20))
	_, fake := c.rollEffectSave(t, c.caio, e, newKey(), effectSaveFace(20))
	if ask == nil || fake == nil || connect.CodeOf(ask) != connect.CodeOf(fake) || ask.Error() != fake.Error() {
		t.Errorf("a player's answer to the lich's window: %v, to an invented one: %v; want the same refusal", ask, fake)
	}
	if !slices.ContainsFunc(c.get(t, c.master).GetReactionWindows(), func(o *playv1.ReactionWindow) bool { return o.GetId() == w.GetId() }) {
		t.Error("the question is gone after a player's refused answer")
	}
}

// A lich the players do not see is asked like any other: the question is the master's, the players read neither the
// creature nor its window, and the line of the save keeps it hidden from them.
func TestAZoneAsksTheMasterForAHiddenLichAndNamesItToNoPlayer(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	e := c.lichFight(t)
	if _, err := c.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: c.campaignID, EncounterId: e.GetId(), CombatantId: c.id(t, "Lich"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(8, 7), nil)
	c.declineCounterspell(t)
	c.endTurnsUntilAsked(t, e)
	w := c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatal("the hidden lich in the beam is not asked")
	}
	c.h.roller.queue(5, 5)
	if _, err := c.answerBoss(t, w, 1, true); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	for _, u := range []*user{c.ana, c.caio, c.bia} {
		enc := c.get(t, u)
		if len(enc.GetReactionWindows()) != 0 || slices.ContainsFunc(enc.GetCombatants(), func(o *playv1.Combatant) bool { return o.GetLabel() == "Lich" }) {
			t.Errorf("a player reads the hidden lich or its question: %v", enc)
		}
		for _, r := range c.log(t, u, c.get(t, c.master)).GetRounds() {
			for _, en := range r.GetEntries() {
				if slices.Contains(en.GetEffect().GetTargetLabels(), "Lich") || en.GetTargetLabel() == "Lich" {
					t.Errorf("a player's log names the hidden lich: %v", en)
				}
			}
		}
	}
}
