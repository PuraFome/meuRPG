package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The damage of a power attack: a hit that made the trade opens a damage with an automatic flat part of the weapon's type
// (partPowerAttack), counted in the total and by type, never doubled by a critical hit. Pipo's greatsword rolls 2d6 and
// adds 3 (Strength 16); the heavy trade adds 10, the free one 2. The Goblin has armor class 12 and the Capitão 18.

const slashing = "damage-type:slashing"

// powerPartOf is the part the trade added to a damage, nil when there is none.
func powerPartOf(parts []*playv1.DamagePart) *playv1.DamagePart {
	var found *playv1.DamagePart
	for _, p := range parts {
		if p.GetKey() == partPowerAttack {
			if found != nil {
				return &playv1.DamagePart{Key: "twice"}
			}
			found = p
		}
	}
	return found
}

func powerRollOf(rolls []*playv1.DamagePartRoll) *playv1.DamagePartRoll {
	for _, r := range rolls {
		if r.GetPartKey() == partPowerAttack {
			return r
		}
	}
	return nil
}

// rolled rolls the damage as Pipo's player, typing the dice's sum.
func (g *powerTable) rolled(t *testing.T, e *playv1.Encounter, pendingID string, sum int32) *playv1.PendingDamage {
	t.Helper()
	return g.mustDamage(t, g.caio, e, pendingID, typedDamage(sum)).GetPendingDamage()
}

// wantPowerPart fails unless the pending damage lists the trade's part once, with the number.
func wantPowerPart(t *testing.T, what string, parts []*playv1.DamagePart, name, source string, flat int32) {
	t.Helper()
	p := powerPartOf(parts)
	if p == nil {
		t.Fatalf("%s: parts = %v, want the power attack's part", what, parts)
	}
	if p.GetKey() == "twice" {
		t.Fatalf("%s: parts = %v, want the power attack's part once", what, parts)
	}
	if p.GetLabelPt() != name || p.GetSourceKey() != source || p.GetFlat() != flat || p.GetDiceCount() != 0 || !p.GetAuto() || p.GetChoosable() ||
		p.GetDamageTypeKey() != slashing || !p.GetAvailable() || p.GetDoubled() {
		t.Errorf("%s: part = %v, want %q from %s, flat %d of slashing, automatic, never chosen, not doubled", what, p, name, source, flat)
	}
}

func TestAHitWithAPowerAttackOpensADamageWithTheFlatPart(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	hit, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack(the heavy trade) = %v, %v; want a hit", hit, err)
	}
	wantPowerPart(t, "the damage the hit opened", hit.GetPendingDamage().GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
	if hit.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL {
		t.Errorf("status = %v, want awaiting the roll", hit.GetPendingDamage().GetStatus())
	}
	rolled := g.rolled(t, e, hit.GetPendingDamage().GetId(), 7)
	// 7 on the dice, the 3 of Strength and the 10 of the trade.
	if rolled.GetAmount() != 7+3+heavyDamage {
		t.Errorf("the damage = %d, want %d: the dice, the modifier and the trade", rolled.GetAmount(), 7+3+heavyDamage)
	}
	wantPowerPart(t, "the damage as rolled", rolled.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
	if r := powerRollOf(rolled.GetPartRolls()); r == nil || r.GetFlat() != heavyDamage || !r.GetCounted() || r.GetDamageTypeKey() != slashing {
		t.Errorf("the roll of the part = %v, want a counted flat %d of slashing", r, heavyDamage)
	}
	if status, amount := g.pendingOf(t, hit.GetPendingDamage().GetId()); status != "applied" || amount != 7+3+heavyDamage {
		t.Errorf("the stored damage = %s for %d, want it applied to the Goblin for %d", status, amount, 7+3+heavyDamage)
	}
}

func TestAnAttackWithNoPowerAttackAddsNoPart(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	hit, err := g.swing(t, e, "", 14)
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack(no trade) = %v, %v; want a hit", hit, err)
	}
	if p := powerPartOf(hit.GetPendingDamage().GetParts()); p != nil {
		t.Errorf("the damage lists %v, want no power part: the attack made no trade", p)
	}
	rolled := g.rolled(t, e, hit.GetPendingDamage().GetId(), 7)
	if rolled.GetAmount() != 7+3 || powerRollOf(rolled.GetPartRolls()) != nil {
		t.Errorf("the damage = %d with the rolls %v, want %d and no roll of the trade", rolled.GetAmount(), rolled.GetPartRolls(), 7+3)
	}
}

// A miss opens no damage, so nothing is written for the trade; the penalty is all the attack cost.
func TestAMissWithAPowerAttackWritesNoDamage(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	miss, err := g.swing(t, e, g.heavyKey, 2) // 2 + 6 - 5 = 3 against 12
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if miss.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || miss.GetPendingDamage() != nil {
		t.Fatalf("the attack = %v with the damage %v, want a miss and none", miss.GetRoll(), miss.GetPendingDamage())
	}
	var rows int
	if err := g.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM pending_damages`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("pending damages = %d (%v), want none", rows, err)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1: the miss still spent the use", got)
	}
}

// A critical hit doubles the dice and not the number: 4d6 (the faces 6, 5, 4 and 3), the 3 of Strength and the 10 once.
func TestACriticalHitDoublesTheDiceAndTheTradeIsAddedOnce(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	hit, err := g.swing(t, e, g.heavyKey, 20)
	if err != nil || hit.GetPendingDamage() == nil || hit.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT {
		t.Fatalf("RollAttack() = %v, %v; want a critical hit", hit, err)
	}
	opened := hit.GetPendingDamage()
	wantPowerPart(t, "the critical damage the hit opened", opened.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
	var weapon *playv1.DamagePart
	for _, p := range opened.GetParts() {
		if p.GetKey() == extraWeaponDamage {
			weapon = p
		}
	}
	if weapon == nil || weapon.GetDiceCount() != 4 || !weapon.GetDoubled() {
		t.Errorf("the weapon's part = %v, want 4 dice, doubled", weapon)
	}
	g.h.roller.queue(6, 5, 4, 3)
	rolled := g.mustDamage(t, g.caio, e, opened.GetId(), inAppDamage).GetPendingDamage()
	if want := int32(6 + 5 + 4 + 3 + 3 + heavyDamage); rolled.GetAmount() != want {
		t.Errorf("the damage = %d, want %d: the 4 dice, the modifier once and the trade once", rolled.GetAmount(), want)
	}
	if r := powerRollOf(rolled.GetPartRolls()); r == nil || r.GetFlat() != heavyDamage || r.GetDiceCount() != 0 || len(r.GetFaces()) != 0 {
		t.Errorf("the roll of the part = %v, want the flat %d and no dice", r, heavyDamage)
	}
	wantPowerPart(t, "the critical damage as rolled", rolled.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
}

// A natural 20 with the trade hits (and is critical) though the total falls short: the free trade's -10 makes 16 against 18.
func TestANaturalTwentyWithAPowerAttackHitsAndCountsTheBonusOnce(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	hit, err := g.swingAt(t, e, greatsword, "Capitão Goblin", g.freeKey, 20)
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack() = %v, %v; want a hit", hit, err)
	}
	if hit.GetRoll().GetD20().GetTotal() != 20+pipoToHit-freePenalty || hit.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT {
		t.Fatalf("the roll = total %d, outcome %v; want 16 and a critical hit", hit.GetRoll().GetD20().GetTotal(), hit.GetRoll().GetOutcome())
	}
	g.h.roller.queue(1, 2, 3, 4)
	rolled := g.mustDamage(t, g.caio, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if want := int32(1 + 2 + 3 + 4 + 3 + freeDamage); rolled.GetAmount() != want {
		t.Errorf("the damage = %d, want %d: the 4 dice, the modifier and the free trade's %d once", rolled.GetAmount(), want, freeDamage)
	}
	wantPowerPart(t, "the natural 20's damage", rolled.GetParts(), powerFreeName, g.freeKey, freeDamage)
}

// The bonus is of the weapon's type, so a creature that resists it takes half of the whole: 7 and 3 and 10 make 20, and 10 lands.
func TestAResistantTargetHalvesThePowerAttackWithTheRestOfTheDamage(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: g.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Pipo": 20, "Toren": 9, "Orla": 5}, setup: true, theatre: true,
	})
	g.h.roller.queue(1)
	added := g.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Hidden = "monster:grick", 1, new(false)
	})
	grick := byLabelID(t, added.GetEncounter(), added.GetCombatantIds()[0])
	e = g.begin(t, g.get(t, g.master))
	for _, o := range g.mustOptions(t, g.caio, e, "Pipo").GetOptions().GetAttacks() {
		if o.GetAttack().GetKey() == greatsword {
			for _, p := range o.GetPowerAttacks() {
				if p.GetName() == powerHeavyName {
					g.heavyKey = p.GetKey()
				}
			}
		}
	}
	hit, err := g.swingAt(t, e, greatsword, grick, g.heavyKey, 14) // 14 + 6 - 5 = 15 against the grick's 14
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack() = %v, %v; want a hit", hit, err)
	}
	g.rolled(t, e, hit.GetPendingDamage().GetId(), 7)
	var step *playv1.DamageStep
	for _, r := range g.log(t, g.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetDamage() != nil && len(en.GetDamage().GetSteps()) == 1 {
				step = en.GetDamage().GetSteps()[0]
			}
		}
	}
	if step == nil || step.GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || step.GetBefore() != 7+3+heavyDamage || step.GetAfter() != (7+3+heavyDamage)/2 {
		t.Fatalf("the master's step = %v, want the grick's resistance taking %d to %d", step, 7+3+heavyDamage, (7+3+heavyDamage)/2)
	}
	if hp, _, _ := g.hp(t, grick); hp != 27-(7+3+heavyDamage)/2 {
		t.Errorf("the grick has %d hit points, want %d", hp, 27-(7+3+heavyDamage)/2)
	}
}

// The undo of the damage and of the attack needs nothing of its own: the target and the use come back, and the damage
// rolled again counts the part once.
func TestTheUndoOfAPowerAttackDamageAndOfTheAttackLeavesNoTrace(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	before, _, _ := g.hp(t, "Goblin")
	hit, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack() = %v, %v; want a hit", hit, err)
	}
	id := hit.GetPendingDamage().GetId()
	g.rolled(t, e, id, 7)
	if hp, _, defeated := g.hp(t, "Goblin"); hp != 0 || !defeated {
		t.Fatalf("the Goblin = %d hit points, defeated %v, want it down by the 20", hp, defeated)
	}
	g.undoLast(t, e) // the damage
	if hp, _, defeated := g.hp(t, "Goblin"); hp != before || defeated {
		t.Errorf("the Goblin = %d hit points, defeated %v after the undo, want %d and standing", hp, defeated, before)
	}
	if status, _ := g.pendingOf(t, id); status != "awaiting_roll" {
		t.Errorf("the damage = %s after the undo, want it awaiting the roll again", status)
	}
	again := g.rolled(t, e, id, 5)
	if again.GetAmount() != 5+3+heavyDamage {
		t.Errorf("the damage rolled again = %d, want %d", again.GetAmount(), 5+3+heavyDamage)
	}
	wantPowerPart(t, "the damage rolled again", again.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
	g.undoLast(t, e) // the damage again
	g.undoLast(t, e) // the attack
	var rows int
	if err := g.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM pending_damages`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("pending damages = %d (%v) after the undo of the attack, want none", rows, err)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A held roll (Lucky for the halfling's 1, a Bardic Inspiration die, the master's check of the hit) runs the attack again
// from the answer: the damage it opens has the part once.
func TestAHeldRollsReplayWritesThePowerPartOnce(t *testing.T) {
	t.Parallel()
	t.Run("Lucky", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fight(t)
		held, err := g.swing(t, e, g.heavyKey, 1)
		if err != nil || held.GetLuckyOffer() == nil {
			t.Fatalf("RollAttack() = %v, %v; want Lucky's question", held, err)
		}
		res, err := g.answerLucky(t, g.caio, e, held.GetLuckyOffer().GetHoldId(), rerollTyped(15))
		if err != nil || res.GetAttack().GetPendingDamage() == nil {
			t.Fatalf("AnswerLucky() = %v, %v; want a hit", res, err)
		}
		pending := res.GetAttack().GetPendingDamage()
		wantPowerPart(t, "the damage after Lucky", pending.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
		if rolled := g.rolled(t, e, pending.GetId(), 7); rolled.GetAmount() != 7+3+heavyDamage {
			t.Errorf("the damage = %d, want %d", rolled.GetAmount(), 7+3+heavyDamage)
		}
	})
	t.Run("Bardic Inspiration", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		e := g.fightInspired(t)
		held, err := g.swing(t, e, g.heavyKey, 9)
		if err != nil || held.GetInspirationOffer() == nil {
			t.Fatalf("RollAttack() = %v, %v; want the die's question", held, err)
		}
		res, err := g.answer(t, g.caio, e, held.GetInspirationOffer().GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) {
			r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_TypedFace{TypedFace: 3}
		})
		if err != nil || res.GetAttack().GetPendingDamage() == nil {
			t.Fatalf("AnswerBardicInspiration() = %v, %v; want a hit", res, err)
		}
		pending := res.GetAttack().GetPendingDamage()
		wantPowerPart(t, "the damage after the die", pending.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
		if rolled := g.rolled(t, e, pending.GetId(), 7); rolled.GetAmount() != 7+3+heavyDamage {
			t.Errorf("the damage = %d, want %d", rolled.GetAmount(), 7+3+heavyDamage)
		}
	})
	t.Run("the master's check", func(t *testing.T) {
		t.Parallel()
		g := newPowerTable(t)
		g.setRules(t, enemyReactionsAlways)
		e := g.fight(t)
		held, err := g.swing(t, e, g.heavyKey, 14)
		if err != nil || !held.GetRoll().GetHeldForReaction() {
			t.Fatalf("RollAttack() = %v, %v; want it held for the master's check", held, err)
		}
		w := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
		if w == nil {
			t.Fatal("the master has no check")
		}
		g.mustAnswer(t, g.master, e, w.GetId(), passAnswer)
		if w2 := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK); w2 != nil {
			g.mustAnswer(t, g.master, e, w2.GetId(), passAnswer)
		}
		var raw string
		if err := g.h.pool.QueryRow(t.Context(), `SELECT parts::TEXT FROM pending_damages ORDER BY created_at DESC LIMIT 1`).Scan(&raw); err != nil {
			t.Fatalf("read the damage's parts: %v", err)
		}
		if n := strings.Count(raw, `"key": "power_attack"`); n != 1 {
			t.Errorf("the stored parts = %s, want the power part once (found %d)", raw, n)
		}
	})
}

// The log: the trade is on the attacker's line with the numbers for the master and the attacker's player, and a miss
// carries it too. Another player reads that a trade was made and nothing of the numbers, and of a retired feature only
// "Outra fonte".
func TestTheLogLineOfAPowerAttackCarriesTheTradeForEachReader(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	if _, err := g.swingAt(t, e, greatsword, "Capitão Goblin", g.heavyKey, 2); err != nil { // a miss
		t.Fatalf("RollAttack(miss) error = %v", err)
	}
	g.passTo(t, e, "Toren")
	g.passTo(t, e, "Pipo")
	hit, err := g.swingAt(t, e, greatsword, "Capitão Goblin", g.freeKey, 20) // a natural 20 hits whatever the total
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack(hit) = %v, %v; want a hit", hit, err)
	}
	g.rolled(t, e, hit.GetPendingDamage().GetId(), 7)
	g.passTo(t, e, "Toren")
	g.passTo(t, e, "Pipo")
	if _, err := g.swingAt(t, e, greatsword, "Capitão Goblin", "", 20); err != nil { // no trade
		t.Fatalf("RollAttack(plain) error = %v", err)
	}
	markers := func(u *user) []*playv1.CombatLogPowerAttack {
		var out []*playv1.CombatLogPowerAttack
		for _, r := range g.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorId() == g.id(t, "Pipo") {
					out = append(out, en.GetPowerAttack())
				}
			}
		}
		return out
	}
	for who, u := range map[string]*user{"the master": g.master, "Caio": g.caio} {
		got := markers(u)
		// The newest round first: the plain attack, the free trade's hit, the heavy trade's miss.
		if len(got) != 3 || got[0] != nil {
			t.Fatalf("%s reads the markers %v, want none on the plain attack and one on each of the others", who, got)
		}
		if m := got[1]; m.GetKey() != g.freeKey || m.GetNamePt() != powerFreeName || m.GetPenalty() != freePenalty || m.GetDamageBonus() != freeDamage || m.GetHidden() {
			t.Errorf("%s reads %v on the hit, want the free trade with %d and %d", who, m, freePenalty, freeDamage)
		}
		if m := got[2]; m.GetKey() != g.heavyKey || m.GetNamePt() != powerHeavyName || m.GetPenalty() != heavyPenalty || m.GetDamageBonus() != heavyDamage {
			t.Errorf("%s reads %v on the miss, want the heavy trade with %d and %d", who, m, heavyPenalty, heavyDamage)
		}
	}
	got := markers(g.ana)
	if len(got) != 3 || got[0] != nil {
		t.Fatalf("Ana reads the markers %v, want none on the plain attack and one on each of the others", got)
	}
	for i, want := range []string{"", powerFreeName, powerHeavyName} {
		if i == 0 {
			continue
		}
		if m := got[i]; m.GetNamePt() != want || m.GetKey() != "" || m.GetPenalty() != 0 || m.GetDamageBonus() != 0 || m.GetHidden() {
			t.Errorf("Ana reads %v, want the name %q alone", m, want)
		}
	}
}

func TestTheLogLineOfAnAttackWithAnArchivedPowerAttackSaysAnotherSourceToAnotherPlayer(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	if _, err := g.master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: g.campaignID, Key: g.subKey})); err != nil {
		t.Fatalf("ArchiveTableEntry() error = %v", err)
	}
	hit, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil || hit.GetPendingDamage() == nil {
		t.Fatalf("RollAttack() = %v, %v; want the sheet to keep the retired feature's trade and hit", hit, err)
	}
	owned := powerPartOf(hit.GetPendingDamage().GetParts())
	if owned == nil || owned.GetLabelPt() != powerHeavyName || owned.GetFlat() != heavyDamage {
		t.Errorf("the attacker's part = %v, want the name and the number of its own sheet", owned)
	}
	var told int
	for _, r := range g.log(t, g.ana, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetPowerAttack() != nil {
				told++
			}
			if m := en.GetPowerAttack(); m != nil && (m.GetNamePt() != otherSourceTx || !m.GetHidden() || m.GetKey() != "" || m.GetPenalty() != 0 || m.GetDamageBonus() != 0) {
				t.Errorf("Ana reads %v of the retired feature, want %q and nothing else", m, otherSourceTx)
			}
		}
	}
	if told != 1 {
		t.Errorf("Ana reads %d markers, want 1: the line says a trade was made", told)
	}
	var seen int
	for _, r := range g.log(t, g.caio, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if m := en.GetPowerAttack(); m != nil {
				seen++
				if m.GetNamePt() != powerHeavyName || m.GetKey() != g.heavyKey || m.GetPenalty() != heavyPenalty || m.GetHidden() {
					t.Errorf("Caio reads %v, want his own feature, name and numbers", m)
				}
			}
		}
	}
	if seen != 1 {
		t.Errorf("Caio reads %d markers, want 1 (the positive control)", seen)
	}
}
