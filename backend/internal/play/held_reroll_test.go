package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The reroll question of a d20 (#573, held_reroll.go): a character whose table feature gives a reroll that covers the
// roll is asked "rolar de novo?" when the d20 shows the rule's limit or lower and the uses are there; the new roll is
// rolled by the server and must be used. The question is asked only with the switch on (Config.D20Rerolls); with it off
// the roll settles as if the player declined. The fixture is Toren, a fighter of level 5 (Caio's, a battleaxe at +5)
// with the table feat of rerollEffects-like effects: a pool of two uses and a reroll of attack, check and save d20 of at
// most 2 that costs one. These tests need the database (MEURPG_TEST_DATABASE_URL).

// d20RerollEffects are the effects of the usual feat for the d20: the pool of rerollUses and a reroll of at most atMost
// on the attack, the check and the save, costing cost.
func d20RerollEffects(pool string, atMost int32, cost string) []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: rerollUses, Max: pool, Recharge: "short_rest"},
		{Type: "reroll", Rolls: []string{"attack", "check", "save"}, AtMost: atMost, Resource: rerollUses, Cost: cost},
	}
}

// d20RerollTable is the casters' party with the feat on Toren; on is the switch.
func d20RerollTable(t *testing.T, on bool, effects []*rulesv1.TableEffect) *rerollTable {
	t.Helper()
	r := &rerollTable{}
	r.armed = newArmedWith(t, func(a *armed) {
		r.feat = a.rerollFeat(t, effects...)
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell, holdPerson}, []string{magicMissileSpell, holdPerson})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds})
		a.giveFeat(t, a.toren, r.feat)
	})
	r.h.svc.d20Rerolls = on
	return r
}

// usualD20RerollTable is the table with the usual feat: two uses, at most 2, one use each.
func usualD20RerollTable(t *testing.T, on bool) *rerollTable {
	t.Helper()
	return d20RerollTable(t, on, d20RerollEffects("2", 2, "1"))
}

// rerollAnswer rolls the d20 again (true) or keeps it.
func rerollAnswer(again bool) func(*playv1.AnswerHeldRollRequest) {
	return func(r *playv1.AnswerHeldRollRequest) {
		r.Answer = &playv1.AnswerHeldRollRequest_Reroll{Reroll: &playv1.RerollAnswer{Reroll: again}}
	}
}

// answerHeldRollKey is answerHeldRoll with the key given, to send the same answer twice.
func (a *armed) answerHeldRollKey(t *testing.T, u *user, holdID, key string, edit func(*playv1.AnswerHeldRollRequest)) (*playv1.AnswerHeldRollResponse, error) {
	t.Helper()
	req := &playv1.AnswerHeldRollRequest{CampaignId: a.campaignID, HoldId: holdID, IdempotencyKey: key}
	edit(req)
	res, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// wantReroll checks the question is the reroll's, for the kind, and what it says of the rule and the die.
func (r *rerollTable) wantReroll(t *testing.T, what string, offer *playv1.HeldRollOffer, kind playv1.HeldRollKind, face int32) {
	t.Helper()
	q := offer.GetReroll()
	if q == nil || offer.GetKind() != kind || q.GetAtMost() != 2 || q.GetResource() != rerollUses || q.GetCost() != 1 || q.GetRerollIndex() != 0 || q.GetSourceKey() == "" {
		t.Fatalf("%s = %v; want the reroll question (kind %v, at most 2, 1 use of %s)", what, offer, kind, rerollUses)
	}
	if got := offer.GetD20().GetFaces(); len(got) != 1 || got[0] != face {
		t.Fatalf("%s: d20 = %v, want the %d as rolled", what, offer.GetD20(), face)
	}
}

// wantRerolled checks the d20 lists the one reroll the rule made, with the rule's source.
func (r *rerollTable) wantRerolled(t *testing.T, what string, d20 *playv1.DiceRoll, from, to int32, source string) {
	t.Helper()
	rr := d20.GetRerolled()
	if len(rr) != 1 || rr[0].GetFrom() != from || rr[0].GetTo() != to || rr[0].GetSourceKey() != source || rr[0].GetSourceKey() == "" {
		t.Fatalf("%s: rerolled = %v, want %d -> %d from %q", what, rr, from, to, source)
	}
	if faces := d20.GetFaces(); len(faces) != 1 || faces[0] != to {
		t.Fatalf("%s: faces = %v, want the new %d", what, faces, to)
	}
}

// holds counts the rows of inspiration_holds of the campaign.
func (a *armed) holds(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), countInspirationHold, a.campaignID).Scan(&n); err != nil {
		t.Fatalf("count the holds: %v", err)
	}
	return n
}

// A weapon attack whose d20 shows the limit or lower is held with the reroll question (the armor class is never told).
// Answering yes rolls the d20 again with the service's roller, the new face is used, the rule's use is spent, and the
// result lists the reroll with the rule's source.
func TestAD20AtTheLimitAsksToRollAgainAndTheNewRollIsUsed(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	offer := res.GetHeldRollOffer()
	r.wantReroll(t, "the held attack", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 2)
	if res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED || res.GetPendingDamage() != nil ||
		offer.GetAttackKey() != battleaxe || offer.GetTargetId() != r.id(t, "Goblin") {
		t.Fatalf("the held attack = %v; want the 2 on the goblin, no result and no damage", res)
	}
	if c := byLabel(t, r.get(t, r.caio), "Toren"); c.GetActionUsed() || c.GetHeldRollOffer().GetHoldId() != offer.GetHoldId() {
		t.Errorf("Toren = action used %v, question %v; want the action kept and the question on him", c.GetActionUsed(), c.GetHeldRollOffer())
	}
	if list := r.listHeldRolls(t, r.ana); len(list) != 0 {
		t.Errorf("another player's list = %v, want nothing", list)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d before the answer, want 2", left)
	}
	if _, err := r.answerHeldRoll(t, r.ana, offer.GetHoldId(), rerollAnswer(true)); err == nil {
		t.Error("another player answered the question")
	}
	r.h.roller.queue(17)
	out, err := r.answerHeldRoll(t, r.caio, offer.GetHoldId(), rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a := out.GetAttack()
	if a.GetRoll().GetOutcome() != hit || a.GetPendingDamage() == nil || out.GetNext() != nil {
		t.Fatalf("the attack = outcome %v, pending %v, next %v; want a hit with the new 17 and damage to roll", a.GetRoll().GetOutcome(), a.GetPendingDamage(), out.GetNext())
	}
	r.wantRerolled(t, "the attack's d20", a.GetRoll().GetD20(), 2, 17, offer.GetReroll().GetSourceKey())
	if a.GetRoll().GetD20().GetTotal() != 17+a.GetRoll().GetD20().GetModifier() {
		t.Errorf("total = %d, want the new 17 plus the bonus", a.GetRoll().GetD20().GetTotal())
	}
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
	if !offer.GetD20().GetPhysical() || a.GetRoll().GetD20().GetPhysical() {
		t.Errorf("physical = %v as held, %v as rerolled; want the typed 2 physical and the server's new roll not", offer.GetD20().GetPhysical(), a.GetRoll().GetD20().GetPhysical())
	}
	if c := byLabel(t, a.GetEncounter(), "Toren"); !c.GetActionUsed() || c.GetHeldRollOffer() != nil {
		t.Errorf("Toren = action used %v, question %v; want the action spent and no question", c.GetActionUsed(), c.GetHeldRollOffer())
	}
}

// The player's undo of the attack gives the use back, and the undone roll is gone from the log.
func TestUndoingARerolledAttackGivesTheUseBack(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil {
		t.Fatalf("RollAttack(1) error = %v", err)
	}
	r.h.roller.queue(9)
	if _, err := r.answerHeldRoll(t, r.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(true)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if left := r.left(t); left != 1 {
		t.Fatalf("uses left = %d, want one spent", left)
	}
	l := r.log(t, r.master, e)
	if err := r.undo(t, r.master, e, l.GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d after the undo, want the use back", left)
	}
}

// Keeping the d20 spends nothing: the attack resolves with the roll as it was and no reroll.
func TestKeepingTheD20SpendsNoUse(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	out, err := r.answerHeldRoll(t, r.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a := out.GetAttack()
	d20 := a.GetRoll().GetD20()
	if a.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || len(d20.GetRerolled()) != 0 || d20.GetFaces()[0] != 2 || out.GetNext() != nil {
		t.Errorf("the attack = outcome %v, d20 %v; want the 2 kept, a miss and no reroll", a.GetRoll().GetOutcome(), d20)
	}
	if !d20.GetPhysical() {
		t.Error("the kept 2 is not physical any more, want the typed face as it was")
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d, want none spent", left)
	}
}

// With the switch off (the default) the same character and the same roll settle at once: no hold, no question, nothing
// spent and nothing written in inspiration_holds; the d20 stays as rolled.
func TestWithTheSwitchOffTheRollSettlesAsIfDeclined(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, false)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	d20 := res.GetRoll().GetD20()
	if res.GetHeldRollOffer() != nil || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || len(d20.GetRerolled()) != 0 || d20.GetFaces()[0] != 2 {
		t.Fatalf("the attack = %v; want it resolved as a miss on the 2, with no question", res)
	}
	if n := r.holds(t); n != 0 {
		t.Errorf("%d rows in inspiration_holds, want none", n)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d, want none spent", left)
	}
	if list := r.listHeldRolls(t, r.caio); len(list) != 0 {
		t.Errorf("the player's list = %v, want nothing", list)
	}
	if c := byLabel(t, res.GetEncounter(), "Toren"); !c.GetActionUsed() || c.GetHeldRollOffer() != nil {
		t.Errorf("Toren = action used %v, question %v; want the action spent and no question", c.GetActionUsed(), c.GetHeldRollOffer())
	}
}

// A face above the limit is not asked, and neither is a roll the character cannot pay: a rule that costs more than the
// pool holds is never offered, and a pool spent by an earlier reroll leaves the next low face alone.
func TestNoQuestionAboveTheLimitOrWithoutTheUses(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(3))
	if err != nil || res.GetHeldRollOffer() != nil || res.GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Fatalf("a 3 with the limit at 2 = %v, %v; want it resolved with no question", res, err)
	}
	if n := r.holds(t); n != 0 {
		t.Errorf("%d rows in inspiration_holds for a face above the limit, want none", n)
	}

	dear := d20RerollTable(t, true, d20RerollEffects("2", 2, "3"))
	e = dear.closeFight(t)
	res, err = dear.attack(t, dear.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil || res.GetHeldRollOffer() != nil || res.GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Fatalf("a rule that costs 3 of a pool of 2 = %v, %v; want it resolved with no question", res, err)
	}

	one := d20RerollTable(t, true, d20RerollEffects("1", 2, "1"))
	e = one.closeFight(t)
	res, err = one.attack(t, one.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil || res.GetHeldRollOffer().GetReroll() == nil {
		t.Fatalf("the first low face = %v, %v; want the question", res, err)
	}
	one.h.roller.queue(12)
	if _, err := one.answerHeldRoll(t, one.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(true)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if left := one.left(t); left != 0 {
		t.Fatalf("uses left = %d, want the only one spent", left)
	}
	e = one.turnOf(t, "Toren")
	res, err = one.attack(t, one.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if err != nil || res.GetHeldRollOffer() != nil || res.GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED {
		t.Errorf("a low face with no use left = %v, %v; want it resolved with no question", res, err)
	}
}

// The new roll is final: another low face is used as it is and never asked about again.
func TestTheNewRollIsFinalEvenWhenLow(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	r.h.roller.queue(1)
	out, err := r.answerHeldRoll(t, r.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(true))
	if err != nil || out.GetNext() != nil || out.GetAttack().GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Fatalf("the answer = %v, %v; want the new 1 used, a miss and no further question", out, err)
	}
	r.wantRerolled(t, "the attack's d20", out.GetAttack().GetRoll().GetD20(), 2, 1, res.GetHeldRollOffer().GetReroll().GetSourceKey())
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
}

// The same answer again (a retry with its key) gives the same result and spends nothing more; another answer to the
// roll is refused.
func TestARetryOfTheRerollAnswerSpendsOnce(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	hold, key := res.GetHeldRollOffer().GetHoldId(), newKey()
	r.h.roller.queue(15)
	first, err := r.answerHeldRollKey(t, r.caio, hold, key, rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	r.h.roller.queue(4) // a second roll would take this one
	again, err := r.answerHeldRollKey(t, r.caio, hold, key, rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() again error = %v", err)
	}
	if a, b := first.GetAttack().GetRoll(), again.GetAttack().GetRoll(); a.GetD20().GetFaces()[0] != 15 || b.GetD20().GetFaces()[0] != 15 || a.GetOutcome() != b.GetOutcome() {
		t.Errorf("the retry = %v, want the first answer's roll %v", b, a)
	}
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d after the retry, want one spent in all", left)
	}
	_, err = r.answerHeldRoll(t, r.caio, hold, rerollAnswer(true))
	wantCode(t, "another answer", err, connect.CodeFailedPrecondition)
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d after the refused answer, want one spent in all", left)
	}
}

// An ability check of the scene asks the question too, and the result lists the reroll; the use is spent.
func TestASceneCheckAtTheLimitAsksToRollAgain(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	point, actions := r.h.newScene(r.mapID, "O muro do pátio", true, 5, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	r.openScene(t, point)
	held, err := r.rollSceneTyped(t, r.caio, actions[0], newKey(), 2)
	if err != nil || held.GetRoll() != nil {
		t.Fatalf("the check on a 2 = %v, %v; want it held", held, err)
	}
	r.wantReroll(t, "the held check", held.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_SCENE_CHECK, 2)
	r.h.roller.queue(16)
	out, err := r.answerHeldRoll(t, r.caio, held.GetHeldRollOffer().GetHoldId(), rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	roll := out.GetSceneCheck().GetRoll()
	if roll == nil || !roll.GetPassed() {
		t.Fatalf("the check = %v, want it written, passed on the new 16", roll)
	}
	r.wantRerolled(t, "the check's d20", roll.GetRoll(), 2, 16, held.GetHeldRollOffer().GetReroll().GetSourceKey())
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
	if n := r.countSceneRolls(t); n != 1 {
		t.Errorf("%d scene rolls written, want the one", n)
	}
}

// With the switch off the same check is written at once, as rolled.
func TestASceneCheckWithTheSwitchOffIsWrittenAsRolled(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, false)
	point, actions := r.h.newScene(r.mapID, "O muro do pátio", true, 5, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	r.openScene(t, point)
	res, err := r.rollSceneTyped(t, r.caio, actions[0], newKey(), 2)
	if err != nil || res.GetHeldRollOffer() != nil || res.GetRoll() == nil || res.GetRoll().GetPassed() || len(res.GetRoll().GetRoll().GetRerolled()) != 0 {
		t.Fatalf("the check = %v, %v; want it written as a failure on the 2, with no question", res, err)
	}
	if n := r.holds(t); n != 0 {
		t.Errorf("%d rows in inspiration_holds, want none", n)
	}
	if left := r.left(t); left != 2 {
		t.Errorf("uses left = %d, want none spent", left)
	}
}

// A saving throw (the effect's) asks the question too: the new roll saves, the effect ends, the use is spent.
func TestAnEffectSaveAtTheLimitAsksToRollAgain(t *testing.T) {
	t.Parallel()
	r := usualD20RerollTable(t, true)
	e := r.closeFight(t)
	r.mustAddEffect(t, e, holdPerson, []string{"Toren"}, r.rounds(t, 10, "Toren"))
	r.passTo(t, e, "Toren")
	if _, err := r.endTurnRaw(t, r.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := r.windowOf(t, r.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want the save of Hold Person", r.windows(t, r.caio))
	}
	res, err := r.rollEffectSave(t, r.caio, e, w.GetId(), effectSaveFace(2))
	if err != nil || res.GetResult() != nil {
		t.Fatalf("RollEffectSave(2) = %v, %v; want the question alone", res, err)
	}
	r.wantReroll(t, "the held save", res.GetHeldRollOffer(), playv1.HeldRollKind_HELD_ROLL_KIND_EFFECT_SAVE, 2)
	r.h.roller.queue(18)
	out, err := r.answerHeldRoll(t, r.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	save := out.GetEffectSave().GetResult()
	if !save.GetSaved() || save.GetD20() != 18 || !save.GetEffectEnded() || out.GetNext() != nil {
		t.Fatalf("the save = %v, next %v; want the new 18 to save and end Hold Person", save, out.GetNext())
	}
	if rr := save.GetRerolled(); len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 18 || rr[0].GetSourceKey() != res.GetHeldRollOffer().GetReroll().GetSourceKey() {
		t.Errorf("rerolled = %v, want 2 -> 18 from the rule's source", rr)
	}
	if left := r.left(t); left != 1 {
		t.Errorf("uses left = %d, want one spent", left)
	}
}
