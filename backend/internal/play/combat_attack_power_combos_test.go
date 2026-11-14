package play

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A power attack with the other things that change an attack after its d20: a d20 rerolled with Lucky, Stroke of Luck, a
// Parry that turns the hit into a miss and Deflect Missiles' throw back. The trade here is a table feat's, with a pool
// of two uses: 2 off the roll for 4 damage, any weapon.

const (
	comboPool    = "trocas_do_talento"
	comboPenalty = 2
	comboDamage  = 4
)

// powerFeat creates the feat with the trade and its pool; its key is the key the request names.
func powerFeat(t *testing.T, a *armed) string {
	t.Helper()
	res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
		CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
			NamePt: "Troca do Talento", DescPt: []string{"Troca precisão por dano."},
			Effects: []*rulesv1.TableEffect{
				{Type: "resource", Resource: comboPool, Max: "2", Recharge: "short_rest"},
				{Type: "power_attack", AttackPenalty: comboPenalty, DamageBonus: comboDamage, Weapon: "any", Resource: comboPool},
			},
		}},
	}))
	if err != nil {
		t.Fatalf("CreateTableEntry(feat) error = %v", err)
	}
	return res.Msg.GetEntry().GetKey()
}

func (a *armed) comboUsesLeft(t *testing.T, c *charactersv1.Character) int32 {
	t.Helper()
	left, total := poolOf(a.vitals(t, c), comboPool)
	if total == 0 {
		t.Fatalf("%s has no pool %s", c.GetName(), comboPool)
	}
	return left
}

// lastTrade is the trade the newest attack event of the combat carries, nil for none.
func (a *armed) lastTrade(t *testing.T) *powerAttackEvent {
	t.Helper()
	var payload string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'attack_rolled' AND payload ? 'total' ORDER BY seq DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatalf("read the attack's event: %v", err)
	}
	var ev struct {
		PowerAttack *powerAttackEvent `json:"power_attack"`
	}
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("read the attack's event %s: %v", payload, err)
	}
	return ev.PowerAttack
}

// Lucky rolls the 1 again: the answer settles the roll with the trade it was held with, and the use is spent once, whether
// the new d20 hits (with the trade's part) or the player keeps the 1 (a miss, no damage, the use still spent).
func TestALuckyRerollOfAPowerAttackSpendsTheUseOnce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		answer func(*playv1.AnswerLuckyRequest)
		hit    bool
	}{
		{"the new d20 hits", rerollTyped(15), true},
		{"the 1 is kept", keepTheOne, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := newPowerTable(t)
			e := g.fight(t)
			held, err := g.swing(t, e, g.heavyKey, 1)
			if err != nil || held.GetHeldRollOffer().GetLucky() == nil {
				t.Fatalf("RollAttack() = %v, %v; want Lucky's question", held, err)
			}
			if got := g.usesLeft(t); got != 2 {
				t.Fatalf("uses left = %d while the roll waits, want 2", got)
			}
			res, err := g.answerLucky(t, g.caio, e, held.GetHeldRollOffer().GetHoldId(), tc.answer)
			if err != nil {
				t.Fatalf("AnswerLucky() error = %v", err)
			}
			if got := g.usesLeft(t); got != 1 {
				t.Errorf("uses left = %d after the answer, want 1: spent once", got)
			}
			if _, trade := g.lastAttackEvent(t); trade == nil || trade.Key != g.heavyKey {
				t.Errorf("the event's trade = %+v, want the heavy trade", trade)
			}
			pending := res.GetAttack().GetPendingDamage()
			switch {
			case tc.hit:
				wantPowerPart(t, "the hit", pending.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
			case pending != nil:
				t.Errorf("the kept 1 opened the damage %v, want a miss", pending)
			}
			g.undoLast(t, e)
			if got := g.usesLeft(t); got != 2 {
				t.Errorf("uses left = %d after the undo, want 2", got)
			}
		})
	}
}

// Stroke of Luck turns the miss into a hit: the roll keeps the trade's penalty (2 + 9 - 2 = 9 against 12 is a miss), the hit
// carries the trade's part and the use of the pool is spent once, next to the feature's own.
func TestAStrokeOfLuckHitOfAPowerAttackKeepsThePenaltyAndAddsThePart(t *testing.T) {
	t.Parallel()
	a := strokeTable(t, "", 20)
	feat := powerFeat(t, a)
	a.giveFeat(t, a.toren, feat)
	e := a.strokeFight(t)
	const bonus = 9 - comboPenalty
	miss, err := a.attack(t, a.caio, e, "Pipo", rapier, "Goblin", func(r *playv1.RollAttackRequest) { d20(2)(r); r.PowerAttackKey = feat })
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	offer := miss.GetHeldRollOffer()
	wantStroke(t, "the held attack", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, true)
	if d := offer.GetD20(); d.GetModifier() != bonus || d.GetTotal() != 2+bonus {
		t.Errorf("the question = modifier %d, total %d; want %d and %d", d.GetModifier(), d.GetTotal(), bonus, 2+bonus)
	}
	if got := a.comboUsesLeft(t, a.toren); got != 2 {
		t.Fatalf("uses left = %d while the roll waits, want 2", got)
	}
	out, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), strokeAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	r := out.GetAttack()
	if r.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || r.GetRoll().GetD20().GetModifier() != bonus || r.GetRoll().GetD20().GetTotal() != 2+bonus {
		t.Fatalf("the answer = %v, want a hit on the total %d, modifier %d", r.GetRoll(), 2+bonus, bonus)
	}
	var part *playv1.DamagePart
	if part = powerPartOf(r.GetPendingDamage().GetParts()); part == nil || part.GetFlat() != comboDamage || part.GetSourceKey() != feat {
		t.Errorf("the hit's power part = %v, want %d from %s", part, comboDamage, feat)
	}
	if got := a.comboUsesLeft(t, a.toren); got != 1 {
		t.Errorf("uses left = %d, want 1: spent once", got)
	}
	if used := a.strokeUsed(t); used != 1 {
		t.Errorf("stroke_of_luck used = %d, want 1", used)
	}
	if tr := a.lastTrade(t); tr == nil || tr.Penalty != comboPenalty || tr.DamageBonus != comboDamage {
		t.Errorf("the event's trade = %+v, want %d and %d", tr, comboPenalty, comboDamage)
	}
}

// A Parry that turns the hit into a miss leaves the use spent (the trade was made with the attack), the event's marker on
// the attack and no damage, so no part.
func TestAParriedPowerAttackKeepsTheUseSpentAndOpensNoDamage(t *testing.T) {
	t.Parallel()
	a := newCastersGear(t, 3, "", []string{battleaxe, daggerKey, shortbowKey})
	feat := powerFeat(t, a)
	a.giveFeat(t, a.toren, feat)
	a, e := monsterFightOn(t, a, knightKey, knight)
	// 14 + 6 - 2 = 18 against the Knight's 18 is a hit; its Parry (+2) makes 20.
	hit, err := a.attack(t, a.caio, e, "Toren", battleaxe, knight, func(r *playv1.RollAttackRequest) { d20(14)(r); r.PowerAttackKey = feat })
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if hit.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || hit.GetPendingDamage() != nil {
		t.Fatalf("the attack = %v / %v, want a miss with no damage", hit.GetRoll(), hit.GetPendingDamage())
	}
	if d := hit.GetRoll().GetD20(); d.GetModifier() != 6-comboPenalty || d.GetTotal() != 14+6-comboPenalty {
		t.Errorf("the roll = modifier %d, total %d; want %d and %d: the penalty is in the total Parry was compared with", d.GetModifier(), d.GetTotal(), 6-comboPenalty, 14+6-comboPenalty)
	}
	if !a.reactionUsedBy(t, knight) {
		t.Fatal("the Knight did not parry: the test's numbers are off")
	}
	if got := a.comboUsesLeft(t, a.toren); got != 1 {
		t.Errorf("uses left = %d, want 1: the parried attack made the trade", got)
	}
	if tr := a.lastTrade(t); tr == nil || tr.Key != feat {
		t.Errorf("the event's trade = %+v, want the feat's", tr)
	}
	if n := a.pendingCount(t); n != 0 {
		t.Errorf("pending damages = %d, want none", n)
	}
}

// Deflect Missiles' throw back has no line on the monk's sheet (it is the missile thrown with the monk's proficiency): a
// trade with no filter goes on it and the penalty comes off the throw. This pins what the code does.
func TestAPowerAttackOnDeflectMissilesThrowBack(t *testing.T) {
	t.Parallel()
	var feat string
	a, e := reactorFight(t, func(t *testing.T, a *armed) *charactersv1.Character {
		t.Helper()
		feat = powerFeat(t, a)
		c := monkThree(t, a)
		a.giveFeat(t, c, feat)
		return c
	})
	hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES)
	if w == nil {
		t.Fatal("the monk has no Deflect Missiles window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 8}
	})
	plain, err := a.throwBack(t, a.ana, e, w.GetId(), "Reator", "Goblin", func(r *playv1.RollAttackRequest) { disadvantage(15)(r); r.PowerAttackKey = feat })
	if err != nil {
		t.Fatalf("RollAttack(throw back with the trade) error = %v", err)
	}
	// The monk throws with Dexterity 16 (+3) and proficiency (+2), the trade's 2 off: +3. The 15 is a 18 against the goblin's 12.
	const bonus = 3 + 2 - comboPenalty
	if d := plain.GetRoll().GetD20(); plain.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || d.GetModifier() != bonus || d.GetTotal() != 15+bonus {
		t.Errorf("the throw = %v, want a hit with modifier %d and total %d", plain.GetRoll(), bonus, 15+bonus)
	}
	part := powerPartOf(plain.GetPendingDamage().GetParts())
	if part == nil || part.GetFlat() != comboDamage || part.GetSourceKey() != feat || part.GetDamageTypeKey() != "damage-type:piercing" {
		t.Errorf("the throw's power part = %v, want %d of piercing from %s", part, comboDamage, feat)
	}
	if got := a.comboUsesLeft(t, a.pens); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}
