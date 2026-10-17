package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The creatures' tests freeze a rule the unified gate has to keep. The four tests that followed them, which froze what
// the two hold tables did only because of the way they were written (the master's side of the held rolls), were
// flipped on purpose by E4-7, when the master got to answer every question: the older RPCs still behave as they
// did, and AnswerHeldRoll, the questions on the combatants and ListHeldRolls say what changed.

// giftedWolfBite is the monster the master gives Brisa, a wolf called Presa, as the creature of her character.
const giftedWolfBite = "monster:wolf#bite"

// giftedFight starts a combat in a theatre of the mind (no reach, no disadvantage) of Brisa, the cleric of
// Bia's, and Presa, the wolf the master gave her: the creature carries Brisa's character_id. The d20 given is
// Presa's initiative, so that she plays before Brisa (20) or after her (1).
func giftedFight(t *testing.T, wolfFace int32) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCasters(t)
	a.give(t, a.bri, "monster:wolf", "Presa")
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 12, "Pensantus": 10, "Brisa": 15},
		reveal:   []string{"Goblin"},
		theatre:  true,
		setup:    true,
	})
	if _, err := a.submitFor(t, a.bia, e, "Presa", wolfFace); err != nil {
		t.Fatalf("SubmitInitiative(Presa) error = %v", err)
	}
	begun, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()}))
	if err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	return a, begun.Msg.GetEncounter()
}

// holdWolfBite has the wolf bite the goblin with a die in hand: the attack waits in roll_holds.
func (a *armed) holdWolfBite(t *testing.T, e *playv1.Encounter) *playv1.RollAttackResponse {
	t.Helper()
	a.passTo(t, e, "Presa")
	a.giveCombatantDie(t, "Presa", a.get(t, a.master))
	held, err := a.attack(t, a.bia, a.get(t, a.master), "Presa", giftedWolfBite, "Goblin", d20(9))
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("the wolf's bite = %v, %v; want the question about its die", held, err)
	}
	return held
}

// holdGuidingBolt has Brisa cast Guiding Bolt with a die in hand: the cast waits in inspiration_holds.
func (a *armed) holdGuidingBolt(t *testing.T, e *playv1.Encounter) *playv1.CastSpellResponse {
	t.Helper()
	a.passTo(t, e, "Brisa")
	a.giveCombatantDie(t, "Brisa", a.get(t, a.master))
	held, err := a.cast(t, a.bia, a.get(t, a.master), "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"),
		func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 9} })
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("Brisa's Guiding Bolt = %v, %v; want the question about her die", held, err)
	}
	return held
}

// answerWolf answers the wolf's question, keeping the die.
func (a *armed) answerWolf(t *testing.T, e *playv1.Encounter, wolf *playv1.RollAttackResponse) {
	t.Helper()
	bite, err := a.answer(t, a.bia, e, wolf.GetInspirationOffer().GetHoldId(), keepTheDie)
	if err != nil || bite.GetAttack().GetRoll().GetD20().GetFaces()[0] != 9 {
		t.Errorf("the wolf's answer = %v, %v; want its bite settled on the d20 of 9", bite.GetAttack().GetRoll(), err)
	}
}

// answerBolt answers Brisa's question, keeping the die.
func (a *armed) answerBolt(t *testing.T, bolt *playv1.CastSpellResponse) {
	t.Helper()
	cast, err := a.answerOutside(t, a.bia, bolt.GetInspirationOffer().GetHoldId(), false, false, 0)
	if err != nil || cast.GetSpellCast().GetCast().GetTargets()[0].GetAttackRoll().GetFaces()[0] != 9 {
		t.Errorf("Brisa's answer = %v, %v; want her Guiding Bolt settled on the d20 of 9", cast.GetSpellCast().GetCast(), err)
	}
}

// wantTwoQuestions checks that both questions are open at once: the wolf's on its combatant and Brisa's
// on her player's list. The answer of the earlier turn's is not asked here: a roll is answered on its
// own combatant's turn.
func (a *armed) wantTwoQuestions(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	if n := countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId()); n != 1 {
		t.Errorf("%d held attacks, want the wolf's", n)
	}
	if n := countRows(t, a.h, countInspirationHold, a.campaignID); n != 2 {
		t.Errorf("%d held rolls in the table, want the wolf's attack and her cast", n)
	}
	if got := a.outsideState(t, a.bia).GetOffers(); len(got) != 1 {
		t.Errorf("Brisa's player reads %d questions of hers, want 1", len(got))
	}
}

// TestACreaturesHeldAttackDoesNotBlockItsOwner: a creature carries its owner's character_id, but it is
// a combatant of its own and not "backed by" the character: the wolf's bite that waits for its die,
// in roll_holds, does not stop Brisa's own roll in inspiration_holds, and both questions are open at once.
func TestACreaturesHeldAttackDoesNotBlockItsOwner(t *testing.T) {
	t.Parallel()
	a, e := giftedFight(t, 20) // the wolf plays before her
	wolf := a.holdWolfBite(t, e)
	bolt := a.holdGuidingBolt(t, e)
	a.wantTwoQuestions(t, e)
	a.answerBolt(t, bolt)
	if c := byLabel(t, a.get(t, a.master), "Presa"); c.GetInspirationOffer().GetHoldId() != wolf.GetInspirationOffer().GetHoldId() {
		t.Errorf("the wolf's question = %v after Brisa's answer, want it still open", c.GetInspirationOffer())
	}
}

// TestACreaturesHeldAttackDoesNotBlockItsOwnerReverse is the other way round: Brisa's cast waits
// in inspiration_holds (her turn comes first), and the wolf's bite is not refused for it.
func TestACreaturesHeldAttackDoesNotBlockItsOwnerReverse(t *testing.T) {
	t.Parallel()
	a, e := giftedFight(t, 1) // the wolf plays after her
	bolt := a.holdGuidingBolt(t, e)
	wolf := a.holdWolfBite(t, e)
	a.wantTwoQuestions(t, e)
	a.answerWolf(t, e, wolf)
	if got := a.outsideState(t, a.bia).GetOffers(); len(got) != 1 || got[0].GetHoldId() != bolt.GetInspirationOffer().GetHoldId() {
		t.Errorf("Brisa's player reads %v after the wolf's answer, want her question still open", got)
	}
}

// TestTheMasterAnswersAnOutsideHoldOnlyByAnswerHeldRoll: the question of a player's roll that waits in
// inspiration_holds (the scene check, the group check, the trap search, the hint try, a spell attack and the saves in
// a combat) is answered by its player and by the master, with AnswerHeldRoll, the die's and Lucky's; the older RPCs
// that answer these rolls stay the player's alone (the master's answer is refused with permission_denied), and
// another player is told the hold is not found, by every RPC.
func TestTheMasterAnswersAnOutsideHoldOnlyByAnswerHeldRoll(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:athletics", dc: new(int32(25))})
	a.openScene(t, point)
	if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
		t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
	}
	// The die's question (a 14): the older RPC refuses the master, another player is not told it exists, the master answers
	// with AnswerHeldRoll and the player's own answer is then late.
	held, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 14)
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("RollSceneCheck() = %v, %v; want the die's question", held, err)
	}
	hold := held.GetInspirationOffer().GetHoldId()
	_, err = a.answerOutside(t, a.master, hold, false, false, 0)
	wantCode(t, "the master's answer to the die, by the older RPC", err, connect.CodePermissionDenied)
	_, err = a.answerHeldRoll(t, a.bia, hold, dieAnswer(false))
	wantCode(t, "another player's answer", err, connect.CodeNotFound)
	res, err := a.answerHeldRoll(t, a.master, hold, dieAnswer(false))
	if err != nil || res.GetSceneCheck().GetRoll() == nil || res.GetNext() != nil {
		t.Fatalf("the master's answer to the die = %v, %v; want the scene check written", res, err)
	}
	if _, err := a.answerOutside(t, dalila, hold, false, false, 0); err == nil {
		t.Error("the player's answer after the master's went through, want it late")
	}
	// Lucky's question (a 1, with the die still in her hand): the older RPC refuses the master; AnswerHeldRoll lets it roll again.
	lucky, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 1)
	if err != nil || lucky.GetLuckyOffer() == nil {
		t.Fatalf("RollSceneCheck() = %v, %v; want Lucky's question", lucky, err)
	}
	luckyHold := lucky.GetLuckyOffer().GetHoldId()
	_, err = answerOutsideLucky(t, a.master, a.campaignID, luckyHold, outsideRerollTyped(15))
	wantCode(t, "the master's answer to Lucky, by the older RPC", err, connect.CodePermissionDenied)
	res, err = a.answerHeldRoll(t, a.master, luckyHold, luckyAnswer(15))
	if err != nil {
		t.Fatalf("the master's answer to Lucky: error = %v", err)
	}
	// The roll waits for the die's question next (the character still holds it), on the new d20, as for the player.
	if next := res.GetNext(); next == nil || next.GetDie() == nil || next.GetD20().GetFaces()[0] != 15 || next.GetHoldId() != luckyHold {
		t.Errorf("the next question = %v, want the die's, on the d20 of 15", res.GetNext())
	}
	if _, err := a.answerHeldRoll(t, dalila, luckyHold, dieAnswer(false)); err != nil {
		t.Errorf("the player's answer to the die after the master's Lucky: error = %v", err)
	}
}

// TestAnOutsideHoldInACombatIsOnItsCombatantForItsPlayerAndTheMaster: a spell attack that waits for its die in a combat is
// on the combatant in the one shape every held roll has, for the player and the master, and for nobody else; the fields
// of a weapon attack's question (inspiration_offer, lucky_offer) stay empty, and the player's own list still has it.
func TestAnOutsideHoldInACombatIsOnItsCombatantForItsPlayerAndTheMaster(t *testing.T) {
	t.Parallel()
	a, e := brisaFight(t, true)
	a.h.roller.queue(4, 13)
	held, err := a.castGuidingBolt(t, e)
	if err != nil || held.GetInspirationOffer() == nil || held.GetHeldRollOffer() == nil {
		t.Fatalf("CastSpell() = %v, %v; want the die's question, in both shapes", held, err)
	}
	hold := held.GetInspirationOffer().GetHoldId()
	if o := held.GetHeldRollOffer(); o.GetHoldId() != hold || o.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_SPELL_ATTACK || o.GetDie() == nil ||
		o.GetRollerLabel() != "Brisa" || o.GetCombatantId() != a.id(t, "Brisa") || o.GetCharacterId() != a.bri.GetId() || o.GetCreatedAt() == nil {
		t.Errorf("the answer's offer = %v, want Brisa's spell attack's die question", o)
	}
	for name, u := range map[string]*user{"Bia": a.bia, "master": a.master} {
		c := byLabel(t, a.get(t, u), "Brisa")
		if c.GetInspirationOffer() != nil || c.GetLuckyOffer() != nil {
			t.Errorf("%s reads %v and %v in the weapon attack's fields on Brisa, want none", name, c.GetInspirationOffer(), c.GetLuckyOffer())
		}
		if o := c.GetHeldRollOffer(); o.GetHoldId() != hold || o.GetDie().GetSides() != 8 {
			t.Errorf("%s reads %v on Brisa, want the question %s about a d8", name, o, hold)
		}
	}
	for name, u := range map[string]*user{"Ana": a.ana, "Caio": a.caio} {
		if o := byLabel(t, a.get(t, u), "Brisa").GetHeldRollOffer(); o != nil {
			t.Errorf("%s reads %v on Brisa, want nothing: it is not their roll (RN-10)", name, o)
		}
	}
	if got := a.outsideState(t, a.bia).GetOffers(); len(got) != 1 || got[0].GetHoldId() != hold {
		t.Errorf("Bia reads the offers %v, want the question %s", got, hold)
	}
}

// TestAHoldInACombatTouchesTheEncounter: the question of any roll held in a combat moves the combat's revision (the
// player and the master read the combat again to see it).
func TestAHoldInACombatTouchesTheEncounter(t *testing.T) {
	t.Parallel()
	t.Run("weapon attack", func(t *testing.T) {
		t.Parallel()
		a, e := brisaFight(t, true)
		a.h.roller.queue(4)
		before := a.get(t, a.master).GetRevision()
		if _, err := a.attack(t, a.bia, e, "Brisa", maceKey, "Goblin", inAppRoll); err != nil {
			t.Fatalf("RollAttack() error = %v", err)
		}
		if after := a.get(t, a.master).GetRevision(); after != before+1 {
			t.Errorf("the revision went from %d to %d, want +1 for the question", before, after)
		}
	})
	t.Run("spell attack", func(t *testing.T) {
		t.Parallel()
		a, e := brisaFight(t, true)
		a.h.roller.queue(4, 13)
		before := a.get(t, a.master).GetRevision()
		if _, err := a.castGuidingBolt(t, e); err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
		if after := a.get(t, a.master).GetRevision(); after != before+1 {
			t.Errorf("the revision went from %d to %d, want +1 for the question", before, after)
		}
	})
}

// TestTheMastersListHasEveryQuestion: ListHeldRolls answers the master every question that waits, in a combat or out of
// one, and a player their own and nobody else's; GetOutsideInspiration, the older list, keeps answering the master the
// dice the characters hold and no question at all.
func TestTheMastersListHasEveryQuestion(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:athletics", dc: new(int32(25))})
	a.openScene(t, point)
	if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
		t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
	}
	held, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 14)
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("RollSceneCheck() = %v, %v; want the die's question", held, err)
	}
	if got := a.outsideState(t, dalila); len(got.GetOffers()) != 1 {
		t.Errorf("the player reads %d offers, want her question", len(got.GetOffers()))
	}
	got := a.outsideState(t, a.master)
	if len(got.GetOffers()) != 0 || len(got.GetLuckyOffers()) != 0 || len(got.GetHeld()) != 1 {
		t.Errorf("the master reads %d offers, %d Lucky offers and %d dice held, want no question and Dalila's die", len(got.GetOffers()), len(got.GetLuckyOffers()), len(got.GetHeld()))
	}
	hold := held.GetInspirationOffer().GetHoldId()
	for name, u := range map[string]*user{"the master": a.master, "Dalila": dalila} {
		list := a.listHeldRolls(t, u)
		if len(list) != 1 || list[0].GetHoldId() != hold || list[0].GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_SCENE_CHECK || list[0].GetDie() == nil {
			t.Errorf("%s reads %v, want the scene check's die question %s", name, list, hold)
		}
	}
	if list := a.listHeldRolls(t, a.bia); len(list) != 0 {
		t.Errorf("another player reads %v, want nothing", list)
	}
}

// answerHeldRoll calls AnswerHeldRoll as u, with a key of its own.
func (a *armed) answerHeldRoll(t *testing.T, u *user, holdID string, edit func(*playv1.AnswerHeldRollRequest)) (*playv1.AnswerHeldRollResponse, error) {
	t.Helper()
	req := &playv1.AnswerHeldRollRequest{CampaignId: a.campaignID, HoldId: holdID, IdempotencyKey: newKey()}
	edit(req)
	res, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// dieAnswer keeps the Bardic Inspiration die (false) or uses it, rolled in the app (true).
func dieAnswer(use bool) func(*playv1.AnswerHeldRollRequest) {
	return func(r *playv1.AnswerHeldRollRequest) {
		d := &playv1.DieAnswer{Use: use}
		if use {
			d.Roll = &playv1.DieAnswer_RollInApp{RollInApp: true}
		}
		r.Answer = &playv1.AnswerHeldRollRequest_Die{Die: d}
	}
}

// luckyAnswer rolls the die that shows the 1 again, typed.
func luckyAnswer(face int32) func(*playv1.AnswerHeldRollRequest) {
	return func(r *playv1.AnswerHeldRollRequest) {
		r.Answer = &playv1.AnswerHeldRollRequest_Lucky{Lucky: &playv1.LuckyAnswer{Reroll: true, Roll: &playv1.LuckyAnswer_TypedFace{TypedFace: face}}}
	}
}

// listHeldRolls calls ListHeldRolls as u.
func (a *armed) listHeldRolls(t *testing.T, u *user) []*playv1.HeldRollOffer {
	t.Helper()
	res, err := u.resource.ListHeldRolls(t.Context(), connect.NewRequest(&playv1.ListHeldRollsRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("ListHeldRolls() error = %v", err)
	}
	return res.Msg.GetOffers()
}
