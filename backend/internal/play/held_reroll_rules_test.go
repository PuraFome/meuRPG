package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// More of the reroll question of a d20 (#573, held_reroll.go): the rules of the sheet (free, dearer, two of them), the
// kinds that ask and the ones that never do, the pairs of advantage and disadvantage, the answer of the master, the
// switch turned off with a question waiting, Lucky going first and the rows of the database. These tests need the
// database (MEURPG_TEST_DATABASE_URL), but the NPC one, which is a unit test.

// otherReroll is a second pool of uses, for the tables whose sheet has two rules.
const otherReroll = "dado_de_reserva"

// d20RerollParty is the casters' party (of one race when given) with the feat on Toren and, for each of the others named
// (Pensantus or Brisa), on them too; on is the switch.
func d20RerollParty(t *testing.T, on bool, effects []*rulesv1.TableEffect, race string, others ...string) *rerollTable {
	t.Helper()
	r := &rerollTable{armed: newCastersOf(t, 3, race)}
	r.feat = r.rerollFeat(t, effects...)
	r.giveFeat(t, r.toren, r.feat)
	for _, name := range others {
		r.giveFeat(t, map[string]*charactersv1.Character{"Pensantus": r.pens, "Brisa": r.bri}[name], r.feat)
	}
	r.h.svc.d20Rerolls = on
	return r
}

// leftOf is what is left of a pool of a character.
func (r *rerollTable) leftOf(t *testing.T, c *charactersv1.Character, key string) int32 {
	t.Helper()
	left, _ := poolOf(r.vitals(t, c), key)
	return left
}

// wantRerollOffer checks the question is the reroll's, for the kind, on the die at the index, with the rule's uses.
func wantRerollOffer(t *testing.T, what string, offer *playv1.HeldRollOffer, kind playv1.HeldRollKind, index int32, resource string, cost int32) {
	t.Helper()
	q := offer.GetReroll()
	if q == nil || offer.GetKind() != kind || q.GetRerollIndex() != index || q.GetResource() != resource || q.GetCost() != cost || q.GetSourceKey() == "" {
		t.Fatalf("%s = %v; want the reroll question (kind %v, die %d, %d of %q)", what, offer, kind, index, cost, resource)
	}
}

// rolledAgain answers yes with the new d20 the roller will give.
func (r *rerollTable) rolledAgain(t *testing.T, u *user, offer *playv1.HeldRollOffer, face int) *playv1.AnswerHeldRollResponse {
	t.Helper()
	r.h.roller.queue(face)
	out, err := r.answerHeldRoll(t, u, offer.GetHoldId(), rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	return out
}

// With advantage the question is about the die that counts (the higher one), whichever place it has in the pair, and
// the new roll replaces that die: the pair is [1, 7] and the 7 counts.
func TestAnAdvantagePairAsksAboutTheCountedDieAndTheChoiceIsMadeAgain(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "")
	e := r.closeFight(t)
	r.setConditions(t, e, "Goblin", "condition:restrained")
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", func(q *playv1.RollAttackRequest) { q.D20Faces = []int32{1, 2} })
	if err != nil {
		t.Fatalf("RollAttack(advantage) error = %v", err)
	}
	offer := res.GetHeldRollOffer()
	wantRerollOffer(t, "the held attack", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 1, rerollUses, 1)
	if offer.GetD20().GetCountedIndex() != 1 {
		t.Fatalf("the held pair = %v, want the 2 counting", offer.GetD20())
	}
	d := r.rolledAgain(t, r.caio, offer, 7).GetAttack().GetRoll().GetD20()
	rr := d.GetRerolled()
	if faces := d.GetFaces(); len(faces) != 2 || faces[0] != 1 || faces[1] != 7 || d.GetCountedIndex() != 1 || len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 7 {
		t.Errorf("the d20 = %v, want [1 7] with the 7 counting, rolled again from the 2", d)
	}
}

// With disadvantage the lower die counts and is asked about; when the new roll is higher than the other die, the other
// die counts: the pair is [18, 9] and the 9 counts, so the choice is made again over the new pair.
func TestADisadvantagePairMakesTheChoiceAgainOverTheNewPair(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "")
	e := r.closeFight(t)
	r.setConditions(t, e, "Toren", "condition:poisoned")
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", func(q *playv1.RollAttackRequest) { q.D20Faces = []int32{2, 9} })
	if err != nil {
		t.Fatalf("RollAttack(disadvantage) error = %v", err)
	}
	offer := res.GetHeldRollOffer()
	wantRerollOffer(t, "the held attack", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 0, rerollUses, 1)
	d := r.rolledAgain(t, r.caio, offer, 18).GetAttack().GetRoll().GetD20()
	if faces := d.GetFaces(); len(faces) != 2 || faces[0] != 18 || faces[1] != 9 || d.GetCountedIndex() != 1 || d.GetTotal() != 9+d.GetModifier() {
		t.Errorf("the d20 = %v, want [18 9] with the 9 counting", d)
	}
}

// A spell attack of a cast (Guiding Bolt, which has disadvantage next to the goblin: the pair is [2, 15]) asks too; yes
// spends the use, the cast is a hit, and the undo of the cast gives the use back.
func TestASpellAttackAsksToRollAgainAndTheUndoOfTheCastGivesTheUseBack(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "", "Brisa")
	r.closeFight(t)
	e := r.turnOf(t, "Brisa")
	res, err := r.cast(t, r.bia, e, "Brisa", guidingBolt, slotOfLevel(1), r.at(t, "Goblin"), d20Faces(2, 15))
	if err != nil || res.GetCast() != nil {
		t.Fatalf("CastSpell(2) = %v, %v; want the question and no cast", res, err)
	}
	wantRerollOffer(t, "the held spell attack", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_SPELL_ATTACK, 0, rerollUses, 1)
	out := r.rolledAgain(t, r.bia, res.GetHeldRollOffer(), 18)
	targets := out.GetSpellCast().GetCast().GetTargets()
	if len(targets) != 1 || targets[0].GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Fatalf("the cast = %v, want a hit on the new 18", out.GetSpellCast().GetCast())
	}
	if rr := targets[0].GetAttackRoll().GetRerolled(); len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 18 {
		t.Errorf("the attack roll = %v, want the 2 rolled again to 18", targets[0].GetAttackRoll())
	}
	if left := r.leftOf(t, r.bri, rerollUses); left != 1 {
		t.Fatalf("uses left = %d, want one spent", left)
	}
	l := r.log(t, r.master, e)
	if err := r.undo(t, r.master, e, l.GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if left := r.leftOf(t, r.bri, rerollUses); left != 2 {
		t.Errorf("uses left = %d after the undo of the cast, want the use back", left)
	}
}

// A group check asks too, and the roll written is the new one.
func TestAGroupCheckAtTheLimitAsksToRollAgain(t *testing.T) {
	t.Parallel()
	r := usualD20RerollParty(t)
	view := r.mustRequestGroup(t, nil)
	held, err := r.rollGroup(t, r.caio, view.GetId(), 2)
	if err != nil || held.GetGroupCheck() != nil {
		t.Fatalf("RollGroupCheck(2) = %v, %v; want the question alone", held, err)
	}
	wantRerollOffer(t, "the held group check", held.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_GROUP_CHECK, 0, rerollUses, 1)
	out := r.rolledAgain(t, r.caio, held.GetHeldRollOffer(), 16)
	roll := memberOf(out.GetGroupCheck().GetGroupCheck(), r.toren.GetId()).GetRoll()
	if rr := roll.GetRerolled(); roll.GetFaces()[0] != 16 || len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 16 {
		t.Errorf("the member's roll = %v, want the 2 rolled again to 16", roll)
	}
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
}

// A concentration save asks too: the new roll keeps the concentration.
func TestAConcentrationSaveAtTheLimitAsksToRollAgain(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "", "Pensantus")
	_, e := concentratingHitOn(t, r.armed, 6)
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", r.windows(t, r.ana))
	}
	res, err := r.resolveConcentration(t, r.ana, e, w.GetId(), concentrationFace(2))
	if err != nil || res.GetResult() != nil {
		t.Fatalf("ResolveConcentrationSave(2) = %v, %v; want the question alone", res, err)
	}
	wantRerollOffer(t, "the held concentration save", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_CONCENTRATION_SAVE, 0, rerollUses, 1)
	out := r.rolledAgain(t, r.ana, res.GetHeldRollOffer(), 15)
	save := out.GetConcentrationSave().GetResult()
	if !save.GetKept() || save.GetSave().GetFaces()[0] != 15 || len(save.GetSave().GetRerolled()) != 1 {
		t.Errorf("the save = %v, want the new 15 to keep the concentration", save)
	}
	if left := r.leftOf(t, r.pens, rerollUses); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
}

// usualD20RerollParty is d20RerollParty with the usual feat (two uses, at most 2, one use each) on Toren.
func usualD20RerollParty(t *testing.T) *rerollTable {
	t.Helper()
	return d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "")
}

// totalUsed is every use spent in the character's pools.
func (r *rerollTable) totalUsed(t *testing.T, c *charactersv1.Character) int32 {
	t.Helper()
	var n int32
	for _, p := range r.vitals(t, c).GetResources() {
		n += p.GetUsed()
	}
	return n
}

// A rule with no resource is free: the question comes, the roll is made again, and no use is spent anywhere.
func TestAFreeRuleRollsAgainWithoutSpending(t *testing.T) {
	t.Parallel()
	free := []*rulesv1.TableEffect{{Type: "reroll", Rolls: []string{"attack"}, AtMost: 2}}
	r := d20RerollParty(t, true, free, "")
	e := r.closeFight(t)
	before := r.totalUsed(t, r.toren)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	wantRerollOffer(t, "the held attack", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 0, "", 0)
	d := r.rolledAgain(t, r.caio, res.GetHeldRollOffer(), 15).GetAttack().GetRoll().GetD20()
	if rr := d.GetRerolled(); d.GetFaces()[0] != 15 || len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 15 {
		t.Errorf("the d20 = %v, want the 2 rolled again to 15", d)
	}
	if after := r.totalUsed(t, r.toren); after != before {
		t.Errorf("uses spent = %d, want the %d it was: the rule is free", after, before)
	}
}

// A rule that costs two spends two, and the undo of the attack gives the two back.
func TestARuleThatCostsTwoSpendsTwoAndTheUndoGivesTwoBack(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("3", 2, "2"), "")
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack(1) error = %v", err)
	}
	wantRerollOffer(t, "the held attack", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 0, rerollUses, 2)
	r.rolledAgain(t, r.caio, res.GetHeldRollOffer(), 14)
	if left := r.left(t); left != 1 {
		t.Fatalf("uses left = %d, want two of the three spent", left)
	}
	l := r.log(t, r.master, e)
	if err := r.undo(t, r.master, e, l.GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if left := r.left(t); left != 3 {
		t.Errorf("uses left = %d after the undo, want the two back (3)", left)
	}
}

// twoRules is a sheet with two rules for the attack, each on a pool of its own: the first costs two, the second one.
func twoRules(first, second string) []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: rerollUses, Max: first, Recharge: "short_rest"},
		{Type: "resource", Resource: otherReroll, Max: second, Recharge: "short_rest"},
		{Type: "reroll", Rolls: []string{"attack"}, AtMost: 2, Resource: rerollUses, Cost: "2"},
		{Type: "reroll", Rolls: []string{"attack"}, AtMost: 2, Resource: otherReroll, Cost: "1"},
	}
}

// With two rules the first one the character can pay is the one asked: the first when both are payable, the second when
// the first costs more than its pool holds.
func TestTheFirstRuleTheCharacterCanPayIsTheOneAsked(t *testing.T) {
	t.Parallel()
	t.Run("first unpayable", func(t *testing.T) {
		t.Parallel()
		r := d20RerollParty(t, true, twoRules("2", "1"), "")
		r.correct(t, r.toren, func(q *playv1.AdjustCharacterVitalsRequest) {
			q.ResourcesUsed = []*playv1.ResourceUsed{{Key: rerollUses, Used: 1}} // one of two left: the rule that costs two is out
		})
		e := r.closeFight(t)
		res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
		if err != nil {
			t.Fatalf("RollAttack(2) error = %v", err)
		}
		wantRerollOffer(t, "the held attack", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 0, otherReroll, 1)
		r.rolledAgain(t, r.caio, res.GetHeldRollOffer(), 15)
		if a, b := r.leftOf(t, r.toren, rerollUses), r.leftOf(t, r.toren, otherReroll); a != 1 || b != 0 {
			t.Errorf("uses left = %d and %d, want the second rule's pool spent alone (1 and 0)", a, b)
		}
	})
	t.Run("both payable", func(t *testing.T) {
		t.Parallel()
		r := d20RerollParty(t, true, twoRules("2", "2"), "")
		e := r.closeFight(t)
		res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
		if err != nil {
			t.Fatalf("RollAttack(2) error = %v", err)
		}
		wantRerollOffer(t, "the held attack", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 0, rerollUses, 2)
	})
}

// A roller that is not a player's character is never asked, and its sheet is never read.
func TestAnNPCRollerIsNeverAskedAboutAReroll(t *testing.T) {
	t.Parallel()
	s := &Service{d20Rerolls: true} // no roster and no vitals: reading either would panic
	spec := rollSpec{
		Kind: heldWeaponAttack, MayAsk: true,
		Roller: heldRoller{CampaignID: "c", CharacterID: "npc", Combatant: &playdb.Combatant{Kind: kindNPC}},
	}
	rules, err := s.rerollsLeft(t.Context(), gateEnv{}, spec)
	if err != nil || len(rules) != 0 {
		t.Errorf("rerollsLeft(NPC) = %v, %v; want nothing", rules, err)
	}
}

// The master answers for the player after the player's first 10 seconds, and the use spent is the player's.
func TestTheMasterAnswersTheRerollForThePlayerAfterTheirChance(t *testing.T) {
	t.Parallel()
	r := usualD20RerollParty(t)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	hold := res.GetHeldRollOffer().GetHoldId()
	if list := r.listHeldRolls(t, r.master); len(list) != 1 {
		t.Errorf("the master's list = %v, want the question", list)
	}
	_, err = r.answerHeldRoll(t, r.master, hold, rerollAnswer(true))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	r.h.afterThePlayersChance()
	r.h.roller.queue(17)
	out, err := r.answerHeldRoll(t, r.master, hold, rerollAnswer(true))
	if err != nil || out.GetAttack().GetRoll().GetOutcome() != hit {
		t.Fatalf("the master's answer = %v, %v; want the hit on the new 17", out, err)
	}
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want the player's use spent", left)
	}
}

// The uses may be spent elsewhere between the question and the answer: yes is refused with no uses left, nothing is
// taken (never a negative pool), and keeping the roll still works.
func TestAYesWithTheUsesSpentElsewhereIsRefusedAndKeepingStillWorks(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("1", 2, "1"), "")
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	hold := res.GetHeldRollOffer().GetHoldId()
	r.correct(t, r.toren, func(q *playv1.AdjustCharacterVitalsRequest) {
		q.ResourcesUsed = []*playv1.ResourceUsed{{Key: rerollUses, Used: 1}}
	})
	r.h.roller.queue(15)
	_, err = r.answerHeldRoll(t, r.caio, hold, rerollAnswer(true))
	wantBlockedBy(t, "a reroll with the uses spent elsewhere", err, blockedNoUses)
	if used, total := resourceUsed(r.vitals(t, r.toren), rerollUses); used != 1 || total != 1 {
		t.Fatalf("pool = %d of %d, want the 1 of 1 it was, nothing more taken", used, total)
	}
	out, err := r.answerHeldRoll(t, r.caio, hold, rerollAnswer(false))
	if err != nil || out.GetAttack().GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || len(out.GetAttack().GetRoll().GetD20().GetRerolled()) != 0 {
		t.Fatalf("keeping the 2 = %v, %v; want a miss on the 2 as rolled", out, err)
	}
}

// A kind that never asks (the contest of a grapple) is not held for a reroll even with a rule that covers "check" and a
// face at the limit, with the switch on: nothing is held, written or spent.
func TestAKindThatNeverAsksIsNotHeldForARerollWithTheSwitchOn(t *testing.T) {
	t.Parallel()
	r := usualD20RerollParty(t)
	c := r.arena(t, arenaPlan{})
	c.h.roller.queue(2)
	res, err := c.startContest(t, r.caio, func(q *playv1.StartContestRequest) {
		q.InitiatorId, q.TargetId, q.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	if err != nil || res.GetHeldRollOffer() != nil || res.GetContest() == nil {
		t.Fatalf("StartContest(2) = %v, %v; want the contest rolled, with no question", res, err)
	}
	if rr := res.GetContest().GetInitiatorRoll().GetRerolled(); len(rr) != 0 {
		t.Errorf("the roll = %v, want no reroll", rr)
	}
	if n := r.holds(t); n != 0 {
		t.Errorf("%d rows in inspiration_holds, want none", n)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d, want none spent", left)
	}
}

// A question written with the switch on that still waits when it goes off never blocks the roller. A roller whose gate
// runs (a halfling, whose Lucky may ask) forgets it in the next roll, which is written at once and not refused with
// REROLL_PENDING; for any other roller the gate has nothing to ask and does not look, and the old question, which the
// client cannot answer (its answer fails), stays harmlessly in the table until a roll of that kind runs the gate.
func TestAQuestionLeftWaitingWhenTheSwitchGoesOffDoesNotBlockTheRoller(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, race string }{{"a roller whose gate runs", "race:halfling"}, {"a roller whose gate does not run", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), tc.race)
			point, actions := r.h.newScene(r.mapID, "O muro do pátio", true, 5, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
			r.openScene(t, point)
			held, err := r.rollSceneTyped(t, r.caio, actions[0], newKey(), 2)
			if err != nil || held.GetHeldRollOffer().GetReroll() == nil {
				t.Fatalf("the check on a 2 = %v, %v; want the reroll question", held, err)
			}
			r.h.svc.d20Rerolls = false
			next, err := r.rollSceneTyped(t, r.caio, actions[0], newKey(), 2)
			if err != nil || next.GetHeldRollOffer() != nil || next.GetRoll() == nil {
				t.Fatalf("the next check = %v, %v; want it written at once, not refused for the old question", next, err)
			}
			if tc.race != "" {
				if n := r.holds(t); n != 0 {
					t.Errorf("%d rows in inspiration_holds, want the old question forgotten by the roll", n)
				}
			}
			if _, err := r.answerHeldRoll(t, r.caio, held.GetHeldRollOffer().GetHoldId(), rerollAnswer(true)); err == nil {
				t.Error("the old question was answered with the switch off")
			}
			if left := r.left(t); left != 2 {
				t.Errorf("uses left = %d, want none spent", left)
			}
		})
	}
}

// A halfling with a rule rolling a 1 is asked Lucky's question and no reroll's, before and after answering Lucky: the new
// roll (a 2, at the rule's limit) is used as it is, and so is a kept 1.
func TestLuckyGoesFirstAndTheRerollIsNeverAskedOnTheSameRoll(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, true, d20RerollEffects("2", 2, "1"), "race:halfling")
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil || res.GetHeldRollOffer().GetLucky() == nil || res.GetHeldRollOffer().GetReroll() != nil {
		t.Fatalf("RollAttack(1) = %v, %v; want Lucky's question and no reroll's", res, err)
	}
	var asks bool
	if err := r.h.pool.QueryRow(t.Context(), `SELECT bool_or(reroll_ask) FROM inspiration_holds WHERE campaign_id = $1`, r.campaignID).Scan(&asks); err != nil || asks {
		t.Fatalf("the hold of the 1 keeps a reroll question (%v, %v), want only Lucky's", asks, err)
	}
	out, err := r.answerLucky(t, r.caio, e, res.GetHeldRollOffer().GetHoldId(), rerollTyped(2))
	if err != nil || out.GetAttack().GetHeldRollOffer() != nil ||
		out.GetAttack().GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Fatalf("Lucky's answer (a 2) = %v, %v; want the attack settled, with no reroll question", out, err)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d, want none spent", left)
	}
	e = r.turnOf(t, "Toren")
	res, err = r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil || res.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollAttack(1) again = %v, %v; want Lucky's question", res, err)
	}
	out, err = r.answerLucky(t, r.caio, e, res.GetHeldRollOffer().GetHoldId(), keepTheOne)
	if err != nil || out.GetAttack().GetHeldRollOffer() != nil || out.GetAttack().GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Fatalf("keeping the 1 = %v, %v; want the attack settled, with no reroll question", out, err)
	}
}

// A row written before the reroll columns existed has them at their defaults, and reads back as a roll with no reroll
// (nothing asked, nothing rolled again) while its Lucky question is still there.
func TestARowWithTheRerollColumnsAtTheirDefaultsReadsBackAsNoReroll(t *testing.T) {
	t.Parallel()
	r := d20RerollParty(t, false, d20RerollEffects("2", 2, "1"), "race:halfling")
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil || res.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollAttack(1) = %v, %v; want Lucky's question", res, err)
	}
	if _, err := r.h.pool.Exec(t.Context(), `UPDATE inspiration_holds SET reroll_ask = DEFAULT, reroll_index = DEFAULT, reroll_at_most = DEFAULT,
		reroll_resource = DEFAULT, reroll_cost = DEFAULT, reroll_source = DEFAULT, reroll_answer_key = DEFAULT, reroll_from = DEFAULT, reroll_to = DEFAULT
		WHERE campaign_id = $1`, r.campaignID); err != nil {
		t.Fatalf("reset the reroll columns: %v", err)
	}
	rows, err := playdb.New(r.h.pool).ListOpenInspirationHoldsOfCampaign(t.Context(), r.campaignID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("the open holds = %v, %v; want the one", rows, err)
	}
	h := heldOfInspirationHold(rows[0])
	if rerollAsks(h) || rerolledOf(h) != nil || !luckyAsks(h) {
		t.Errorf("the old row = reroll asks %v, rerolled %v, lucky asks %v; want no reroll and Lucky still asking", rerollAsks(h), rerolledOf(h), luckyAsks(h))
	}
}

// The cleanup of the holds whose die is gone keeps a roll that waits for the reroll (the question needs no die) and
// still deletes a stale hold that asks nothing.
func TestTheOrphanCleanupKeepsAPendingRerollAndDeletesAStaleHoldThatAsksNothing(t *testing.T) {
	t.Parallel()
	r := usualD20RerollParty(t)
	point, actions := r.h.newScene(r.mapID, "O muro do pátio", true, 5, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	r.openScene(t, point)
	held, err := r.rollSceneTyped(t, r.caio, actions[0], newKey(), 2)
	if err != nil || held.GetHeldRollOffer().GetReroll() == nil {
		t.Fatalf("the check on a 2 = %v, %v; want the reroll question", held, err)
	}
	q := playdb.New(r.h.pool)
	if err := q.DeleteOrphanInspirationHolds(t.Context(), r.campaignID); err != nil {
		t.Fatalf("DeleteOrphanInspirationHolds() error = %v", err)
	}
	if n := r.holds(t); n != 1 {
		t.Fatalf("%d rows after the cleanup, want the pending reroll kept", n)
	}
	if _, err := r.h.pool.Exec(t.Context(), `UPDATE inspiration_holds SET reroll_ask = false WHERE campaign_id = $1`, r.campaignID); err != nil {
		t.Fatalf("make the hold ask nothing: %v", err)
	}
	if err := q.DeleteOrphanInspirationHolds(t.Context(), r.campaignID); err != nil {
		t.Fatalf("DeleteOrphanInspirationHolds() error = %v", err)
	}
	if n := r.holds(t); n != 0 {
		t.Errorf("%d rows after the cleanup, want the stale hold that asks nothing deleted", n)
	}
}
