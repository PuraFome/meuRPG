package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The halfling's Lucky on a weapon attack (SRD 5.1, Halfling): a 1 on the d20 of an attack roll may
// be rolled again, and the new roll stands; with advantage or disadvantage only the die that shows
// the 1 is rolled again. The player is asked before the result ("Tirou 1: rolar de novo?"). These
// tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is Pipo, a halfling fighter 2
// (Caio's, a battleaxe at +5), Toren, a human fighter 2 (Ana's, the same battleaxe), and Orla, a
// bard 5 (Bia's, a d8 to give), with a goblin (AC 12, 7 hit points) next to Pipo and Toren.

// newLuckyTable is the party of this file, with the goblin and the map of newArmedWith.
func newLuckyTable(t *testing.T) *armed {
	t.Helper()
	fighter := &rulesv1.AbilityScores{Strength: 16, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10}
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Pipo", "class:fighter", "race:halfling", 2, fighter, []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, fighter, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Orla", "class:bard", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
	})
}

// luckyFight starts the combat with Pipo first, then Toren and Orla, the goblin last. The goblin
// stands below Pipo and to the right of Toren.
func (a *armed) luckyFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Pipo": 20, "Toren": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Pipo": {4, 3}, "Toren": {3, 4}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
}

// luckyInspired starts the combat with Orla first, gives Pipo her d8, then passes to Pipo's turn.
func (a *armed) luckyInspired(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Orla": 20, "Pipo": 14, "Toren": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Pipo": {4, 3}, "Toren": {3, 4}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	if _, err := a.giveInspiration(t, a.bia, e, "Orla", "Pipo"); err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	return a.turnOf(t, "Pipo")
}

func (a *armed) answerLucky(t *testing.T, u *user, e *playv1.Encounter, holdID string, edit func(*playv1.AnswerLuckyRequest)) (*playv1.AnswerLuckyResponse, error) {
	t.Helper()
	req := &playv1.AnswerLuckyRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), HoldId: holdID, IdempotencyKey: newKey()}
	edit(req)
	res, err := u.resource.AnswerLucky(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func rerollInApp(r *playv1.AnswerLuckyRequest) {
	r.Reroll, r.Roll = true, &playv1.AnswerLuckyRequest_RollInApp{RollInApp: true}
}

func rerollTyped(face int32) func(*playv1.AnswerLuckyRequest) {
	return func(r *playv1.AnswerLuckyRequest) {
		r.Reroll, r.Roll = true, &playv1.AnswerLuckyRequest_TypedFace{TypedFace: face}
	}
}

func keepTheOne(r *playv1.AnswerLuckyRequest) { r.Reroll = false }

// wantLuckyReroll checks the d20 Lucky rolled again, as a DiceRoll lists it.
func wantLuckyReroll(t *testing.T, what string, roll *playv1.DiceRoll, index, to int32) {
	t.Helper()
	got := roll.GetRerolled()
	if len(got) != 1 || got[0].GetIndex() != index || got[0].GetFrom() != 1 || got[0].GetTo() != to || got[0].GetSourceKey() != link.LuckyTrait {
		t.Errorf("%s rerolled = %v, want die %d from 1 to %d by %s", what, got, index, to, link.LuckyTrait)
	}
}

// attackLine is the last attack line of the log as u reads it.
func (a *armed) attackLine(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogEntry {
	t.Helper()
	var line *playv1.CombatLogEntry
	for _, l := range logEntries(a.log(t, u, e)) {
		if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
			line = l
		}
	}
	return line
}

// SRD 5.1, Halfling, Lucky: a typed 1 is held with the question and no result; nothing is spent or
// written. The question is the attacker's player's and the master's, never another player's (RN-10).
// Rolled again to 15, the attack hits (15 + 5 = 20 against AC 12) and the roll says what Lucky did.
func TestLuckyAsksAboutAOneBeforeTheResult(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	offer := held.GetLuckyOffer()
	if offer == nil || held.GetInspirationOffer() != nil || held.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED || held.GetPendingDamage() != nil {
		t.Fatalf("the held roll = lucky %v, inspiration %v, outcome %v, pending %v; want the Lucky question and no result",
			offer, held.GetInspirationOffer(), held.GetRoll().GetOutcome(), held.GetPendingDamage())
	}
	if d := offer.GetD20(); len(d.GetFaces()) != 1 || d.GetFaces()[0] != 1 || d.GetTotal() != 6 || !d.GetPhysical() || offer.GetLuckyIndex() != 0 {
		t.Errorf("offer = %v, want the typed d20 of 1 (total 6) and the die 0", offer)
	}
	if offer.GetAttackKey() != battleaxe || offer.GetTargetId() != a.id(t, "Goblin") {
		t.Errorf("offer = attack %q, target %q; want the battleaxe on the goblin", offer.GetAttackKey(), offer.GetTargetId())
	}
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); c.GetActionUsed() {
		t.Error("the action is spent before the answer")
	}
	if cur, _, _ := a.hp(t, "Goblin"); cur != 7 {
		t.Errorf("the goblin has %d hit points before the answer, want 7", cur)
	}
	if line := a.attackLine(t, a.master, e); line != nil {
		t.Errorf("the log has the attack before the answer: %v", line)
	}
	// The question is on the combatant for its player and the master (a reload shows it again), and
	// for nobody else: Ana and Bia read no question, with the same combat.
	for name, u := range map[string]*user{"Caio": a.caio, "master": a.master} {
		c := byLabel(t, a.get(t, u), "Pipo")
		if c.GetLuckyOffer().GetHoldId() != offer.GetHoldId() || c.GetInspirationOffer() != nil {
			t.Errorf("%s reads lucky %v, inspiration %v; want the question %s alone", name, c.GetLuckyOffer(), c.GetInspirationOffer(), offer.GetHoldId())
		}
	}
	for name, u := range map[string]*user{"Ana": a.ana, "Bia": a.bia} {
		if o := byLabel(t, a.get(t, u), "Pipo").GetLuckyOffer(); o != nil {
			t.Errorf("%s reads Pipo's question %v", name, o)
		}
	}

	res, err := a.answerLucky(t, a.caio, e, offer.GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetFaces()[0] != 15 || roll.GetD20().GetTotal() != 20 {
		t.Errorf("the attack = outcome %v, d20 %v; want a hit with 15 (total 20)", roll.GetOutcome(), roll.GetD20())
	}
	wantLuckyReroll(t, "the answer's d20", roll.GetD20(), 0, 15)
	if res.GetAttack().GetPendingDamage() == nil || res.GetAttack().GetLuckyOffer() != nil {
		t.Errorf("the answer = pending %v, question %v; want the damage to roll and no question", res.GetAttack().GetPendingDamage(), res.GetAttack().GetLuckyOffer())
	}
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); !c.GetActionUsed() || c.GetLuckyOffer() != nil {
		t.Errorf("after the answer: action used %v, question %v; want the action spent and the question gone", c.GetActionUsed(), c.GetLuckyOffer())
	}
	// The log keeps the reroll after a reload, with the dice: the master's and the attacker's player's.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio} {
		line := a.attackLine(t, u, e)
		if line == nil {
			t.Fatalf("%s has no attack line", name)
		}
		wantLuckyReroll(t, name+"'s log line", line.GetAttackRoll(), 0, 15)
	}
	if line := a.attackLine(t, a.ana, e); line == nil || line.GetAttackRoll() != nil {
		t.Errorf("Ana's attack line = %v, want the line without the dice", line)
	}
}

// Lucky is the halfling's: a human's 1 is a miss at once, as it always was.
func TestANonHalflingsOneIsNeverHeld(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	a.luckyFight(t)
	e := a.turnOf(t, "Toren")
	res, err := a.attack(t, a.ana, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if res.GetLuckyOffer() != nil || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || len(res.GetRoll().GetD20().GetRerolled()) != 0 {
		t.Errorf("the attack = question %v, outcome %v; want a miss at once", res.GetLuckyOffer(), res.GetRoll().GetOutcome())
	}
}

// A halfling's roll that shows no 1 is resolved at once: nothing to ask.
func TestALuckyRollWithoutAOneIsResolvedAtOnce(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.h.roller.queue(9)
	res, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if res.GetLuckyOffer() != nil || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || res.GetRoll().GetD20().GetTotal() != 14 {
		t.Errorf("the attack = question %v, outcome %v, total %d; want a hit with 14 at once", res.GetLuckyOffer(), res.GetRoll().GetOutcome(), res.GetRoll().GetD20().GetTotal())
	}
}

// The app rolls the new d20 when the player asks it to.
func TestLuckyRollsTheNewD20InTheApp(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.h.roller.queue(1)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if held.GetLuckyOffer() == nil || held.GetLuckyOffer().GetD20().GetPhysical() {
		t.Fatalf("the held roll = %v, want the question about the app's 1", held.GetLuckyOffer())
	}
	a.h.roller.queue(14)
	res, err := a.answerLucky(t, a.caio, e, held.GetLuckyOffer().GetHoldId(), rerollInApp)
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetTotal() != 19 || roll.GetD20().GetPhysical() {
		t.Errorf("the attack = outcome %v, d20 %v; want a hit with the app's 14 (total 19)", roll.GetOutcome(), roll.GetD20())
	}
	wantLuckyReroll(t, "the answer's d20", roll.GetD20(), 0, 14)
}

// Keeping the 1: the roll goes on with it, a natural 1 that always misses (SRD 5.1, "Rolling 1 or 20").
func TestKeepingTheOneIsAMiss(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	res, err := a.answerLucky(t, a.caio, e, held.GetLuckyOffer().GetHoldId(), keepTheOne)
	if err != nil {
		t.Fatalf("AnswerLucky(keep) error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || roll.GetD20().GetFaces()[0] != 1 || len(roll.GetD20().GetRerolled()) != 0 {
		t.Errorf("the attack = outcome %v, d20 %v; want the miss of the 1, nothing rolled again", roll.GetOutcome(), roll.GetD20())
	}
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); !c.GetActionUsed() {
		t.Error("keeping the 1 did not spend the action")
	}
}

// The new roll stands, even when it is another 1: it is never asked about again.
func TestTheNewRollIsNeverAskedAgain(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	res, err := a.answerLucky(t, a.caio, e, held.GetLuckyOffer().GetHoldId(), rerollTyped(1))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if res.GetAttack().GetLuckyOffer() != nil || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Errorf("the answer = question %v, outcome %v; want the miss of the new 1 and no second question", res.GetAttack().GetLuckyOffer(), roll.GetOutcome())
	}
	wantLuckyReroll(t, "the answer's d20", roll.GetD20(), 0, 1)
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); c.GetLuckyOffer() != nil {
		t.Errorf("Pipo is asked again: %v", c.GetLuckyOffer())
	}
}

// With advantage (the goblin is restrained) the 1 is the die advantage would drop: the question comes
// all the same (SRD 5.1, "Advantage and Disadvantage"), only that die is rolled again, and the die that
// counts is the higher one of the new pair.
func TestLuckyWithAdvantageRerollsOnlyTheDieOfTheOne(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.setConditions(t, e, "Goblin", "condition:restrained")
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{1, 13} })
	if err != nil {
		t.Fatalf("RollAttack(advantage) error = %v", err)
	}
	offer := held.GetLuckyOffer()
	if offer == nil || offer.GetLuckyIndex() != 0 || offer.GetD20().GetCountedIndex() != 1 || offer.GetD20().GetTotal() != 18 {
		t.Fatalf("the offer = %v, want the 1 of [1, 13] asked about, the 13 counting (total 18)", offer)
	}
	if faces := offer.GetD20().GetFaces(); len(faces) != 2 || faces[0] != 1 || faces[1] != 13 {
		t.Errorf("the offer's faces = %v, want [1 13]", faces)
	}
	res, err := a.answerLucky(t, a.caio, e, offer.GetHoldId(), rerollTyped(17))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	d := res.GetAttack().GetRoll().GetD20()
	if faces := d.GetFaces(); len(faces) != 2 || faces[0] != 17 || faces[1] != 13 || d.GetCountedIndex() != 0 || d.GetTotal() != 22 {
		t.Errorf("the d20 = %v, want [17 13] with the 17 counting (total 22)", d)
	}
	wantLuckyReroll(t, "the answer's d20", d, 0, 17)
	line := a.attackLine(t, a.master, e)
	if line == nil {
		t.Fatal("the master has no attack line")
	}
	if faces := line.GetAttackRoll().GetFaces(); len(faces) != 2 || faces[0] != 17 || faces[1] != 13 {
		t.Errorf("the log's faces = %v, want [17 13]", faces)
	}
	wantLuckyReroll(t, "the master's log line", line.GetAttackRoll(), 0, 17)
}

// Both dice on 1: one of them is rolled again, the other stays.
func TestLuckyWithBothDiceOnOneRerollsOneOfThem(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.setConditions(t, e, "Goblin", "condition:restrained")
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{1, 1} })
	if err != nil {
		t.Fatalf("RollAttack(advantage) error = %v", err)
	}
	if held.GetLuckyOffer().GetLuckyIndex() != 0 {
		t.Errorf("the offer = %v, want the first die asked about", held.GetLuckyOffer())
	}
	res, err := a.answerLucky(t, a.caio, e, held.GetLuckyOffer().GetHoldId(), rerollTyped(12))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	d := res.GetAttack().GetRoll().GetD20()
	if faces := d.GetFaces(); len(faces) != 2 || faces[0] != 12 || faces[1] != 1 || d.GetTotal() != 17 {
		t.Errorf("the d20 = %v, want [12 1] with the 12 counting (total 17)", d)
	}
	wantLuckyReroll(t, "the answer's d20", d, 0, 12)
}

// The 1 is on the table first, so Lucky is asked first; the Bardic Inspiration die is asked after it,
// on the new roll, and added to it (SRD 5.1, Halfling and Bard). The die's question waits meanwhile.
func TestLuckyComesBeforeTheBardicInspirationDie(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	a.h.roller.queue(1)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	luck := held.GetLuckyOffer()
	if luck == nil || held.GetInspirationOffer() != nil {
		t.Fatalf("the held roll = lucky %v, inspiration %v; want the Lucky question alone", luck, held.GetInspirationOffer())
	}
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); c.GetLuckyOffer() == nil || c.GetInspirationOffer() != nil {
		t.Errorf("Pipo reads lucky %v, inspiration %v; want the Lucky question alone", c.GetLuckyOffer(), c.GetInspirationOffer())
	}
	_, err = a.answer(t, a.caio, e, luck.GetHoldId(), useDieInApp)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)
	a.h.roller.queue(6)
	luckyKey := newKey()
	withKey := func(r *playv1.AnswerLuckyRequest) { rerollInApp(r); r.IdempotencyKey = luckyKey }
	res, err := a.answerLucky(t, a.caio, e, luck.GetHoldId(), withKey)
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	insp := res.GetAttack().GetInspirationOffer()
	if insp == nil || res.GetAttack().GetLuckyOffer() != nil || res.GetAttack().GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Fatalf("after Lucky = inspiration %v, lucky %v, outcome %v; want the die's question and no result", insp, res.GetAttack().GetLuckyOffer(), res.GetAttack().GetRoll().GetOutcome())
	}
	if insp.GetD20().GetFaces()[0] != 6 || insp.GetD20().GetTotal() != 11 || insp.GetHoldId() != luck.GetHoldId() {
		t.Errorf("the die's question = %v, want the new 6 (total 11) on the same hold", insp)
	}
	wantLuckyReroll(t, "the die's question", insp.GetD20(), 0, 6)
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); c.GetActionUsed() || c.GetLuckyOffer() != nil || c.GetInspirationOffer() == nil {
		t.Errorf("Pipo = action used %v, lucky %v, inspiration %v; want the die's question, nothing spent", c.GetActionUsed(), c.GetLuckyOffer(), c.GetInspirationOffer())
	}
	// The same Lucky answer again reads the question as it is: nothing is rolled again.
	a.h.roller.queue(2)
	again, err := a.answerLucky(t, a.caio, e, luck.GetHoldId(), withKey)
	if err != nil {
		t.Fatalf("AnswerLucky() retry error = %v", err)
	}
	if again.GetAttack().GetInspirationOffer().GetD20().GetFaces()[0] != 6 {
		t.Errorf("the retry = %v, want the same 6", again.GetAttack().GetInspirationOffer())
	}
	// The die on the new roll: the d8 takes the 2 the retry left in the queue (6 + 5 + 2 = 13).
	res2, err := a.answer(t, a.caio, e, insp.GetHoldId(), useDieInApp)
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	roll := res2.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetTotal() != 13 {
		t.Errorf("the attack = outcome %v, total %d; want a hit with 13 (6 + 5 + the d8's 2)", roll.GetOutcome(), roll.GetD20().GetTotal())
	}
	wantLuckyReroll(t, "the resolved d20", roll.GetD20(), 0, 6)
	if b := roll.GetBonusDice(); len(b) != 1 || b[0].GetSides() != 8 || b[0].GetFace() != 2 {
		t.Errorf("bonus dice = %v, want the d8 of 2", b)
	}
	// Once resolved, the Lucky answer repeated gives the attack as it was resolved.
	last, err := a.answerLucky(t, a.caio, e, luck.GetHoldId(), withKey)
	if err != nil {
		t.Fatalf("AnswerLucky() after the die error = %v", err)
	}
	if last.GetAttack().GetRoll().GetD20().GetTotal() != 13 {
		t.Errorf("the Lucky answer after the die = %v, want the resolved attack (13)", last.GetAttack().GetRoll())
	}
}

// A retry of the roll and of the answer rolls nothing twice and writes one attack; another answer is
// refused once the question was answered; another attack waits for the answer (LUCKY_PENDING).
func TestLuckyIsAnsweredOnceAndRetriesRollNothing(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.h.roller.queue(1)
	req := &playv1.RollAttackRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, "Pipo"), AttackKey: battleaxe, TargetId: a.id(t, "Goblin"), IdempotencyKey: newKey(),
		Roll: &playv1.RollAttackRequest_RollInApp{RollInApp: true},
	}
	first, err := a.caio.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	holdID := first.Msg.GetLuckyOffer().GetHoldId()
	// The same request again: the same question, no new d20 (the queued 19 waits for the answer).
	a.h.roller.queue(19)
	again, err := a.caio.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RollAttack() retry error = %v", err)
	}
	if again.Msg.GetLuckyOffer().GetHoldId() != holdID || again.Msg.GetRoll().GetD20().GetFaces()[0] != 1 {
		t.Errorf("the retry = %v, want the same question and the d20 of 1", again.Msg.GetLuckyOffer())
	}
	// Another attack while the question waits.
	_, err = a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)
	answerKey := newKey()
	withKey := func(r *playv1.AnswerLuckyRequest) { rerollInApp(r); r.IdempotencyKey = answerKey }
	res, err := a.answerLucky(t, a.caio, e, holdID, withKey)
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	if total := res.GetAttack().GetRoll().GetD20().GetTotal(); total != 24 {
		t.Errorf("the answer's total = %d, want the queued 19 + 5", total)
	}
	a.h.roller.queue(3)
	retry, err := a.answerLucky(t, a.caio, e, holdID, withKey)
	if err != nil {
		t.Fatalf("AnswerLucky() retry error = %v", err)
	}
	if retry.GetAttack().GetRoll().GetD20().GetTotal() != 24 {
		t.Errorf("the retry = %v, want the first answer's 24", retry.GetAttack().GetRoll())
	}
	if _, err := a.answerLucky(t, a.caio, e, holdID, keepTheOne); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a second answer with another key = %v, want failed_precondition", err)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events WHERE kind = 'attack_rolled' AND encounter_id = $1`, e.GetId()).Scan(&n); err != nil || n != 1 {
		t.Errorf("attack events = %d (%v), want 1", n, err)
	}
}

// The answer is the roll's player's or the master's: another player is refused, a roll that is not
// found is not found, and the master answers for the player.
func TestTheLuckyQuestionIsAnsweredByItsPlayerOrTheMaster(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	holdID := held.GetLuckyOffer().GetHoldId()
	if _, err := a.answerLucky(t, a.ana, e, holdID, rerollTyped(15)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's answer = %v, want permission_denied", err)
	}
	if _, err := a.answerLucky(t, a.caio, e, newKey(), keepTheOne); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown roll = %v, want not_found", err)
	}
	a.h.afterThePlayersChance()
	res, err := a.answerLucky(t, a.master, e, holdID, rerollTyped(11))
	if err != nil {
		t.Fatalf("the master's AnswerLucky() error = %v", err)
	}
	if roll := res.GetAttack().GetRoll(); roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetTotal() != 16 {
		t.Errorf("the master's answer = %v, want a hit with 16 (11 + 5)", roll)
	}
}

// RN-18: the new d20 comes the way the campaign has the player roll. With real dice forced, the app's
// roll is refused and the typed face is taken, 1 to 20; rolling again needs one of the two.
func TestLuckyFollowsTheCampaignsDice(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack(typed) error = %v", err)
	}
	holdID := held.GetLuckyOffer().GetHoldId()
	_, err = a.answerLucky(t, a.caio, e, holdID, rerollInApp)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE)
	for _, face := range []int32{0, 21, -3} {
		if _, err := a.answerLucky(t, a.caio, e, holdID, rerollTyped(face)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("a typed face of %d = %v, want invalid_argument", face, err)
		}
	}
	if _, err := a.answerLucky(t, a.caio, e, holdID, func(r *playv1.AnswerLuckyRequest) { r.Reroll = true }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("rolling again with no roll = %v, want invalid_argument", err)
	}
	res, err := a.answerLucky(t, a.caio, e, holdID, rerollTyped(16))
	if err != nil {
		t.Fatalf("AnswerLucky(typed) error = %v", err)
	}
	if d := res.GetAttack().GetRoll().GetD20(); d.GetTotal() != 21 || !d.GetPhysical() {
		t.Errorf("the d20 = %v, want the typed 16 (total 21), a real die", d)
	}
}

// A hold that asks no Lucky question (an attacker with a die and no 1) is not answered with Lucky.
func TestAnswerLuckyRefusesARollWithoutTheQuestion(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	a.h.roller.queue(5)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if held.GetLuckyOffer() != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("the held roll = lucky %v, inspiration %v; want the die's question alone", held.GetLuckyOffer(), held.GetInspirationOffer())
	}
	if _, err := a.answerLucky(t, a.caio, e, held.GetInspirationOffer().GetHoldId(), rerollTyped(15)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("AnswerLucky on the die's question = %v, want failed_precondition", err)
	}
}

// The master's undo of an attack whose 1 was rolled again gives the action back, the damage goes, and
// the next roll asks again.
func TestUndoOfARerolledAttack(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	res, err := a.answerLucky(t, a.caio, e, held.GetLuckyOffer().GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	if res.GetAttack().GetPendingDamage() == nil {
		t.Fatal("the hit opened no damage")
	}
	a.undoLast(t, e)
	if c := byLabel(t, a.get(t, a.caio), "Pipo"); c.GetActionUsed() {
		t.Error("the undo did not give the action back")
	}
	if line := a.attackLine(t, a.master, e); line != nil {
		t.Errorf("the attack is still in the log after the undo: %v", line)
	}
	again, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil || again.GetLuckyOffer() == nil || again.GetLuckyOffer().GetHoldId() == held.GetLuckyOffer().GetHoldId() {
		t.Errorf("RollAttack() after the undo = %v, %v; want a new question", again.GetLuckyOffer(), err)
	}
}
